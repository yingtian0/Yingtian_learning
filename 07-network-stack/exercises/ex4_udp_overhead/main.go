// ex4: パケット1個あたりのコストと、データの大きさ
//
// UDP のデータグラムの大きさを変えて、1 秒間、できるだけ多く送る。
//
//	「1 秒あたりのパケット数 (pps)」と「1 秒あたりのバイト数 (MB/s)」は、どう変わるか。
//
// 小さいパケットは、パケット数が多くても、運べるバイト数が少ない。
// 1 パケットごとのコスト (システムコール、IP/UDP の処理、割り込み) が、支配的だからである (第6回)。
//
// 注意: ループバックでの測定。送信と受信は、同じプロセスの、別々の goroutine。
// 実行: go run .
package main

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

func main() {
	fmt.Printf("%8s %14s %14s %12s %10s\n", "データ", "送信 pps", "受信 pps", "受信 MB/s", "損失")
	for _, size := range []int{16, 64, 256, 1024, 1472, 4096, 8192} {
		srv, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
		if err != nil {
			panic(err)
		}
		srv.SetReadBuffer(8 << 20)
		var received, recvBytes atomic.Int64
		done := make(chan struct{})
		go func() {
			buf := make([]byte, 16384)
			for {
				n, _, err := srv.ReadFromUDP(buf)
				if err != nil {
					close(done)
					return
				}
				received.Add(1)
				recvBytes.Add(int64(n))
			}
		}()

		cli, err := net.Dial("udp", srv.LocalAddr().String())
		if err != nil {
			panic(err)
		}
		payload := make([]byte, size)
		var sent int64
		start := time.Now()
		deadline := start.Add(time.Second)
		for time.Now().Before(deadline) {
			for i := 0; i < 100; i++ { // 時刻の確認の頻度を減らす
				if _, err := cli.Write(payload); err == nil {
					sent++
				}
			}
		}
		el := time.Since(start).Seconds()
		time.Sleep(200 * time.Millisecond)
		srv.Close()
		<-done
		cli.Close()
		rcv := received.Load()
		fmt.Printf("%7dB %14.0f %14.0f %12.1f %9.1f%%\n", size, float64(sent)/el, float64(rcv)/el,
			float64(recvBytes.Load())/1e6/el, float64(sent-rcv)/float64(sent)*100)
	}
}
