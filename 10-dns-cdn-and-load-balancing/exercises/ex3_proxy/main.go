// ex3: ロードバランサ (リバースプロキシ) を、実際に作って、測る
//
// 3 台のバックエンド (ループバックの HTTP サーバー) の前に、次のプロキシを置く。
//
//	L4: TCP の接続を、そのまま中継する (HTTP の中身は、見ない)
//	L7: HTTP を、解釈して、リクエストごとに、バックエンドを選ぶ (httputil.ReverseProxy)
//
//	A. 経路ごとの、遅延とスループット: 直接 / L4 / L7 (バックエンドへの接続を使い回す) / L7 (毎回、新しい接続)
//	B. バックエンドが 1 台、遅いとき: ラウンドロビン と 最小接続
//	C. バックエンドが 1 台、途中で落ちたとき: 再試行とヘルスチェックの有無で、エラーの数が、どう変わるか
//
// すべて、ループバック。クライアントは、32 並行。各測定は 2〜3 秒。
// 実行: go run ./ex3_proxy
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type backend struct {
	id      int
	srv     *http.Server
	ln      net.Listener
	addr    string
	delay   time.Duration
	hits    atomic.Int64
	active  atomic.Int64 // プロキシから見た、処理中のリクエスト数 (最小接続用)
	downTil atomic.Int64 // この時刻 (UnixNano) まで、故障とみなす (パッシブなヘルスチェック)
}

func startBackend(id int, delay time.Duration) *backend {
	b := &backend{id: id, delay: delay}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	b.ln, b.addr = ln, ln.Addr().String()
	body := make([]byte, 1024)
	b.srv = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.hits.Add(1)
		if b.delay > 0 {
			time.Sleep(b.delay)
		}
		w.Header().Set("X-Backend", fmt.Sprint(b.id))
		w.Write(body)
	})}
	go b.srv.Serve(ln)
	return b
}

// ---------------- L4 プロキシ ----------------
func startL4(backends []*backend) string {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	var n atomic.Int64
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				b := backends[int(n.Add(1))%len(backends)]
				s, err := net.Dial("tcp", b.addr)
				if err != nil {
					return
				}
				defer s.Close()
				done := make(chan struct{}, 2)
				go func() { io.Copy(s, c); s.(*net.TCPConn).CloseWrite(); done <- struct{}{} }()
				go func() { io.Copy(c, s); done <- struct{}{} }()
				<-done
			}(c)
		}
	}()
	return ln.Addr().String()
}

// ---------------- L7 プロキシ ----------------
type lbTransport struct {
	backends []*backend
	base     http.RoundTripper
	policy   string // "rr" | "lc"
	retry    bool
	health   bool
	rr       atomic.Int64
}

func (t *lbTransport) pick(skip map[int]bool) *backend {
	now := time.Now().UnixNano()
	var cands []*backend
	for _, b := range t.backends {
		if skip[b.id] || (t.health && b.downTil.Load() > now) {
			continue
		}
		cands = append(cands, b)
	}
	if len(cands) == 0 {
		return nil
	}
	if t.policy == "lc" {
		best := cands[0]
		for _, b := range cands[1:] {
			if b.active.Load() < best.active.Load() {
				best = b
			}
		}
		return best
	}
	return cands[int(t.rr.Add(1))%len(cands)]
}

func (t *lbTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	skip := map[int]bool{}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		b := t.pick(skip)
		if b == nil {
			break
		}
		r2 := req.Clone(req.Context())
		r2.URL.Scheme, r2.URL.Host = "http", b.addr
		b.active.Add(1)
		resp, err := t.base.RoundTrip(r2)
		if err == nil {
			resp.Body = &countBody{ReadCloser: resp.Body, done: func() { b.active.Add(-1) }}
			return resp, nil
		}
		b.active.Add(-1)
		lastErr = err
		if t.health {
			b.downTil.Store(time.Now().Add(2 * time.Second).UnixNano()) // 失敗したら、しばらく外す
		}
		if !t.retry {
			return nil, err
		}
		skip[b.id] = true // 別のバックエンドで、再試行 (GET は冪等なので、安全)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("使えるバックエンドがない")
	}
	return nil, lastErr
}

type countBody struct {
	io.ReadCloser
	once sync.Once
	done func()
}

func (c *countBody) Close() error { c.once.Do(c.done); return c.ReadCloser.Close() }

func startL7(backends []*backend, policy string, keepAlive, retry, health bool) string {
	base := &http.Transport{MaxIdleConnsPerHost: 256, DisableKeepAlives: !keepAlive}
	lb := &lbTransport{backends: backends, base: base, policy: policy, retry: retry, health: health}
	rp := &httputil.ReverseProxy{
		Director:  func(r *http.Request) {},
		Transport: lb,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.WriteHeader(http.StatusBadGateway)
		},
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	go http.Serve(ln, rp)
	return ln.Addr().String()
}

// ---------------- 負荷 ----------------
type stats struct {
	n, errs  int64
	mean     time.Duration
	p50, p99 time.Duration
	dur      time.Duration
}

