// ex3: おもちゃの GC を作って、アルゴリズムの違いを数える
//
//	A. マーク & スイープ と コピー (Cheney の半空間) の、仕事量と時間 (生きている割合を変えて)
//	B. 断片化: マーク & スイープの後の空きの散らばり と、コピーの後 (すべて詰まる)
//	C. 参照カウント: 循環したゴミが、永久に残る
//	D. 世代別 (弱い世代仮説): 大半のオブジェクトは、すぐ死ぬ。世代別なら、コピーの量が、どれだけ減るか (モデル)
//
// ヒープは、配列 (オブジェクト = 参照 2 つ) で表す。これは、アルゴリズムの仕事量を見るための、おもちゃで、
// 実際のランタイム (Go、JVM) の実装ではない。
// 実行: go run ./ex3_toy_collectors
package main

import (
	"fmt"
	"math/rand"
	"sort"
	"time"
)

type Obj struct {
	ref  [2]int32 // -1 = 参照なし
	mark bool
	_    [7]byte
}

// 生きているオブジェクトは、根から到達できる。ゴミは、到達できない (ゴミ同士は、互いに参照しあう)
func buildHeap(n int, liveFrac float64, rng *rand.Rand) ([]Obj, []int32) {
	h := make([]Obj, n)
	perm := rng.Perm(n)
	liveN := int(float64(n) * liveFrac)
	for i := range h {
		h[i].ref = [2]int32{-1, -1}
	}
	var roots []int32
	for k := 0; k < liveN; k++ {
		o := perm[k]
		if k < 16 {
			roots = append(roots, int32(o))
		} else {
			parent := perm[rng.Intn(k)] // 先にできた生きているオブジェクトから、参照される
			slot := rng.Intn(2)
			if h[parent].ref[slot] == -1 {
				h[parent].ref[slot] = int32(o)
			} else if h[parent].ref[1-slot] == -1 {
				h[parent].ref[1-slot] = int32(o)
			} else {
				p2 := perm[rng.Intn(k)]
				h[p2].ref[rng.Intn(2)] = int32(o)
				h[o].ref[0] = int32(perm[rng.Intn(k)]) // 古い生きているものへの、後ろ向きの参照 (循環を含む)
			}
		}
	}
	for k := liveN; k < n; k++ { // ゴミ: 互いに参照しあう (一部は循環)
		o := perm[k]
		if k+1 < n {
			h[o].ref[0] = int32(perm[k+1])
		}
		if rng.Intn(2) == 0 && k > liveN {
			h[o].ref[1] = int32(perm[liveN+rng.Intn(k-liveN)])
		}
	}
	return h, roots
}

func markSweep(h []Obj, roots []int32, live []bool) (marked int) {
	stack := append([]int32(nil), roots...)
	for len(stack) > 0 {
		o := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if o < 0 || h[o].mark {
			continue
		}
		h[o].mark = true
		marked++
		stack = append(stack, h[o].ref[0], h[o].ref[1])
	}
	for i := range h { // スイープ: ヒープ全体を走査
		if h[i].mark {
			if live != nil {
				live[i] = true
			}
			h[i].mark = false
		} else {
			h[i] = Obj{ref: [2]int32{-1, -1}}
		}
	}
	return
}

func copying(h []Obj, roots []int32) (copied int, to []Obj) {
	forward := make([]int32, len(h))
	for i := range forward {
		forward[i] = -1
	}
	to = make([]Obj, 0, len(h)/2)
	evac := func(o int32) int32 {
		if o < 0 {
			return -1
		}
		if forward[o] >= 0 {
			return forward[o]
		}
		to = append(to, h[o])
		forward[o] = int32(len(to) - 1)
		return forward[o]
	}
	for i := range roots {
		roots[i] = evac(roots[i])
	}
	for scan := 0; scan < len(to); scan++ { // Cheney: 到達先の空間を、先頭から順に走査
		to[scan].ref[0] = evac(to[scan].ref[0])
		to[scan].ref[1] = evac(to[scan].ref[1])
	}
	return len(to), to
}

func median(d []time.Duration) time.Duration {
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	return d[len(d)/2]
}

