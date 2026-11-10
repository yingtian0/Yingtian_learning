// ex2: HTTP/1.1 と HTTP/2 を、往復時間 50 ms の環境で比べる
//
// 10 KB のリソースを 60 個、取得する。TLS (ALPN で、HTTP のバージョンを決める)。
// 往復時間 (RTT) 50 ms は、netsim で、人工的に加える (TCP の 3 ウェイハンドシェイクの分も、加える)。帯域の制限はない。
//
//  1. HTTP/1.1、keep-alive、接続 1 本、順番に
//  2. HTTP/1.1、keep-alive、接続 6 本 (ブラウザの典型)、並行
//  3. HTTP/1.1、リクエストごとに新しい接続、6 並行
//  4. HTTP/2、接続 1 本、60 個を同時に (多重化)
//     さらに、5 と 6: 「パケットが 1 つ失われて、再送を待つ 200 ms の間、その接続の後ろのデータが全部止まる」
//     (TCP のヘッド・オブ・ライン・ブロッキング) を、サーバー → クライアントの、1 回限りの停止として、模擬する。
//
// 実行: go run ./ex2_http_versions
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"ex9web/internal/netsim"
	"ex9web/internal/pki"
)

const (
	oneWay = 25 * time.Millisecond
	nReq   = 60
	body   = 10 * 1024
)

type result struct {
	total    time.Duration
	p50, p95 time.Duration
	conns    int64
	max      time.Duration
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func run(h2, keepAlive bool, maxConns, conc int, stall *netsim.Stall) result {
	root, err := pki.NewRoot("演習用ルート", "ecdsa")
	must(err)
	leaf, err := root.IssueLeaf("localhost", []string{"localhost"}, "ecdsa", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	must(err)
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	must(err)
	ln := &netsim.Listener{Listener: rawLn, Delay: oneWay, Stall: stall}
	payload := make([]byte, body)
	srv := &http.Server{
		ErrorLog:  log.New(os.Stderr, "[サーバー] ", 0),
		Handler:   http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(payload) }),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{pki.Chain(leaf)}},
	}
	go srv.ServeTLS(ln, "", "")
	defer srv.Close()

	var conns atomic.Int64
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		conns.Add(1)
		time.Sleep(2 * oneWay) // TCP の SYN → SYN+ACK の、1 RTT
		return netsim.Wrap(c, oneWay, nil), nil
	}
	transport := &http.Transport{
		DialContext:       dial,
		TLSClientConfig:   &tls.Config{RootCAs: pki.Pool(root), ServerName: "localhost"},
		DisableKeepAlives: !keepAlive,
		MaxConnsPerHost:   maxConns,
		ForceAttemptHTTP2: h2,
	}
	if !h2 {
		transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	}
	client := &http.Client{Transport: transport}
	defer transport.CloseIdleConnections()

	url := "https://" + rawLn.Addr().String() + "/r"
	var mu sync.Mutex
	var lats []time.Duration
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < nReq; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			t0 := time.Now()
			resp, err := client.Get(url)
			if err != nil {
				fmt.Println("エラー:", err)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			mu.Lock()
			lats = append(lats, time.Since(t0))
			mu.Unlock()
		}()
	}
	wg.Wait()
	total := time.Since(start)
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	return result{total, lats[len(lats)/2], lats[len(lats)*95/100], conns.Load(), lats[len(lats)-1]}
}

func main() {
	fmt.Printf("往復時間 %v、%d 個のリソース (各 %d KB)、TLS (TCP の接続の分も含む)\n\n", 2*oneWay, nReq, body/1024)
	fmt.Printf("  %-52s %9s %9s %9s %9s %6s\n", "条件", "全体", "p50", "p95", "最大", "接続数")
	cases := []struct {
		name      string
		h2, ka    bool
		maxConns  int
		conc      int
		withStall bool
	}{
		{"1. HTTP/1.1、keep-alive、接続 1 本、順番に", false, true, 1, 1, false},
		{"2. HTTP/1.1、keep-alive、接続 6 本、並行", false, true, 6, 6, false},
		{"3. HTTP/1.1、毎回、新しい接続、6 並行", false, false, 0, 6, false},
		{"4. HTTP/2、接続 1 本、60 個を同時に", true, true, 1, nReq, false},
		{"5. 2 に、1 回の停止 (200 ms) を加える (HTTP/1.1、6 本)", false, true, 6, 6, true},
		{"6. 4 に、1 回の停止 (200 ms) を加える (HTTP/2、1 本)", true, true, 1, nReq, true},
	}
	for ci, c := range cases {
		if len(os.Args) > 1 && os.Args[1] != fmt.Sprint(ci+1) { // 引数で、1 つのケースだけを実行できる
			continue
		}
		var st *netsim.Stall
		if c.withStall {
			st = &netsim.Stall{At: 100 * 1024, Dur: 200 * time.Millisecond}
		}
		r := run(c.h2, c.ka, c.maxConns, c.conc, st)
		fmt.Printf("  %-52s %7.0f ms %7.0f ms %7.0f ms %7.0f ms %6d\n", c.name, ms(r.total), ms(r.p50), ms(r.p95), ms(r.max), r.conns)
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
