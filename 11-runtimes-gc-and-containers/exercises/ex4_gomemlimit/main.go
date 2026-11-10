// ex4: GOMEMLIMIT (メモリの上限) を、実際の Go ランタイムで試す
//
// 子プロセスが、ポインタを含む、100 MB の生きているヒープを持ち、2 秒間、4 KB の「すぐ捨てるデータ」を割り当て続ける (ex1 と同じ)。
// GOGC と GOMEMLIMIT を変えて、(1) ヒープの最大、(2) GC の CPU 割合、(3) 割り当ての速度 (= アプリが進んだ量) を比べる。
//
// 見るもの:
//
//	GOGC=100 (既定)          : ヒープは、生きている量の約 2 倍まで膨らむ。上限は、ない。
//	GOGC=off + GOMEMLIMIT    : 上限に近づくまで GC をしない。上限に近づくと、GC が増える。
//	上限が、生きている量に近い : GC が、ほぼ連続して走る (GC の CPU 割合が、跳ね上がる)。ランタイムは、GC の CPU を約 50% に抑える (CPU リミッタ)
//	上限が、生きている量より小さい: 守れない。上限は「ソフト」な目標で、OOM を防ぐ保証ではない
//
// 実行: go run ./ex4_gomemlimit
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

	var peak, peakTotal atomic.Uint64
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
			// ランタイムが、OS から得ていて、返していない量 (RSS の近似)
			t := uint64(readFloat("/memory/classes/total:bytes") - readFloat("/memory/classes/heap/released:bytes"))
			if t > peakTotal.Load() {
				peakTotal.Store(t)
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
	fmt.Printf("RESULT %.2f %.4f %.1f %.1f %.3f\n", (g1-g0)/el, (gcCPU1-gcCPU0)/(totCPU1-totCPU0), float64(peak.Load())/1048576, float64(peakTotal.Load())/1048576, float64(allocated)/el/1e9)
	runtime.KeepAlive(live)
}

func run(liveMB int, gogc, limit string) []float64 {
	cmd := exec.Command(os.Args[0], "child", strconv.Itoa(liveMB))
	cmd.Env = append(os.Environ(), "GOGC="+gogc, "GOMEMLIMIT="+limit, "GOMAXPROCS=2")
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
	fmt.Println("GOMEMLIMIT の効果 (子プロセス、GOMAXPROCS = 2、各 2 秒。生きているヒープ = 100 MB (ポインタ入り))")
	fmt.Printf("  %-34s %10s %12s %13s %15s %13s\n", "設定", "GC 回数/秒", "GC の CPU 割合", "ヒープの最大", "OS から得た最大", "割り当て GB/秒")
	for _, c := range []struct{ gogc, limit, label string }{
		{"100", "off", "GOGC=100 (既定)"},
		{"off", "400MiB", "GOGC=off, 上限 400 MiB"},
		{"off", "250MiB", "GOGC=off, 上限 250 MiB"},
		{"off", "150MiB", "GOGC=off, 上限 150 MiB"},
		{"off", "110MiB", "GOGC=off, 上限 110 MiB (生きている量に近い)"},
		{"off", "80MiB", "GOGC=off, 上限 80 MiB (生きている量より小さい)"},
		{"100", "150MiB", "GOGC=100 + 上限 150 MiB"},
	} {
		v := run(100, c.gogc, c.limit)
		fmt.Printf("  %-34s %10.1f %11.1f%% %10.0f MB %12.0f MB %13.2f\n", c.label, v[0], v[1]*100, v[2], v[3], v[4])
	}
}