func partA() {
	const n = 2000000
	fmt.Printf("A. マーク & スイープ と コピー (ヒープ = %d 個のオブジェクト。時間は 5 回の中央値)\n", n)
	fmt.Printf("  %-12s | %-34s | %-34s\n", "生きている割合", "マーク & スイープ", "コピー (半空間)")
	fmt.Printf("  %-12s | %10s %10s %10s | %10s %10s %10s\n", "", "触った数", "(mark)", "時間", "コピーした数", "", "時間")
	for _, f := range []float64{0.05, 0.25, 0.50, 0.90} {
		var tms, tcp []time.Duration
		var marked, copied int
		for i := 0; i < 5; i++ {
			h, roots := buildHeap(n, f, rand.New(rand.NewSource(1)))
			t0 := time.Now()
			marked = markSweep(h, roots, nil)
			tms = append(tms, time.Since(t0))
			h2, roots2 := buildHeap(n, f, rand.New(rand.NewSource(1)))
			t0 = time.Now()
			copied, _ = copying(h2, roots2)
			tcp = append(tcp, time.Since(t0))
		}
		fmt.Printf("  割当 %3.0f%% (実際 %4.1f%%) | %10d %10d %7.1f ms | %10d %10s %7.1f ms\n", f*100, float64(marked)/n*100, marked+n, marked, float64(median(tms).Microseconds())/1000, copied, "", float64(median(tcp).Microseconds())/1000)
	}
	fmt.Println("  (マーク & スイープの「触った数」= マークした数 + スイープで走査した、ヒープ全体の数。コピーは、生きているものだけを触る。ただし、半分の空き領域 (2 倍のメモリ) が要る。)")
}

func partB() {
	const n = 1000000
	fmt.Println("\nB. 断片化: GC の後の、空き領域の散らばり (ヒープ 100 万個、生きている割合 (割り当て時) 50%、生きているものは、ヒープ全体に、ランダムに散らばっている)")
	h, roots := buildHeap(n, 0.5, rand.New(rand.NewSource(1)))
	freeRuns, largest, free, cur := 0, 0, 0, 0
	live := make([]bool, n)
	markSweep(h, roots, live)
	hh, roots2 := buildHeap(n, 0.5, rand.New(rand.NewSource(1)))
	for i := 0; i < n; i++ {
		if !live[i] {
			free++
			cur++
			if cur > largest {
				largest = cur
			}
		} else {
			if cur > 0 {
				freeRuns++
			}
			cur = 0
		}
	}
	if cur > 0 {
		freeRuns++
	}
	fmt.Printf("  マーク & スイープの後: 空きの合計 %d 個、空きの塊 %d 個、最大の連続した空き = %d 個 (空きの合計の %.4f%%)\n", free, freeRuns, largest, float64(largest)/float64(free)*100)
	fmt.Printf("  → 1,000 個の連続した領域が欲しくても、最大の空きは %d 個。空きの合計は足りるのに、確保できない (外部断片化。第2回のアロケータと同じ問題)\n", largest)
	_, to := copying(hh, roots2)
	fmt.Printf("  コピーの後: 生きているものは、先頭から詰まった %d 個。残りの %d 個が、1 つの連続した空き\n", len(to), n-len(to))
}

func partC() {
	fmt.Println("\nC. 参照カウント: 循環したゴミ (ゴミの塊: 5 個のオブジェクト。半分は木 (非循環)、半分は輪 (循環))")
	const clusters = 20000
	const size = 5
	type obj struct {
		ref [2]int32
		rc  int32
	}
	heap := make([]obj, clusters*size)
	for i := range heap {
		heap[i].ref = [2]int32{-1, -1}
	}
	roots := 0
	for c := 0; c < clusters; c++ {
		base := c * size
		heap[base].rc++ // 根 (これから外される) からの参照
		roots++
		if c%2 == 0 { // 木: base → base+1, base+2。base+1 → base+3, base+4
			heap[base].ref = [2]int32{int32(base + 1), int32(base + 2)}
			heap[base+1].ref = [2]int32{int32(base + 3), int32(base + 4)}
			for _, k := range []int{1, 2, 3, 4} {
				heap[base+k].rc++
			}
		} else { // 輪: base → base+1 → ... → base+4 → base
			for k := 0; k < size; k++ {
				nx := base + (k+1)%size
				heap[base+k].ref[0] = int32(nx)
				heap[nx].rc++
			}
		}
	}
	freed := 0
	var dec func(o int32)
	dec = func(o int32) {
		heap[o].rc--
		if heap[o].rc == 0 {
			freed++
			for _, r := range heap[o].ref {
				if r >= 0 {
					dec(r)
				}
			}
		}
	}
	for c := 0; c < clusters; c++ { // 根からの参照を、すべて外す (= プログラムが、これらを使い終わった)
		dec(int32(c * size))
	}
	total := clusters * size
	fmt.Printf("  すべてがゴミになった (%d 個)。参照カウントが 0 になって、解放できたのは %d 個。残った (漏れた) のは %d 個 (%.1f%%)\n", total, freed, total-freed, float64(total-freed)/float64(total)*100)
	fmt.Println("  → 木 (非循環) は、すべて解放されるが、輪 (循環) は、互いに参照しあうため、カウントが 0 にならず、永久に残る。")
	fmt.Println("    参照カウントを使う言語 (CPython、Swift、Rust の Rc) は、循環の検出 (別の GC) や、弱い参照を併用する。")
	fmt.Println("    また、ポインタを書き換えるたびに、カウントの更新 (+1 と −1) が要る (マルチスレッドでは、アトミック操作)。")
}

