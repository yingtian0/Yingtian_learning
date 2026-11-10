// ex2: GC の「停止時間」と「遅延の裾」
//
//	A. 1 回の GC (mark) にかかる時間は、「生きているヒープの大きさ」と「ポインタの密度」で決まる。
//	   同じ 400 MB でも、ポインタを含まないデータ ([]byte) と、ポインタだらけのデータ (連結したノード) で、どれだけ違うか。
//	   また、stop-the-world (STW) の停止時間は、どれだけ短いか。
//	B. GC が動いている間の、アプリの遅延の裾: 1 ms ごとに起きるタイマーのずれと、1 回の処理の遅延 (p99、最大) を、
//	   ポインタを含まない / ポインタだらけ、の 2 つのヒープで比べる。
//
// 各測定は、別のプロセス (子) で行う。
// 実行: go run ./ex2_gc_latency
package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type node struct {
	next *node
	pad  [56]byte
}

var keep any
var sink []byte

func build(kind string, mb int) {
	switch kind {
	case "ptrfree": // ポインタを含まない: 1 MB の []byte を並べる
		bs := make([][]byte, 0, mb)
		for i := 0; i < mb; i++ {
			b := make([]byte, 1<<20)
			for j := 0; j < len(b); j += 4096 {
				b[j] = 1
			}
			bs = append(bs, b)
		}
		keep = bs
	case "ptrrich": // ポインタだらけ: 64 バイトのノードの連結
		n := mb << 20 / 64
		all := make([]*node, 0, n)
		var prev *node
		for i := 0; i < n; i++ {
			nd := &node{next: prev}
			all = append(all, nd)
			prev = nd
		}
		keep = all
	}
}

// ---------------- A ----------------
func childA(kind string, mb int) {
	debug.SetGCPercent(-1) // 自動の GC を止め、手動の GC だけを測る
	build(kind, mb)
	runtime.GC()
	var times []time.Duration
	for i := 0; i < 5; i++ {
		t0 := time.Now()
		runtime.GC() // 完了まで、ブロックする (mark + sweep)
		times = append(times, time.Since(t0))
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	var maxPause uint64
	n := int(ms.NumGC)
	if n > 256 {
		n = 256
	}
	for i := 0; i < n; i++ {
		if p := ms.PauseNs[(int(ms.NumGC)-1-i+256)%256]; p > maxPause {
			maxPause = p
		}
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	fmt.Printf("RESULT %.2f %d\n", float64(times[len(times)/2].Microseconds())/1000, maxPause/1000)
	runtime.KeepAlive(keep)
}

// ---------------- B ----------------
func pct(d []time.Duration, p float64) time.Duration {
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	return d[int(float64(len(d)-1)*p)]
}

func childB(kind string, mb int) {
	build(kind, mb)
	runtime.GC()
	var gcs0 runtime.MemStats
	runtime.ReadMemStats(&gcs0)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	// 割り当てを続ける goroutine (GC を、動かし続ける)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				b := make([]byte, 64<<10)
				b[0] = 1
				sink = b
			}
		}()
	}
	// タイマーのずれを測る goroutine
	var ticks []time.Duration
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			t0 := time.Now()
			time.Sleep(time.Millisecond)
			ticks = append(ticks, time.Since(t0)-time.Millisecond)
		}
	}()
	// 「リクエスト」の処理の遅延を測る (1 KB の割り当て + 少しの計算)
	var ops []time.Duration
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		t0 := time.Now()
		b := make([]byte, 1024)
		x := 0
		for i := 0; i < 2000; i++ {
			x += i * i
		}
		b[0] = byte(x)
		sink = b
		ops = append(ops, time.Since(t0))
		time.Sleep(200 * time.Microsecond)
	}
	close(stop)
	wg.Wait()
	var gcs1 runtime.MemStats
	runtime.ReadMemStats(&gcs1)
	us := func(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e3 }
	fmt.Printf("RESULT %d %.0f %.0f %.0f %.0f %.0f %.0f\n", gcs1.NumGC-gcs0.NumGC,
		us(pct(ticks, 0.5)), us(pct(ticks, 0.99)), us(pct(ticks, 1)), us(pct(ops, 0.5)), us(pct(ops, 0.99)), us(pct(ops, 1)))
	runtime.KeepAlive(keep)
}

func run(args ...string) []float64 {
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "GOMAXPROCS=4")
	out, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "RESULT ") {
			var v []float64
			for _, f := range strings.Fields(l)[1:] {
				x, _ := strconv.ParseFloat(f, 64)
				v = append(v, x)
			}
			return v
		}
	}
	panic("結果なし: " + string(out))
}

func main() {
	if len(os.Args) > 3 {
		mb, _ := strconv.Atoi(os.Args[3])
		switch os.Args[1] {
		case "A":
			childA(os.Args[2], mb)
		case "B":
			childB(os.Args[2], mb)
		}
		return
	}
	fmt.Println("A. 1 回の GC (mark + sweep) にかかる時間と、stop-the-world の停止 (GOMAXPROCS = 4、5 回の中央値)")
	fmt.Printf("  %-34s %16s %14s\n", "生きているヒープ", "GC 1 回の時間", "最大の STW 停止")
	for _, c := range []struct {
		kind string
		mb   int
		name string
	}{{"ptrfree", 400, "ポインタなし ([]byte) 400 MB"}, {"ptrrich", 100, "ポインタだらけ 100 MB"}, {"ptrrich", 400, "ポインタだらけ 400 MB"}, {"ptrrich", 800, "ポインタだらけ 800 MB"}} {
		v := run("A", c.kind, strconv.Itoa(c.mb))
		fmt.Printf("  %-34s %13.2f ms %11.0f µs\n", c.name, v[0], v[1])
	}
	fmt.Println("\nB. GC が、動き続けている間の、アプリの遅延 (3 秒。裏で 2 つの goroutine が、64 KB を割り当て続ける。GOMAXPROCS = 4)")
	fmt.Printf("  %-26s %8s | %-32s | %-32s\n", "生きているヒープ", "GC 回数", "1 ms タイマーのずれ (p50 / p99 / 最大)", "1 回の処理の遅延 (p50 / p99 / 最大)")
	for _, c := range []struct {
		kind string
		mb   int
		name string
	}{{"ptrfree", 400, "ポインタなし 400 MB"}, {"ptrrich", 400, "ポインタだらけ 400 MB"}} {
		v := run("B", c.kind, strconv.Itoa(c.mb))
		fmt.Printf("  %-26s %8.0f | %7.0f / %7.0f / %8.0f µs | %7.0f / %7.0f / %8.0f µs\n", c.name, v[0], v[1], v[2], v[3], v[4], v[5], v[6])
	}
}
