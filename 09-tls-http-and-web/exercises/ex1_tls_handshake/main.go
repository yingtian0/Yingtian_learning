// ex1: TLS のハンドシェイクを、測る
//
//	A. 送受信の「塊」(フライト) を数え、TLS 1.2 と TLS 1.3 の、往復の回数を比べる。
//	B. 往復時間 (RTT) 50 ms を、人工的に加えて、ハンドシェイクの時間を測る。
//	   (TLS 1.2 の全体 / TLS 1.3 の全体 / 再開 (セッションチケット))
//	C. CPU のコスト: 証明書の鍵の種類 (ECDSA / RSA)、鍵交換の曲線 (X25519 / P-256)、再開、で比べる。
//	D. ハンドシェイクで送られるバイト数 (証明書の鎖の大きさ)。
//
// 注意: 鍵と証明書は、演習のために、実行時に作る使い捨てのもので、本物の認証局とは無関係。
// 接続は、ループバックだけ (人工的な遅延は、netsim で加える)。
// 実行: go run ./ex1_tls_handshake
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"ex9web/internal/netsim"
	"ex9web/internal/pki"
)

type flight struct {
	dir   string
	bytes int
}

type recConn struct {
	net.Conn
	mu      sync.Mutex
	flights []flight
}

func (r *recConn) add(dir string, n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n == 0 {
		return
	}
	if l := len(r.flights); l > 0 && r.flights[l-1].dir == dir {
		r.flights[l-1].bytes += n
	} else {
		r.flights = append(r.flights, flight{dir, n})
	}
}
func (r *recConn) Write(b []byte) (int, error) {
	n, err := r.Conn.Write(b)
	r.add("→", n)
	return n, err
}
func (r *recConn) Read(b []byte) (int, error) {
	n, err := r.Conn.Read(b)
	r.add("←", n)
	return n, err
}

func newPKI(kind string) (*pki.Entity, *pki.Entity, tls.Certificate) {
	root, err := pki.NewRoot("演習用ルート", kind)
	must(err)
	inter, err := root.IssueCA("演習用中間", kind)
	must(err)
	leaf, err := inter.IssueLeaf("localhost", []string{"localhost"}, kind, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	must(err)
	return root, inter, pki.Chain(leaf, inter)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// 1回の接続を終えたら、サーバーが 1 バイトを送る (TLS 1.3 のセッションチケットを、クライアントに処理させるため)
func startServer(cfg *tls.Config, ln net.Listener) {
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				tc := tls.Server(c, cfg)
				if tc.Handshake() != nil {
					return
				}
				tc.Write([]byte{'k'})
				io.Copy(io.Discard, tc)
			}(c)
		}
	}()
}

func versionName(v uint16) string { return tls.VersionName(v) }

func partA(serverCfg *tls.Config, pool *x509Pool) {
	fmt.Println("A. ハンドシェイクの、送受信の塊 (フライト)。クライアントから見て、→ は送信、← は受信")
	for _, ver := range []struct {
		name string
		v    uint16
	}{{"TLS 1.2", tls.VersionTLS12}, {"TLS 1.3", tls.VersionTLS13}} {
		ln, _ := net.Listen("tcp", "127.0.0.1:0")
		startServer(serverCfg, ln)
		raw, _ := net.Dial("tcp", ln.Addr().String())
		rc := &recConn{Conn: raw}
		cfg := &tls.Config{RootCAs: pool.p, ServerName: "localhost", MinVersion: ver.v, MaxVersion: ver.v}
		tc := tls.Client(rc, cfg)
		must(tc.Handshake())
		st := tc.ConnectionState()
		var parts []string
		round := 0
		for i, f := range rc.flights {
			parts = append(parts, fmt.Sprintf("%s %d B", f.dir, f.bytes))
			if f.dir == "←" && i < len(rc.flights) {
				round++
			}
		}
		fmt.Printf("  %s: %s / %s\n    塊: %s\n    クライアントが、データを送り始められるまでの、往復 = %d 回 (応答の塊の数)\n", ver.name, versionName(st.Version), tls.CipherSuiteName(st.CipherSuite), strings.Join(parts, "  "), round)
		tc.Close()
		ln.Close()
	}
}

type x509Pool struct{ p *x509.CertPool }

func median(d []time.Duration) time.Duration {
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	return d[len(d)/2]
}

