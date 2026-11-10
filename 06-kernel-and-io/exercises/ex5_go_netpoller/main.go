// ex5: Go のネットワーク I/O (netpoller) と、ブロックするシステムコールの違い
//
// Go の net.Conn の Read は、書き方は「ブロックする」が、中身は違う。
// ランタイムは、ソケットを非ブロックにして、kqueue / epoll に登録し、
// 待っている goroutine を「眠らせる」だけで、OS スレッドは占有しない (第3回)。
//
//	A. 3000 本の接続を、それぞれ goroutine が Read で待つ  → OS スレッドは増えるか?
//	B. 200 本のパイプを、生のシステムコール (syscall.Read) で待つ goroutine → OS スレッドは増えるか?
//	C. 暇な接続が大量にあっても、活発な接続 10 本の往復は、速いままか?
//
// 実行: go run . [暇な接続の数 (既定 3000)]
package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func numThreads() int {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/self/status")
		if err != nil {
			return -1
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "Threads:") {
				n, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Threads:")))
				return n
			}
		}
		return -1
	}
	out, err := exec.Command("ps", "-M", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return -1
	}
	return strings.Count(string(out), "\n") - 1 // 先頭の見出しの行を除く
}

func report(label string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("  %-44s OS スレッド %4d   goroutine %6d   スタック使用 %6.1f MB   Sys %6.1f MB\n",
		label, numThreads(), runtime.NumGoroutine(), float64(m.StackInuse)/(1<<20), float64(m.Sys)/(1<<20))
}

func main() {
	idle := 3000
	if len(os.Args) > 1 {
		idle, _ = strconv.Atoi(os.Args[1])
	}
	report("起動直後")

	// ---- A. 暇な接続 ----
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	var accepted sync.WaitGroup
	accepted.Add(idle + 10)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) { // 1 接続 = 1 goroutine。書き方は、ブロックする Read
				accepted.Done()
				buf := make([]byte, 64)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					c.Write(buf[:n])
				}
			}(c)
		}
	}()
	conns := make([]net.Conn, 0, idle+10)
	for i := 0; i < idle+10; i++ {
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			fmt.Println("接続できなかった:", err, "(", i, "本まで)")
			break
		}
		conns = append(conns, c)
	}
	time.Sleep(300 * time.Millisecond)
	fmt.Printf("\nA. %d 本の接続を、それぞれ goroutine が Read で待っている\n", len(conns))
	report("A. 接続の goroutine が、全員 Read で待機中")

	// ---- B. ブロックするシステムコール ----
	const npipes = 200
	var wfds []int
	for i := 0; i < npipes; i++ {
		var p [2]int
		if err := syscall.Pipe(p[:]); err != nil {
			panic(err)
		}
		wfds = append(wfds, p[1])
		go func(rfd int) {
			buf := make([]byte, 1)
			syscall.Read(rfd, buf) // 生のシステムコール。データが来るまで、スレッドごとブロックする
		}(p[0])
	}
	time.Sleep(500 * time.Millisecond)
	fmt.Printf("\nB. %d 本のパイプを、生の syscall.Read で待つ goroutine を追加\n", npipes)
	report("B. ブロックする syscall.Read が 200 本")
	for _, w := range wfds {
		syscall.Write(w, []byte{1}) // 解放する
	}
	time.Sleep(300 * time.Millisecond)
	report("B. 解放した後")

	// ---- C. 活発な接続 10 本の往復 ----
	fmt.Printf("\nC. 暇な接続 %d 本があるまま、活発な 10 本が往復する (1.5 秒)\n", len(conns)-10)
	var total int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	stop := time.Now().Add(1500 * time.Millisecond)
	for i := 0; i < 10; i++ {
		c := conns[len(conns)-1-i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 1)
			n := 0
			for time.Now().Before(stop) {
				c.Write([]byte{'x'})
				if _, err := c.Read(buf); err != nil {
					return
				}
				n++
			}
			mu.Lock()
			total += int64(n)
			mu.Unlock()
		}()
	}
	wg.Wait()
	rps := float64(total) / 1.5
	fmt.Printf("  %.0f 往復/秒、平均 %.1f µs\n", rps, 10/rps*1e6)
	report("C. 実行後")
	for _, c := range conns {
		c.Close()
	}
}
