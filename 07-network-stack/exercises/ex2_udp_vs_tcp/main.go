// ex2: UDP と TCP の違いを、ソケットで確かめる
//
//	A. メッセージの境界:  UDP は、1回の送信 = 1回の受信 (境界が保たれる)。TCP は、バイトの流れで、境界がない。
//	B. 受信バッファからのあふれ: UDP は、受信側が遅く、バッファが一杯になると、黙って捨てる。
//	C. 1つのデータグラムの最大サイズ: OS の設定によって、上限が違う。
//
// 実行: go run .
package main

import (
	"fmt"
	"net"
	"time"
)

func partA() {
	fmt.Println("A. メッセージの境界")
	// UDP
	srv, _ := net.ListenPacket("udp", "127.0.0.1:0")
	defer srv.Close()
	cli, _ := net.Dial("udp", srv.LocalAddr().String())
	for _, n := range []int{10, 20, 30} {
		cli.Write(make([]byte, n))
	}
	time.Sleep(50 * time.Millisecond)
	buf := make([]byte, 100)
	fmt.Print("  UDP: 10, 20, 30 バイトを3回送信 → 受信: ")
	srv.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	for {
		n, _, err := srv.ReadFrom(buf)
		if err != nil {
			break
		}
		fmt.Printf("%d バイト ", n)
	}
	fmt.Println("(送ったとおりの境界で届いた)")

	// TCP
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	done := make(chan []int)
	go func() {
		c, _ := ln.Accept()
		time.Sleep(100 * time.Millisecond) // 3回の送信が、すべて届いてから読む
		var sizes []int
		c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		for {
			n, err := c.Read(buf)
			if err != nil {
				break
			}
			sizes = append(sizes, n)
		}
		done <- sizes
	}()
	tc, _ := net.Dial("tcp", ln.Addr().String())
	for _, n := range []int{10, 20, 30} {
		tc.Write(make([]byte, n))
	}
	sizes := <-done
	fmt.Printf("  TCP: 10, 20, 30 バイトを3回送信 → 受信: %v バイトずつ (境界はない。読み手が、メッセージの区切りを決める必要がある)\n", sizes)
	tc.Close()
}

func partB() {
	fmt.Println("\nB. UDP の受信バッファあふれ (512 バイト × 10,000 個を一気に送り、受信側は 300 ms 読まない)")
	for _, rcv := range []int{8 << 10, 64 << 10, 1 << 20, 8 << 20} {
		srv, _ := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
		srv.SetReadBuffer(rcv)
		cli, _ := net.Dial("udp", srv.LocalAddr().String())
		payload := make([]byte, 512)
		for i := 0; i < 10000; i++ {
			cli.Write(payload)
		}
		time.Sleep(300 * time.Millisecond)
		got := 0
		buf := make([]byte, 2048)
		srv.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		for {
			if _, _, err := srv.ReadFrom(buf); err != nil {
				break
			}
			got++
		}
		fmt.Printf("  受信バッファ %8d バイト: 10,000 個のうち %5d 個を受信 (損失 %5.1f%%)\n", rcv, got, float64(10000-got)/100)
		srv.Close()
		cli.Close()
	}
}

func partC() {
	fmt.Println("\nC. 1つのデータグラムの最大サイズ (ループバックへ送信)")
	srv, _ := net.ListenPacket("udp", "127.0.0.1:0")
	defer srv.Close()
	cli, _ := net.Dial("udp", srv.LocalAddr().String())
	for _, n := range []int{1000, 1472, 9000, 9216, 9217, 20000, 65507, 65508} {
		_, err := cli.Write(make([]byte, n))
		status := "送信できた"
		if err != nil {
			status = "失敗: " + err.Error()
		}
		fmt.Printf("  %6d バイト: %s\n", n, status)
	}
	fmt.Println("  (IPv4 + UDP の理論上の最大データ = 65535 − 20 − 8 = 65507 バイト)")
}

func main() {
	partA()
	partB()
	partC()
}
