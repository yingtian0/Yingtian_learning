// ex1: GC のコストを、GOGC (メモリと CPU のトレードオフ) と、生きているヒープの大きさで測る
//
// 子プロセスが、(1) ポインタを含む、生きているヒープ (64 バイトのノードの連結) を作り、
// (2) 2 秒間、4 KB の「すぐ捨てるデータ」を、割り当て続ける。GOMAXPROCS = 2 に絞り、GC が、アプリと CPU を、奪い合うようにする。
// 親プロセスは、GOGC と、生きているヒープの大きさを変えて、子を起動し、次を表にする。
//
//	GC の回数 / 秒、GC が使った CPU の割合、ヒープの最大値 (と、理論値 = 生きている量 × (1 + GOGC/100))、割り当ての速度、最大の停止時間
//
// 実行: go run ./ex1_gc_basics
package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/metrics"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type node struct {
	next *node
	pad  [56]byte
}

var live []*node
var sink []byte

func readFloat(name string) float64 {
	s := []metrics.Sample{{Name: name}}
	metrics.Read(s)
	switch s[0].Value.Kind() {
	case metrics.KindUint64:
		return float64(s[0].Value.Uint64())
	case metrics.KindFloat64:
		return s[0].Value.Float64()
	}
	return 0
}

func child(liveMB int, dur time.Duration) {
	n := liveMB << 20 / 64
	live = make([]*node, 0, n)
	var prev *node
	for i := 0; i < n; i++ {
		nd := &node{next: prev}
		live = append(live, nd)
		prev = nd
	}
	runtime.GC()

	var peak atomic.Uint64
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			if v := uint64(readFloat("/memory/classes/heap/objects:bytes")); v > peak.Load() {
				peak.Store(v)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()

	g0, gcCPU0, totCPU0 := readFloat("/gc/cycles/total:gc-cycles"), readFloat("/cpu/classes/gc/total:cpu-seconds"), readFloat("/cpu/classes/total:cpu-seconds")
	t0 := time.Now()
	var allocated int64
	for time.Since(t0) < dur {
		for i := 0; i < 1000; i++ {
			b := make([]byte, 4096)
			b[0] = 1
			sink = b
			allocated += 4096
		}
	}
	el := time.Since(t0).Seconds()
	close(stop)
	g1, gcCPU1, totCPU1 := readFloat("/gc/cycles/total:gc-cycles"), readFloat("/cpu/classes/gc/total:cpu-seconds"), readFloat("/cpu/classes/total:cpu-seconds")
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	var maxPause uint64
	for _, p := range ms.PauseNs {
		if p > maxPause {
			maxPause = p
		}
	}
	fmt.Printf("RESULT %.2f %.4f %.1f %.3f %d\n", (g1-g0)/el, (gcCPU1-gcCPU0)/(totCPU1-totCPU0), float64(peak.Load())/1048576, float64(allocated)/el/1e9, maxPause/1000)
	runtime.KeepAlive(live)
}

func run(liveMB, gogc int) []float64 {
	cmd := exec.Command(os.Args[0], "child", strconv.Itoa(liveMB))
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOGC=%d", gogc), "GOMAXPROCS=2")
	out, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "RESULT ") {
			var vals []float64
			for _, f := range strings.Fields(l)[1:] {
				v, _ := strconv.ParseFloat(f, 64)
				vals = append(vals, v)
			}
			return vals
		}
	}
	panic("結果が得られなかった: " + string(out))
}

func main() {
	if len(os.Args) > 2 && os.Args[1] == "child" {
		mb, _ := strconv.Atoi(os.Args[2])
		child(mb, 2*time.Second)
		return
	}
	fmt.Println("GC のコスト (子プロセス、GOMAXPROCS = 2、各 2 秒。生きているヒープは、ポインタを含む 64 バイトのノード)")
	fmt.Printf("  %-26s %10s %12s %14s %14s %12s %10s\n", "条件", "GC 回数/秒", "GC の CPU 割合", "ヒープの最大", "理論値", "割り当て GB/秒", "最大停止")
	row := func(liveMB, gogc int) {
		v := run(liveMB, gogc)
		fmt.Printf("  生きている %4d MB、GOGC=%-4d %10.1f %11.1f%% %11.0f MB %11.0f MB %12.2f %7d µs\n", liveMB, gogc, v[0], v[1]*100, v[2], float64(liveMB)*(1+float64(gogc)/100), v[3], int(v[4]))
	}
	fmt.Println("\n  A. 生きているヒープ = 100 MB で、GOGC を変える")
	for _, g := range []int{25, 50, 100, 200, 400} {
		row(100, g)
	}
	fmt.Println("\n  B. GOGC = 100 で、生きているヒープを変える")
	for _, mb := range []int{25, 100, 400} {
		row(mb, 100)
	}
}