func partB(serverCfg *tls.Config, pool *x509Pool) {
	const oneWay = 25 * time.Millisecond
	fmt.Printf("\nB. 往復時間 %v を加えたときの、TLS ハンドシェイクの時間 (TCP の接続の分は含まない。各 5 回の中央値)\n", 2*oneWay)
	type tc struct {
		name   string
		ver    uint16
		resume bool
	}
	for _, c := range []tc{
		{"TLS 1.2 (全体)", tls.VersionTLS12, false},
		{"TLS 1.2 (再開)", tls.VersionTLS12, true},
		{"TLS 1.3 (全体)", tls.VersionTLS13, false},
		{"TLS 1.3 (再開)", tls.VersionTLS13, true},
	} {
		rawLn, _ := net.Listen("tcp", "127.0.0.1:0")
		ln := &netsim.Listener{Listener: rawLn, Delay: oneWay} // サーバー → クライアントにも、片道の遅延を加える
		startServer(serverCfg, ln)
		cache := tls.NewLRUClientSessionCache(8)
		cfg := &tls.Config{RootCAs: pool.p, ServerName: "localhost", MinVersion: c.ver, MaxVersion: c.ver}
		if c.resume {
			cfg.ClientSessionCache = cache
		}
		dial := func() (time.Duration, bool) {
			raw, _ := net.Dial("tcp", ln.Addr().String())
			w := netsim.Wrap(raw, oneWay, nil)
			conn := tls.Client(w, cfg)
			t0 := time.Now()
			must(conn.Handshake())
			d := time.Since(t0)
			buf := make([]byte, 1)
			conn.Read(buf) // チケットを処理する
			resumed := conn.ConnectionState().DidResume
			conn.Close()
			return d, resumed
		}
		if c.resume {
			dial() // 最初の1回で、チケットをもらう
		}
		var ds []time.Duration
		resumed := false
		for i := 0; i < 5; i++ {
			d, r := dial()
			ds = append(ds, d)
			resumed = r
		}
		fmt.Printf("  %-16s %7.1f ms  (再開したか: %v)\n", c.name, float64(median(ds).Microseconds())/1000, resumed)
		ln.Close()
	}
	fmt.Printf("  (TCP の 3 ウェイハンドシェイクで、さらに 1 RTT = %v が、前に加わる)\n", 2*oneWay)
}

func partC() {
	fmt.Println("\nC. ハンドシェイク 1 回の CPU コスト (クライアントとサーバーの両方の計算を含む。メモリ内でつなぐ。TLS 1.3)")
	fmt.Printf("  %-34s %10s %12s\n", "条件", "1回あたり", "1秒あたり")
	type cc struct {
		name   string
		kind   string
		curves []tls.CurveID
		resume bool
		n      int
	}
	for _, c := range []cc{
		{"ECDSA P-256 証明書、X25519", "ecdsa", []tls.CurveID{tls.X25519}, false, 300},
		{"ECDSA P-256 証明書、P-256 で鍵交換", "ecdsa", []tls.CurveID{tls.CurveP256}, false, 300},
		{"RSA 2048 証明書、X25519", "rsa2048", []tls.CurveID{tls.X25519}, false, 200},
		{"RSA 3072 証明書、X25519", "rsa3072", []tls.CurveID{tls.X25519}, false, 100},
		{"ECDSA P-256 証明書 (再開)", "ecdsa", []tls.CurveID{tls.X25519}, true, 300},
	} {
		root, _, chain := newPKI(c.kind)
		pool := pki.Pool(root)
		scfg := &tls.Config{Certificates: []tls.Certificate{chain}, MinVersion: tls.VersionTLS13, CurvePreferences: c.curves}
		ccfg := &tls.Config{RootCAs: pool, ServerName: "localhost", MinVersion: tls.VersionTLS13, CurvePreferences: c.curves}
		if c.resume {
			ccfg.ClientSessionCache = tls.NewLRUClientSessionCache(4)
		}
		once := func() {
			a, b := net.Pipe()
			done := make(chan struct{})
			go func() {
				s := tls.Server(b, scfg)
				if s.Handshake() == nil {
					s.Write([]byte{'k'})
					io.Copy(io.Discard, s)
				}
				close(done)
			}()
			cl := tls.Client(a, ccfg)
			must(cl.Handshake())
			buf := make([]byte, 1)
			cl.Read(buf)
			cl.Close()
			<-done
		}
		once()
		t0 := time.Now()
		for i := 0; i < c.n; i++ {
			once()
		}
		el := time.Since(t0)
		fmt.Printf("  %-34s %7.2f ms %10.0f 回\n", c.name, float64(el.Microseconds())/1000/float64(c.n), float64(c.n)/el.Seconds())
	}
}

func partD() {
	fmt.Println("\nD. ハンドシェイクで流れる、バイト数の合計 (TLS 1.3、証明書の鎖 = 葉 + 中間)")
	fmt.Printf("  %-12s %12s %16s\n", "鍵の種類", "鎖の大きさ", "ハンドシェイク全体")
	for _, kind := range []string{"ecdsa", "rsa2048", "rsa3072", "rsa4096"} {
		root, _, chain := newPKI(kind)
		scfg := &tls.Config{Certificates: []tls.Certificate{chain}, MinVersion: tls.VersionTLS13}
		ln, _ := net.Listen("tcp", "127.0.0.1:0")
		startServer(scfg, ln)
		raw, _ := net.Dial("tcp", ln.Addr().String())
		rc := &recConn{Conn: raw}
		tc := tls.Client(rc, &tls.Config{RootCAs: pki.Pool(root), ServerName: "localhost", MinVersion: tls.VersionTLS13})
		must(tc.Handshake())
		total := 0
		for _, f := range rc.flights {
			total += f.bytes
		}
		chainBytes := 0
		for _, c := range chain.Certificate {
			chainBytes += len(c)
		}
		fmt.Printf("  %-12s %9d B %13d B\n", kind, chainBytes, total)
		tc.Close()
		ln.Close()
	}
}

func main() {
	root, _, chain := newPKI("ecdsa")
	serverCfg := &tls.Config{Certificates: []tls.Certificate{chain}}
	pool := &x509Pool{p: pki.Pool(root)}
	partA(serverCfg, pool)
	partB(serverCfg, pool)
	partC()
	partD()
}