func load(addr string, conc int, dur time.Duration, onTick func(elapsed time.Duration)) stats {
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: conc, MaxConnsPerHost: conc}, Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	var mu sync.Mutex
	var lats []time.Duration
	var errs atomic.Int64
	var wg sync.WaitGroup
	start := time.Now()
	stop := start.Add(dur)
	if onTick != nil {
		go onTick(0)
	}
	for i := 0; i < conc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var local []time.Duration
			for time.Now().Before(stop) {
				t0 := time.Now()
				resp, err := client.Get("http://" + addr + "/")
				if err != nil {
					errs.Add(1)
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode != 200 {
					errs.Add(1)
					continue
				}
				local = append(local, time.Since(t0))
			}
			mu.Lock()
			lats = append(lats, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	el := time.Since(start)
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	var sum time.Duration
	for _, l := range lats {
		sum += l
	}
	s := stats{n: int64(len(lats)), errs: errs.Load(), dur: el}
	if len(lats) > 0 {
		s.mean, s.p50, s.p99 = sum/time.Duration(len(lats)), lats[len(lats)/2], lats[len(lats)*99/100]
	}
	return s
}

func us(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e3 }

func partA() {
	fmt.Println("A. 経路ごとの、遅延とスループット (バックエンドの処理時間 0。32 並行、各 2 秒)")
	bs := []*backend{startBackend(0, 0), startBackend(1, 0), startBackend(2, 0)}
	type c struct {
		name string
		addr string
	}
	cases := []c{
		{"直接 (1 台のバックエンド)", bs[0].addr},
		{"L4 プロキシ (3 台に、接続ごとにラウンドロビン)", startL4(bs)},
		{"L7 プロキシ (バックエンドへの接続を使い回す)", startL7(bs, "rr", true, false, false)},
		{"L7 プロキシ (バックエンドへ、毎回、新しい接続)", startL7(bs, "rr", false, false, false)},
	}
	fmt.Printf("  %-52s %12s %10s %10s %10s %6s\n", "経路", "リクエスト/秒", "平均", "p50", "p99", "失敗")
	for _, cs := range cases {
		s := load(cs.addr, 32, 2*time.Second, nil)
		fmt.Printf("  %-52s %12.0f %7.0f µs %7.0f µs %7.0f µs %6d\n", cs.name, float64(s.n)/s.dur.Seconds(), us(s.mean), us(s.p50), us(s.p99), s.errs)
	}
	for _, b := range bs {
		b.srv.Close()
	}
}

func partB() {
	fmt.Println("\nB. バックエンドの 1 台が遅いとき (1 台: 50 ms、他の 2 台: 1 ms)。32 並行、3 秒")
	fmt.Printf("  %-30s %12s %10s %10s   各バックエンドへの振り分け (速い, 速い, 遅い)\n", "方式", "リクエスト/秒", "平均", "p99")
	for _, pol := range []struct{ name, p string }{{"ラウンドロビン", "rr"}, {"最小接続", "lc"}} {
		bs := []*backend{startBackend(0, time.Millisecond), startBackend(1, time.Millisecond), startBackend(2, 50*time.Millisecond)}
		addr := startL7(bs, pol.p, true, false, false)
		s := load(addr, 32, 3*time.Second, nil)
		fmt.Printf("  %-30s %12.0f %7.1f ms %7.1f ms   %d, %d, %d\n", pol.name, float64(s.n)/s.dur.Seconds(), us(s.mean)/1000, us(s.p99)/1000, bs[0].hits.Load(), bs[1].hits.Load(), bs[2].hits.Load())
		for _, b := range bs {
			b.srv.Close()
		}
	}
}

func partC() {
	fmt.Println("\nC. バックエンドの 1 台が、1 秒後に落ちる (3 秒間、32 並行、処理時間 1 ms)")
	fmt.Printf("  %-44s %10s %8s %10s\n", "プロキシの設定", "成功", "失敗", "失敗の割合")
	for _, cfg := range []struct {
		name          string
		retry, health bool
	}{{"再試行なし、ヘルスチェックなし", false, false}, {"再試行あり、ヘルスチェックなし", true, false}, {"再試行あり、ヘルスチェックあり (失敗したら 2 秒外す)", true, true}} {
		bs := []*backend{startBackend(0, time.Millisecond), startBackend(1, time.Millisecond), startBackend(2, time.Millisecond)}
		addr := startL7(bs, "rr", true, cfg.retry, cfg.health)
		go func() {
			time.Sleep(time.Second)
			bs[1].srv.Close() // 1 台が落ちる (接続も、受け付けなくなる)
			bs[1].ln.Close()
		}()
		s := load(addr, 32, 3*time.Second, nil)
		fmt.Printf("  %-44s %10d %8d %9.2f%%\n", cfg.name, s.n, s.errs, float64(s.errs)/float64(s.n+s.errs)*100)
		for _, b := range bs {
			b.srv.Close()
		}
	}
}

func main() {
	partA()
	partB()
	partC()
}