func partD() {
	const M = 2000000
	fmt.Printf("\nD. 世代別 GC のモデル: %d 回の割り当て。95%% は、すぐ死ぬ (平均 50 回の割り当ての間だけ生きる)。5%% は、ずっと生きる\n", M)
	rng := rand.New(rand.NewSource(1))
	death := make([]int, M)
	for i := range death {
		if rng.Float64() < 0.95 {
			death[i] = i + 1 + int(rng.ExpFloat64()*50)
		} else {
			death[i] = 1 << 60
		}
	}
	// 世代別でない (全体をコピーする GC): 前回の GC の後に、割り当てた量が、生き残った量に達したら、GC (ヒープ = 生きている量の 2 倍)
	{
		var alive []int
		copies, gcs, since, lastLive := 0, 0, 0, 5000
		for i := 0; i < M; i++ {
			alive = append(alive, i)
			since++
			if since >= lastLive {
				var next []int
				for _, o := range alive {
					if death[o] > i {
						next = append(next, o)
					}
				}
				alive, copies, gcs, since = next, copies+len(next), gcs+1, 0
				if lastLive = len(next); lastLive < 5000 {
					lastLive = 5000
				}
			}
		}
		fmt.Printf("  世代別でない (全体をコピー): GC %3d 回、コピーしたオブジェクト %8d 個 (割り当て 1 回あたり %.3f 個)\n", gcs, copies, float64(copies)/M)
	}
	// 世代別: 若い世代 (20,000 個) を頻繁に、老いた世代 (昇格したもの) は、2 倍に増えたときだけ、全体を集める
	for _, young := range []int{5000, 20000, 100000} {
		var youngSet, old []int
		copies, minor, major, oldLive := 0, 0, 0, 5000
		for i := 0; i < M; i++ {
			youngSet = append(youngSet, i)
			if len(youngSet) >= young {
				for _, o := range youngSet { // 若い世代の GC: 生き残ったものを、老いた世代へ昇格 (コピー)
					if death[o] > i {
						old = append(old, o)
						copies++
					}
				}
				youngSet = youngSet[:0]
				minor++
				if len(old) >= 2*oldLive { // 老いた世代の GC (全体)
					var next []int
					for _, o := range old {
						if death[o] > i {
							next = append(next, o)
						}
					}
					copies += len(next)
					old = next
					major++
					if oldLive = len(next); oldLive < 5000 {
						oldLive = 5000
					}
				}
			}
		}
		fmt.Printf("  世代別 (若い世代 %6d 個): 若い世代の GC %3d 回、老いた世代の GC %2d 回、コピーしたオブジェクト %7d 個 (割り当て 1 回あたり %.3f 個)\n", young, minor, major, copies, float64(copies)/M)
	}
	fmt.Println("  → 大半のオブジェクトが、若い世代の GC の前に死ぬので、コピーするのは、生き残った 5% だけ。")
	fmt.Println("    ただし、老いた世代から若い世代への参照を、追跡するための仕組み (書き込みバリア、カードテーブル) が要る。Go の GC は、世代別ではない (§本文)。")
}

func main() {
	partA()
	partB()
	partC()
	partD()
}
