// ex2: Nagle のアルゴリズムと、遅延 ACK の相互作用
//
// クライアントは、リクエストを「ヘッダ (4 バイト)」と「本体 (100 バイト)」の 2 回の write で送り、
// サーバーは、全部 (104 バイト) を受け取ってから、1 バイトの応答を返す。
//
//	Nagle のアルゴリズム: 未確認 (ACK が来ていない) のデータがあるとき、小さいデータは、すぐには送らず、溜めておく。
//	遅延 ACK            : 受け取った側は、ACK を、すぐには返さず、少し待つ (応答に便乗するため)。
//
// 2つが組み合わさると、「2回目の write は、1回目の ACK を待つ」 → 「ACK は、遅れて返される」 → 止まる。
//
// 比べる方法:
//  1. Nagle あり (TCP_NODELAY なし)、2 回の write
//  2. Nagle なし (TCP_NODELAY)、2 回の write
//  3. Nagle あり、1 回の write (ヘッダと本体をまとめて送る)
//
// Go は、既定で TCP_NODELAY を有効にしている。ここでは、SetNoDelay(false) で、Nagle を有効にする。
//
// 実行: go run .
package main

import (
	"fmt"
	"io"
	"net"
	"sort"
	"time"
)

func server(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			buf := make([]byte, 104)
			for {
				if _, err := io.ReadFull(c, buf); err != nil {
					return
				}
				c.Write([]byte{'k'})
			}
		}(c)
	}
}

func run(name string, nodelay bool, combine bool, n int) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go server(ln)
	c, _ := net.Dial("tcp", ln.Addr().String())
	defer c.Close()
	c.(*net.TCPConn).SetNoDelay(nodelay)
	hdr := make([]byte, 4)
	body := make([]byte, 100)
	all := make([]byte, 104)
	resp := make([]byte, 1)

	lat := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		t0 := time.Now()
		if combine {
			c.Write(all)
		} else {
			c.Write(hdr)
			c.Write(body)
		}
		io.ReadFull(c, resp)
		lat = append(lat, float64(time.Since(t0).Microseconds())/1000)
	}
	sort.Float64s(lat)
	var sum float64
	for _, l := range lat {
		sum += l
	}
	fmt.Printf("  %-44s 平均 %8.3f ms   中央値 %8.3f ms   最大 %8.3f ms\n", name, sum/float64(n), lat[n/2], lat[n-1])
}

func main() {
	fmt.Println("リクエスト (104 バイト) → 応答 (1 バイト) の往復時間 (ループバック、各 100 回)")
	run("1. Nagle あり、2 回の write (ヘッダ、本体)", false, false, 100)
	run("2. Nagle なし (TCP_NODELAY)、2 回の write", true, false, 100)
	run("3. Nagle あり、1 回の write (まとめて送る)", false, true, 100)
}
