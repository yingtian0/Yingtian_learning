// ex4: HTTP の、実際にネットワークを流れるバイトを読む
//
//	A. HTTP/1.1 を、生の TCP で話す: Content-Length と chunked (分割転送) の違い。1回の送信で、2つのリクエストを送る (パイプライン)。
//	B. HTTP/2 を、生の TLS で話す: プリフェースと、バイナリのフレーム (SETTINGS、HEADERS、DATA…) を、自分で組み立てて、読む。
//	C. HTTP/1.1 の、メッセージの区切りが、曖昧になるとき: Content-Length と Transfer-Encoding の両方を持つメッセージを、
//	   「2つの解釈」で読むと、リクエストの数が食い違う (リクエストスマグリングの原理)。実際の Go のサーバーは、どう扱うか。
//
// C のパーサーは、概念の説明のための、メモリ内の簡易版 (どこにも接続しない)。実在のサーバーを、攻撃する道具ではない。
// 実行: go run ./ex4_http_wire
package main

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ex9web/internal/pki"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func visible(s string) string { return strings.ReplaceAll(s, "\r\n", "\\r\\n\n    ") }

// ---------------- A ----------------
func partA() {
	fmt.Println("A. HTTP/1.1 を、生の TCP で話す")
	mux := http.NewServeMux()
	mux.HandleFunc("/fixed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "11")
		w.Write([]byte("hello world"))
	})
	mux.HandleFunc("/chunked", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
		w.(http.Flusher).Flush() // ここで、まだ全体の長さが分からないまま、送り始める
		time.Sleep(20 * time.Millisecond)
		w.Write([]byte(" world"))
	})
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go http.Serve(ln, mux)

	c, _ := net.Dial("tcp", ln.Addr().String())
	defer c.Close()
	req := "GET /fixed HTTP/1.1\r\nHost: x\r\n\r\n" + "GET /chunked HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"
	fmt.Printf("  送信 (2 つのリクエストを、1 回の write で。%d バイト):\n    %s\n", len(req), visible(req))
	c.Write([]byte(req))
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp, _ := io.ReadAll(c)
	out := string(resp)
	// 日時は、毎回違うので、除く
	var lines []string
	for _, l := range strings.Split(out, "\r\n") {
		if strings.HasPrefix(l, "Date:") {
			l = "Date: (省略)"
		}
		lines = append(lines, l)
	}
	fmt.Printf("  受信 (%d バイト):\n    %s\n", len(resp), visible(strings.Join(lines, "\r\n")))
}

// ---------------- B ----------------
var frameTypes = map[byte]string{0: "DATA", 1: "HEADERS", 2: "PRIORITY", 3: "RST_STREAM", 4: "SETTINGS", 5: "PUSH_PROMISE", 6: "PING", 7: "GOAWAY", 8: "WINDOW_UPDATE", 9: "CONTINUATION"}
var settingNames = map[uint16]string{1: "HEADER_TABLE_SIZE", 2: "ENABLE_PUSH", 3: "MAX_CONCURRENT_STREAMS", 4: "INITIAL_WINDOW_SIZE", 5: "MAX_FRAME_SIZE", 6: "MAX_HEADER_LIST_SIZE"}

func frame(t, flags byte, stream uint32, payload []byte) []byte {
	b := make([]byte, 9, 9+len(payload))
	b[0], b[1], b[2] = byte(len(payload)>>16), byte(len(payload)>>8), byte(len(payload))
	b[3], b[4] = t, flags
	binary.BigEndian.PutUint32(b[5:], stream&0x7fffffff)
	return append(b, payload...)
}

func partB() {
	fmt.Println("\nB. HTTP/2 を、生の TLS で話す (フレームを自分で組み立てる)")
	root, err := pki.NewRoot("演習用ルート", "ecdsa")
	must(err)
	leaf, err := root.IssueLeaf("localhost", []string{"localhost"}, "ecdsa", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	must(err)
	srv := &http.Server{
		Handler:   http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("hello from h2")) }),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{pki.Chain(leaf)}},
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	go srv.ServeTLS(ln, "", "")
	defer srv.Close()

	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{RootCAs: pki.Pool(root), ServerName: "localhost", NextProtos: []string{"h2"}})
	must(err)
	defer conn.Close()
	fmt.Printf("  ALPN で決まったプロトコル: %q\n", conn.ConnectionState().NegotiatedProtocol)

	preface := "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
	conn.Write([]byte(preface))
	conn.Write(frame(4, 0, 0, nil)) // 空の SETTINGS
	// HPACK: 0x82 = :method GET (静的表の 2 番)、0x87 = :scheme https (7 番)、0x84 = :path / (4 番)、
	//        0x41 = 「:authority (静的表の 1 番) を名前に持つ、リテラルの値」、続いて、長さ 9 と "localhost"
	block := append([]byte{0x82, 0x87, 0x84, 0x41, 9}, []byte("localhost")...)
	conn.Write(frame(1, 0x5, 1, block)) // HEADERS、END_STREAM | END_HEADERS、ストリーム 1
	http1 := "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n"
	fmt.Printf("  送信: プリフェース %d B + SETTINGS フレーム 9 B + HEADERS フレーム (9 B のフレームヘッダ + %d B のヘッダブロック)\n", len(preface), len(block))
	fmt.Printf("  (同じリクエストを、HTTP/1.1 のテキストで書くと %d バイト。HTTP/2 は、ヘッダを静的表の番号 (1 バイト) で表す)\n", len(http1))

	fmt.Println("  受信したフレーム:")
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		hdr := make([]byte, 9)
		if _, err := io.ReadFull(conn, hdr); err != nil {
			break
		}
		n := int(hdr[0])<<16 | int(hdr[1])<<8 | int(hdr[2])
		payload := make([]byte, n)
		if _, err := io.ReadFull(conn, payload); err != nil {
			break
		}
		t, flags := hdr[3], hdr[4]
		stream := binary.BigEndian.Uint32(hdr[5:]) & 0x7fffffff
		note := ""
		switch t {
		case 4:
			if flags&1 != 0 {
				note = "(確認応答)"
			} else {
				var ps []string
				for i := 0; i+6 <= len(payload); i += 6 {
					id := binary.BigEndian.Uint16(payload[i:])
					ps = append(ps, fmt.Sprintf("%s=%d", settingNames[id], binary.BigEndian.Uint32(payload[i+2:])))
				}
				note = strings.Join(ps, ", ")
				conn.Write(frame(4, 1, 0, nil)) // 確認応答を返す
			}
		case 1:
			if len(payload) > 0 && payload[0] == 0x88 {
				note = "先頭バイト 0x88 = 静的表の 8 番 = ':status 200'"
			}
		case 0:
			note = fmt.Sprintf("本体: %q", string(payload))
		case 8:
			note = fmt.Sprintf("増加量 %d", binary.BigEndian.Uint32(payload))
		}
		fmt.Printf("    %-14s flags=0x%02x  長さ=%-5d ストリーム=%d  %s\n", frameTypes[t], flags, n, stream, note)
	}
}

