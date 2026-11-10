// ex5: HTTP の、キャッシュ (条件付きリクエスト) と、圧縮
//
//	A. ETag と If-None-Match: 変わっていなければ、本体を送らない (304)。送られたバイト数を数える。
//	B. 圧縮 (gzip): テキスト、JSON、ランダム (圧縮できない) データで、大きさと、CPU 時間 (レベル 1 / 6 / 9) を比べる。
//
// 実行: go run ./ex5_cache_compress
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type countConn struct {
	net.Conn
	in, out *atomic.Int64
}

func (c countConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.in.Add(int64(n))
	return n, err
}
func (c countConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	c.out.Add(int64(n))
	return n, err
}

func sample(kind string, n int) []byte {
	switch kind {
	case "text":
		var b strings.Builder
		words := []string{"ネットワーク", "プロトコル", "the", "of", "packet", "connection", "server", "client", "request", "response", "header", "latency"}
		for b.Len() < n {
			b.WriteString(words[b.Len()*7%len(words)])
			b.WriteByte(' ')
		}
		return []byte(b.String()[:n])
	case "json":
		var b strings.Builder
		b.WriteString("[")
		for i := 0; b.Len() < n; i++ {
			fmt.Fprintf(&b, `{"id":%d,"name":"user%d","active":%v,"score":%d},`, i, i, i%3 == 0, i*37%1000)
		}
		return []byte(b.String()[:n])
	default:
		b := make([]byte, n)
		rand.Read(b)
		return b
	}
}

func gz(data []byte, level int) []byte {
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, level)
	w.Write(data)
	w.Close()
	return buf.Bytes()
}

func partA() {
	fmt.Println("A. ETag と条件付きリクエスト (200 KB のリソース)")
	data := sample("json", 200*1024)
	sum := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	mux := http.NewServeMux()
	mux.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-cache") // 使う前に、必ず確認する (保存はしてよい)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Write(data)
	})
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go http.Serve(ln, mux)

	var in, out atomic.Int64
	client := &http.Client{Transport: &http.Transport{
		DisableCompression: true,
		Dial: func(network, addr string) (net.Conn, error) {
			c, err := net.Dial(network, addr)
			return countConn{c, &in, &out}, err
		},
	}}
	do := func(label string, hdr map[string]string) {
		in.Store(0)
		out.Store(0)
		req, _ := http.NewRequest("GET", "http://"+ln.Addr().String()+"/data", nil)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("  %-32s → %d、本体 %6d B、ネットワークで受信 %6d B、送信 %4d B\n", label, resp.StatusCode, len(body), in.Load(), out.Load())
	}
	do("1回目 (キャッシュなし)", nil)
	do("2回目 (If-None-Match を付ける)", map[string]string{"If-None-Match": etag})
	do("ETag が古い (If-None-Match が不一致)", map[string]string{"If-None-Match": `"old"`})
}

func partB() {
	fmt.Println("\nB. 圧縮 (gzip): データの種類と、圧縮のレベル (各 1 MiB)")
	fmt.Printf("  %-8s %-10s %12s %8s %12s %14s\n", "データ", "レベル", "圧縮後", "比率", "圧縮の時間", "圧縮の速度")
	for _, kind := range []string{"text", "json", "random"} {
		data := sample(kind, 1<<20)
		for _, lv := range []int{1, 6, 9} {
			t0 := time.Now()
			var out []byte
			for i := 0; i < 5; i++ {
				out = gz(data, lv)
			}
			el := time.Since(t0) / 5
			fmt.Printf("  %-8s level %-4d %9d B %7.1f%% %9.1f ms %10.0f MB/s\n", kind, lv, len(out), float64(len(out))/float64(len(data))*100,
				float64(el.Microseconds())/1000, float64(len(data))/1e6/el.Seconds())
		}
	}
	fmt.Println("  (random は、圧縮できない。圧縮済みの画像や動画に、圧縮をかけても、無駄になる。)")
}

func main() {
	partA()
	partB()
}
