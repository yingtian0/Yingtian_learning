// ex1: TCP の接続の状態と、接続のコスト
//
//	A. 接続の一生を、lsof で観察する。
//	   LISTEN → ESTABLISHED → (先に閉じた側) FIN_WAIT_2 / (後から閉じる側) CLOSE_WAIT
//	   (TIME_WAIT は、カーネルだけが持つ状態で、プロセスに属さないため、lsof には出ない。B で動作から確かめる。)
//	B. TIME_WAIT は、「先に閉じた側」に残る。
//	   サーバーが先に閉じると、サーバーのポートに TIME_WAIT が残り、SO_REUSEADDR なしでは、同じポートで待ち受けられない。
//	   クライアントが先に閉じると、残るのはクライアントの (使い捨ての) ポートで、サーバーは、すぐ再起動できる。
//	C. 接続の確立 (3ウェイハンドシェイク) のコスト:
//	   毎回新しい接続を張る場合と、1本の接続を使い回す場合で、1回のリクエストの時間を比べる (ループバック)。
//
// 注意: A は lsof を使う。この環境では、netstat が、ループバックの接続を表示しなかったため。
// 実行: go run .
package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func lsofStates(port int) []string {
	out, _ := exec.Command("lsof", "-nP", "-a", "-p", fmt.Sprint(os.Getpid()), "-iTCP").CombinedOutput()
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, fmt.Sprintf(":%d", port)) {
			f := strings.Fields(l)
			if len(f) >= 9 {
				lines = append(lines, "    "+strings.Join(f[8:], " "))
			}
		}
	}
	return lines
}

func show(title string, port int) {
	fmt.Println("  " + title)
	ls := lsofStates(port)
	if len(ls) == 0 {
		fmt.Println("    (このプロセスが持つ接続は、もう無い。カーネルだけが、TIME_WAIT などの状態を持っている場合がある)")
	}
	for _, l := range ls {
		fmt.Println(l)
	}
}

func partA(serverCloseFirst bool) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	defer ln.Close()
	show("① listen した直後", port)

	accepted := make(chan net.Conn, 1)
	go func() { c, _ := ln.Accept(); accepted <- c }()
	cli, _ := net.Dial("tcp", ln.Addr().String())
	srv := <-accepted
	time.Sleep(50 * time.Millisecond)
	show("② 接続が確立した (サーバー側とクライアント側の、2 つの行)", port)

	first, second := net.Conn(cli), net.Conn(srv)
	name1, name2 := "クライアント", "サーバー"
	if serverCloseFirst {
		first, second = srv, cli
		name1, name2 = "サーバー", "クライアント"
	}
	first.Close()
	time.Sleep(100 * time.Millisecond)
	show(fmt.Sprintf("③ 先に閉じる側 (%s) が close した後。%s は、まだ close していない", name1, name2), port)
	second.Close()
	time.Sleep(100 * time.Millisecond)
	show(fmt.Sprintf("④ %s も close した後", name2), port)
}

// SO_REUSEADDR の有無を指定して、ポートを bind できるか試す
func tryBind(port int, reuse bool) error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	if reuse {
		syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	}
	return syscall.Bind(fd, &syscall.SockaddrInet4{Port: port, Addr: [4]byte{127, 0, 0, 1}})
}

func partB(serverCloseFirst bool) {
	// サーバーのソケットは、SO_REUSEADDR なしで作る (Go の net.Listen は、既定で有効にするため、syscall を直接使う)
	lfd, _ := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	syscall.Bind(lfd, &syscall.SockaddrInet4{Port: 0, Addr: [4]byte{127, 0, 0, 1}})
	syscall.Listen(lfd, 8)
	sa, _ := syscall.Getsockname(lfd)
	port := sa.(*syscall.SockaddrInet4).Port

	cli, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Println("接続できなかった:", err)
		return
	}
	cfd, _, _ := syscall.Accept(lfd)
	time.Sleep(50 * time.Millisecond)

	who := "クライアント"
	if serverCloseFirst {
		who = "サーバー"
		syscall.Close(cfd) // サーバーが先に閉じる
		time.Sleep(50 * time.Millisecond)
		cli.Close()
	} else {
		cli.Close() // クライアントが先に閉じる
		time.Sleep(50 * time.Millisecond)
		syscall.Close(cfd)
	}
	syscall.Close(lfd) // サーバーのプログラムを、止めた状態
	time.Sleep(100 * time.Millisecond)

	e1 := tryBind(port, false)
	e2 := tryBind(port, true)
	fmt.Printf("  %s が先に close。同じポート %d で、サーバーを再起動しようとすると:\n", who, port)
	fmt.Printf("    SO_REUSEADDR なし → %v\n", result(e1))
	fmt.Printf("    SO_REUSEADDR あり → %v\n", result(e2))
}

func result(err error) string {
	if err == nil {
		return "bind できた"
	}
	return "失敗: " + err.Error()
}

func partC() {
	fmt.Println("\nC. 接続の確立のコスト (ループバック、エコー 1 バイト、2000 回)")
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				b := make([]byte, 1)
				for {
					if _, err := c.Read(b); err != nil {
						return
					}
					c.Write(b)
				}
			}(c)
		}
	}()
	addr := ln.Addr().String()
	const n = 2000
	b := []byte{'x'}
	buf := make([]byte, 1)

	t0 := time.Now()
	var connect time.Duration
	for i := 0; i < n; i++ {
		s := time.Now()
		c, err := net.Dial("tcp", addr)
		if err != nil {
			fmt.Println("接続できなかった:", err)
			return
		}
		connect += time.Since(s)
		c.Write(b)
		c.Read(buf)
		c.(*net.TCPConn).SetLinger(0) // RST で閉じて、TIME_WAIT を残さない (ポートを使い果たさないため)
		c.Close()
	}
	perNew := time.Since(t0) / n

	c, _ := net.Dial("tcp", addr)
	t0 = time.Now()
	for i := 0; i < n; i++ {
		c.Write(b)
		c.Read(buf)
	}
	perReuse := time.Since(t0) / n
	c.Close()

	fmt.Printf("  毎回、新しい接続で 1 リクエスト:     %8.1f µs/回  (うち、Dial (SYN → SYN+ACK → ACK) に %.1f µs)\n", float64(perNew.Nanoseconds())/1e3, float64((connect/n).Nanoseconds())/1e3)
	fmt.Printf("  1 本の接続を、使い回して 1 リクエスト: %8.1f µs/回\n", float64(perReuse.Nanoseconds())/1e3)
	fmt.Printf("  → 新しい接続は、使い回しの %.1f 倍 (ループバックは、往復時間が 0 に近い。実際のネットワークでは、少なくとも 1 往復の RTT が、確実に上乗せされる)\n", float64(perNew)/float64(perReuse))
}

func main() {
	fmt.Println("A. 接続の状態 (lsof)")
	fmt.Println("\n[A-1] クライアントが先に close する場合")
	partA(false)
	fmt.Println("\n[A-2] サーバーが先に close する場合")
	partA(true)

	out, _ := exec.Command("sysctl", "-n", "net.inet.tcp.msl").Output()
	fmt.Printf("\nB. TIME_WAIT は、先に閉じた側に残る (TIME_WAIT は 2 × MSL の間。この環境の net.inet.tcp.msl = %s ミリ秒)\n", strings.TrimSpace(string(out)))
	partB(true)
	partB(false)

	partC()
}