// ---------------- C ----------------
type request struct{ method, path, body string }

// preferTE: true なら Transfer-Encoding を優先、false なら Content-Length を優先する。
func parse(data string, preferTE bool) (reqs []request, rest string) {
	for {
		idx := strings.Index(data, "\r\n\r\n")
		if idx < 0 {
			return reqs, data
		}
		lines := strings.Split(data[:idx], "\r\n")
		start := strings.Fields(lines[0])
		hdr := map[string]string{}
		for _, l := range lines[1:] {
			if k, v, ok := strings.Cut(l, ":"); ok {
				hdr[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
			}
		}
		rest := data[idx+4:]
		cl, errCL := strconv.Atoi(hdr["content-length"])
		hasCL := errCL == nil
		te := strings.EqualFold(hdr["transfer-encoding"], "chunked")
		body := ""
		if te && (preferTE || !hasCL) {
			for {
				nl := strings.Index(rest, "\r\n")
				if nl < 0 {
					return reqs, data
				}
				size, err := strconv.ParseInt(rest[:nl], 16, 32)
				if err != nil {
					return reqs, data
				}
				rest = rest[nl+2:]
				if size == 0 {
					if len(rest) >= 2 {
						rest = rest[2:]
					}
					break
				}
				if int(size)+2 > len(rest) {
					return reqs, data
				}
				body += rest[:size]
				rest = rest[size+2:]
			}
		} else if hasCL {
			if cl > len(rest) {
				return reqs, data
			}
			body, rest = rest[:cl], rest[cl:]
		}
		reqs = append(reqs, request{start[0], start[1], body})
		data = rest
	}
}

func partC() {
	fmt.Println("\nC. メッセージの区切りが曖昧なとき (リクエストスマグリングの原理。メモリ内の、簡易パーサーでの説明)")
	inner := "GET /admin HTTP/1.1\r\nHost: x\r\n\r\n"
	body := "0\r\n\r\n" + inner
	stream := "POST /a HTTP/1.1\r\nHost: x\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\nTransfer-Encoding: chunked\r\n\r\n" + body
	fmt.Printf("  1 本のバイト列 (%d バイト。Content-Length と Transfer-Encoding の両方を持つ):\n    %s\n", len(stream), visible(stream))
	front, restF := parse(stream, false)
	back, restB := parse(stream, true)
	fmt.Printf("  前段 (Content-Length を優先する) の解釈: リクエスト %d 個 %v  残り %d バイト\n", len(front), paths(front), len(restF))
	fmt.Printf("  後段 (Transfer-Encoding を優先する) の解釈: リクエスト %d 個 %v  残り %d バイト\n", len(back), paths(back), len(restB))
	fmt.Println("  → 前段は「1 個の POST」と見て、そのまま通す。後段は「POST と、続く GET /admin」の 2 個と見る。")
	fmt.Println("    前段の検査 (アクセス制御など) を、通っていない GET /admin が、後段に届く。これが、スマグリング。")

	fmt.Println("\n  実際の Go の net/http のサーバーは、この両方を持つメッセージを、どう扱うか (自分で起動した、ループバックのサーバーに送る):")
	var served []string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		served = append(served, r.Method+" "+r.URL.Path)
		io.Copy(io.Discard, r.Body)
		w.Write([]byte("ok"))
	})
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go http.Serve(ln, mux)
	c, _ := net.Dial("tcp", ln.Addr().String())
	defer c.Close()
	c.Write([]byte(stream))
	c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	resp, _ := io.ReadAll(c)
	first, _, _ := strings.Cut(string(resp), "\r\n")
	fmt.Printf("    応答の 1 行目: %q\n", first)
	fmt.Printf("    サーバーが処理したリクエスト: %v\n", served)
}

func paths(rs []request) []string {
	var p []string
	for _, r := range rs {
		p = append(p, r.method+" "+r.path)
	}
	return p
}

func main() {
	partA()
	partB()
	partC()
}
