// ex1: DNS を、バイトから作る
//
//	A. DNS のメッセージを、バイト単位で組み立て、読む (ヘッダ、質問、名前の圧縮)。
//	B. 3 階層の、DNS の世界 (ルート → TLD → 権威サーバー) を、ループバックの上に作る。
//	   各サーバーに、人工的な往復時間を加える (ルート 10 ms、TLD 30 ms、権威 60 ms)。
//	C. 自作の、再帰リゾルバ (キャッシュつき) で、名前を引き、何回問い合わせるか、何 ms かかるかを測る。
//	     冷えた状態 / キャッシュ / NS のキャッシュ / 否定キャッシュ / TTL の期限切れ / CNAME / UDP の切り詰めと TCP
//
// 注意: すべて、ループバックの、自分で起動したサーバーだけ。実在の DNS サーバーには、問い合わせない。
//
//	名前は、予約された `.test`、アドレスは、文書用の範囲 (192.0.2.0/24、203.0.113.0/24)。
//
// 実行: go run ./ex1_dns
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type Zone struct {
	Origin  string
	Records []RR
}

type Server struct {
	label string
	zone  Zone
	rtt   time.Duration
	udp   *net.UDPConn
	tcp   net.Listener
}

func (s *Server) inZone(name string) bool {
	return s.zone.Origin == "." || name == s.zone.Origin || strings.HasSuffix(name, "."+s.zone.Origin)
}

func (s *Server) handle(q *Message) *Message {
	name := fqdn(q.QName)
	resp := &Message{ID: q.ID, Response: true, RD: q.RD, QName: q.QName, QType: q.QType}
	if !s.inZone(name) {
		resp.Rcode = 5 // REFUSED
		return resp
	}
	// 委任 (ゾーンの途中の NS) があれば、紹介 (referral) を返す
	best := ""
	for _, r := range s.zone.Records {
		if r.Type == TypeNS && r.Name != s.zone.Origin && (name == r.Name || strings.HasSuffix(name, "."+r.Name)) && len(r.Name) > len(best) {
			best = r.Name
		}
	}
	if best != "" {
		for _, r := range s.zone.Records {
			if r.Type == TypeNS && r.Name == best {
				resp.Auth = append(resp.Auth, r)
				for _, g := range s.zone.Records { // のり (glue): NS のアドレス
					if g.Type == TypeA && g.Name == r.Data {
						resp.Extra = append(resp.Extra, g)
					}
				}
			}
		}
		return resp
	}
	resp.AA = true
	var exact []RR
	exists := false
	for _, r := range s.zone.Records {
		if r.Name == name {
			exists = true
			if r.Type == q.QType {
				exact = append(exact, r)
			} else if r.Type == TypeCNAME && q.QType != TypeCNAME {
				exact = []RR{r}
				break
			}
		}
	}
	soa := func() {
		for _, r := range s.zone.Records {
			if r.Type == TypeSOA {
				resp.Auth = append(resp.Auth, r)
			}
		}
	}
	switch {
	case len(exact) > 0:
		resp.Answer = exact
	case exists:
		soa() // 名前はあるが、その型の記録がない (NODATA)
	default:
		resp.Rcode = RcodeNXDomain
		soa()
	}
	return resp
}

func (s *Server) respondUDP(q *Message) []byte {
	resp := s.handle(q)
	b := resp.Pack()
	if len(b) > 512 { // UDP の応答は、512 バイトまで。超えたら、切り詰めて TC を立てる
		t := &Message{ID: resp.ID, Response: true, AA: resp.AA, TC: true, RD: resp.RD, QName: resp.QName, QType: resp.QType}
		return t.Pack()
	}
	return b
}

func startServer(label string, zone Zone, rtt time.Duration) (*Server, string) {
	uc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	must(err)
	port := uc.LocalAddr().(*net.UDPAddr).Port
	tl, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	must(err)
	s := &Server{label: label, zone: zone, rtt: rtt, udp: uc, tcp: tl}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, addr, err := uc.ReadFromUDP(buf)
			if err != nil {
				return
			}
			pkt := append([]byte(nil), buf[:n]...)
			go func() {
				q, err := Unpack(pkt)
				if err != nil {
					return
				}
				time.Sleep(s.rtt) // 往復時間の模擬
				uc.WriteToUDP(s.respondUDP(q), addr)
			}()
		}
	}()
	go func() {
		for {
			c, err := tl.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var l [2]byte
				if _, err := io.ReadFull(c, l[:]); err != nil {
					return
				}
				b := make([]byte, binary.BigEndian.Uint16(l[:]))
				io.ReadFull(c, b)
				q, err := Unpack(b)
				if err != nil {
					return
				}
				time.Sleep(s.rtt)
				out := s.handle(q).Pack()
				c.Write(binary.BigEndian.AppendUint16(nil, uint16(len(out))))
				c.Write(out)
			}(c)
		}
	}()
	return s, fmt.Sprintf("127.0.0.1:%d", port)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// ---------------- リゾルバ ----------------

type serverInfo struct {
	addr  string
	rtt   time.Duration
	label string
}

type cacheEntry struct {
	rrs     []RR
	rcode   int
	expires time.Time
}

type nsEntry struct {
	ips     []string
	expires time.Time
}

type Resolver struct {
	mu      sync.Mutex
	network map[string]serverInfo // 偽のアドレス → 実際の接続先
	roots   []string
	cache   map[string]cacheEntry
	ns      map[string]nsEntry
	queries int
	trace   []string
}

func NewResolver(network map[string]serverInfo, roots []string) *Resolver {
	return &Resolver{network: network, roots: roots, cache: map[string]cacheEntry{}, ns: map[string]nsEntry{}}
}

func (r *Resolver) logf(f string, a ...any) { r.trace = append(r.trace, fmt.Sprintf(f, a...)) }

func (r *Resolver) startServers(name string) []string {
	best := ""
	for zone, e := range r.ns {
		if time.Now().Before(e.expires) && (name == zone || strings.HasSuffix(name, "."+zone) || zone == ".") && len(zone) > len(best) {
			best = zone
		}
	}
	if best != "" {
		r.logf("  NS のキャッシュ: %s の権威サーバーを知っている", best)
		return r.ns[best].ips
	}
	return r.roots
}

func (r *Resolver) query(server, name string, qtype uint16) (*Message, error) {
	info := r.network[server]
	r.queries++
	q := &Message{ID: uint16(r.queries), QName: name, QType: qtype}
	conn, err := net.Dial("udp", info.addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.Write(q.Pack())
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	resp, err := Unpack(buf[:n])
	if err != nil {
		return nil, err
	}
	r.logf("  → %-22s UDP で問い合わせ: %s %s  (応答 %d バイト%s)", info.label, name, typeName(qtype), n, map[bool]string{true: "、切り詰め(TC)", false: ""}[resp.TC])
	if resp.TC { // 切り詰められた: TCP でやり直す
		time.Sleep(info.rtt) // TCP の 3 ウェイハンドシェイクの分 (1 RTT)
		r.queries++
		tc, err := net.Dial("tcp", info.addr)
		if err != nil {
			return nil, err
		}
		defer tc.Close()
		tc.Write(binary.BigEndian.AppendUint16(nil, uint16(len(q.Pack()))))
		tc.Write(q.Pack())
		var l [2]byte
		if _, err := io.ReadFull(tc, l[:]); err != nil {
			return nil, err
		}
		b := make([]byte, binary.BigEndian.Uint16(l[:]))
		io.ReadFull(tc, b)
		resp, err = Unpack(b)
		r.logf("  → %-22s TCP でやり直し (ハンドシェイク + 問い合わせ): 応答 %d バイト", info.label, len(b)+2)
	}
	return resp, err
}

func minTTL(rrs []RR) uint32 {
	m := uint32(1 << 30)
	for _, r := range rrs {
		if r.TTL < m {
			m = r.TTL
		}
	}
	return m
}

func (r *Resolver) Lookup(name string, qtype uint16) ([]RR, int, error) {
	name = fqdn(name)
	key := fmt.Sprintf("%s|%d", name, qtype)
	if e, ok := r.cache[key]; ok && time.Now().Before(e.expires) {
		r.logf("  キャッシュにあった: %s %s (残り %.1f 秒)%s", name, typeName(qtype), time.Until(e.expires).Seconds(), map[bool]string{true: " ← 否定キャッシュ", false: ""}[e.rcode == RcodeNXDomain])
		return e.rrs, e.rcode, nil
	}
	if e, ok := r.cache[name+"|5"]; ok && qtype != TypeCNAME && time.Now().Before(e.expires) {
		r.logf("  CNAME がキャッシュにあった: %s → %s", name, e.rrs[0].Data)
		rest, rc, err := r.Lookup(e.rrs[0].Data, qtype)
		return append(append([]RR{}, e.rrs...), rest...), rc, err
	}
	servers := r.startServers(name)
	for iter := 0; iter < 8; iter++ {
		resp, err := r.query(servers[0], name, qtype)
		if err != nil {
			return nil, -1, err
		}
		switch {
		case resp.Rcode == RcodeNXDomain || (resp.AA && len(resp.Answer) == 0):
			ttl := uint32(60)
			for _, a := range resp.Auth {
				if a.Type == TypeSOA {
					var mn, rn string
					var s, rf, rt, ex, minimum uint32
					fmt.Sscanf(a.Data, "%s %s %d %d %d %d %d", &mn, &rn, &s, &rf, &rt, &ex, &minimum)
					ttl = minimum
				}
			}
			r.cache[key] = cacheEntry{nil, resp.Rcode, time.Now().Add(time.Duration(ttl) * time.Second)}
			return nil, resp.Rcode, nil
		case len(resp.Answer) > 0:
			if !(resp.Answer[0].Type == TypeCNAME && qtype != TypeCNAME) { // CNAME だけの応答は、「その型の答え」としては、保存しない
				r.cache[key] = cacheEntry{resp.Answer, resp.Rcode, time.Now().Add(time.Duration(minTTL(resp.Answer)) * time.Second)}
			}
			if resp.Answer[0].Type == TypeCNAME && qtype != TypeCNAME {
				r.cache[name+"|5"] = cacheEntry{resp.Answer[:1], 0, time.Now().Add(time.Duration(resp.Answer[0].TTL) * time.Second)}
				r.logf("  CNAME: %s → %s。別名の先を、改めて引く", name, resp.Answer[0].Data)
				rest, rc, err := r.Lookup(resp.Answer[0].Data, qtype)
				return append(append([]RR{}, resp.Answer[:1]...), rest...), rc, err
			}
			return resp.Answer, resp.Rcode, nil
		default: // 紹介
			if len(resp.Auth) == 0 {
				return nil, resp.Rcode, nil
			}
			zone := resp.Auth[0].Name
			var ips []string
			for _, g := range resp.Extra {
				if g.Type == TypeA {
					ips = append(ips, g.Data)
				}
			}
			r.ns[zone] = nsEntry{ips, time.Now().Add(time.Duration(minTTL(resp.Auth)) * time.Second)}
			r.logf("    紹介: %s の権威サーバーは %s (のり: %v)", zone, resp.Auth[0].Data, ips)
			servers = ips
		}
	}
	return nil, -1, fmt.Errorf("紹介が長すぎる")
}

func (r *Resolver) run(title, name string, qtype uint16) {
	r.queries, r.trace = 0, nil
	t0 := time.Now()
	ans, rc, err := r.Lookup(name, qtype)
	el := time.Since(t0)
	fmt.Printf("\n  [%s] %s %s\n", title, name, typeName(qtype))
	for _, l := range r.trace {
		fmt.Println("  " + l)
	}
	var as []string
	for _, a := range ans {
		d := a.Data
		if len(d) > 24 {
			d = fmt.Sprintf("%s…(%d 文字)", d[:8], len(d))
		}
		as = append(as, fmt.Sprintf("%s=%s", typeName(a.Type), d))
	}
	if len(as) > 3 {
		as = append(as[:3], fmt.Sprintf("...(全 %d 件)", len(ans)))
	}
	rcs := map[int]string{0: "NOERROR", 3: "NXDOMAIN"}[rc]
	fmt.Printf("    → 結果: %s %v、サーバーへの問い合わせ %d 回、所要時間 %.1f ms\n", rcs, as, r.queries, float64(el.Microseconds())/1000)
	if err != nil {
		fmt.Println("    エラー:", err)
	}
}

func hexdump(b []byte) {
	for i := 0; i < len(b); i += 16 {
		end := i + 16
		if end > len(b) {
			end = len(b)
		}
		fmt.Printf("    %04x  ", i)
		for j := i; j < end; j++ {
			fmt.Printf("%02x ", b[j])
		}
		fmt.Println()
	}
}

func main() {
	soa := func(origin string, minimum int) RR {
		return RR{Name: origin, Type: TypeSOA, TTL: 300, Data: fmt.Sprintf("ns.%s hostmaster.%s 1 3600 600 86400 %d", origin, origin, minimum)}
	}
	var txt []RR
	for i := 0; i < 12; i++ {
		txt = append(txt, RR{Name: "big.example.test.", Type: TypeTXT, TTL: 300, Data: strings.Repeat(string(rune('a'+i)), 100)})
	}
	authZone := Zone{Origin: "example.test.", Records: append([]RR{
		soa("example.test.", 30),
		{"example.test.", TypeNS, 300, "ns1.example.test."},
		{"ns1.example.test.", TypeA, 300, "192.0.2.20"},
		{"www.example.test.", TypeCNAME, 300, "lb.example.test."},
		{"lb.example.test.", TypeA, 60, "203.0.113.10"},
		{"lb.example.test.", TypeA, 60, "203.0.113.11"},
		{"api.example.test.", TypeA, 300, "203.0.113.20"},
		{"short.example.test.", TypeA, 2, "203.0.113.30"},
		{"v6.example.test.", TypeAAAA, 300, "2001:db8::1"},
		{"example.test.", TypeMX, 300, "10 mail.example.test."},
		{"mail.example.test.", TypeA, 300, "203.0.113.40"},
	}, txt...)}
	tldZone := Zone{Origin: "test.", Records: []RR{
		soa("test.", 60),
		{"example.test.", TypeNS, 3600, "ns1.example.test."},
		{"ns1.example.test.", TypeA, 3600, "192.0.2.20"},
	}}
	rootZone := Zone{Origin: ".", Records: []RR{
		soa(".", 60),
		{"test.", TypeNS, 86400, "a.nic.test."},
		{"a.nic.test.", TypeA, 86400, "192.0.2.10"},
	}}

	_, rootAddr := startServer("ルート", rootZone, 10*time.Millisecond)
	_, tldAddr := startServer("TLD (test.)", tldZone, 30*time.Millisecond)
	authSrv, authAddr := startServer("権威 (example.test.)", authZone, 60*time.Millisecond)
	network := map[string]serverInfo{
		"192.0.2.1":  {rootAddr, 10 * time.Millisecond, "ルート"},
		"192.0.2.10": {tldAddr, 30 * time.Millisecond, "TLD (test.)"},
		"192.0.2.20": {authAddr, 60 * time.Millisecond, "権威 (example.test.)"},
	}

	fmt.Println("A. DNS のメッセージを、バイトで見る")
	q := &Message{ID: 0x1234, RD: true, QName: "www.example.test.", QType: TypeA}
	qb := q.Pack()
	fmt.Printf("  質問: www.example.test. の A (%d バイト)\n", len(qb))
	hexdump(qb)
	fmt.Printf("    ヘッダ(12 バイト): ID=0x%04x フラグ=0x%04x (RD=再帰を希望)  質問数=%d 回答数=%d\n", binary.BigEndian.Uint16(qb), binary.BigEndian.Uint16(qb[2:]), binary.BigEndian.Uint16(qb[4:]), binary.BigEndian.Uint16(qb[6:]))
	fmt.Println("    名前: 03 'www' 07 'example' 04 'test' 00  (ラベルの長さ + 文字列、最後に 0)。続いて、型 0x0001 (A)、クラス 0x0001 (IN)")
	resp := authSrv.handle(&Message{ID: 0x1234, RD: true, QName: "api.example.test.", QType: TypeA})
	rb := resp.Pack()
	fmt.Printf("  権威サーバーの応答: api.example.test. の A (%d バイト)\n", len(rb))
	hexdump(rb)
	fmt.Printf("    フラグ=0x%04x (QR=応答、AA=権威)。回答の名前は、0xC00C = 「先頭から 12 バイト目の名前を、もう一度使う」(名前の圧縮)\n", binary.BigEndian.Uint16(rb[2:]))
	for _, tc := range []struct {
		n string
		t uint16
	}{{"api.example.test.", TypeA}, {"big.example.test.", TypeTXT}} {
		full := authSrv.handle(&Message{QName: tc.n, QType: tc.t}).Pack()
		fmt.Printf("  %s の %s の応答の大きさ: %d バイト%s\n", tc.n, typeName(tc.t), len(full), map[bool]string{true: " (> 512 バイト: UDP では切り詰められる)", false: ""}[len(full) > 512])
	}

	fmt.Println("\nB. 3 階層の DNS (往復時間: ルート 10 ms、TLD 30 ms、権威 60 ms)。リゾルバは、キャッシュを持つ")
	res := NewResolver(network, []string{"192.0.2.1"})
	fmt.Println("\nC. 名前を引く")
	res.run("1. 冷えた状態 (キャッシュなし)。CNAME を含む", "www.example.test", TypeA)
	res.run("2. もう一度 (すべてキャッシュ)", "www.example.test", TypeA)
	res.run("3. 同じゾーンの別の名前 (NS はキャッシュにある)", "api.example.test", TypeA)
	res.run("4. 存在しない名前 (NXDOMAIN)", "nothing.example.test", TypeA)
	res.run("5. 存在しない名前を、もう一度 (否定キャッシュ)", "nothing.example.test", TypeA)
	res.run("6. TTL が 2 秒の名前", "short.example.test", TypeA)
	res.run("7. もう一度 (まだ TTL の中)", "short.example.test", TypeA)
	time.Sleep(2200 * time.Millisecond)
	res.run("8. 2.2 秒待ってから (TTL の期限切れ)", "short.example.test", TypeA)
	res.run("9. 大きな応答 (TXT 12 件): UDP が切り詰められ、TCP でやり直す", "big.example.test", TypeTXT)
	res.run("10. MX と AAAA", "v6.example.test", TypeAAAA)
}
