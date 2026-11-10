// ex1: 論理時計 (Lamport 時計とベクタークロック)
//
// N 個のプロセスが、メッセージを送りあう。各イベントに、(a) Lamport 時計、(b) ベクタークロック を付ける。
// 「正解」は、メッセージのつながりから、独立に計算した、因果関係 (happened-before) の推移閉包。
// 確かめること:
//  1. a → b ならば L(a) < L(b)  (Lamport 時計の性質。常に成り立つはず)
//  2. L(a) < L(b) でも a → b とは限らない (逆は成り立たない)。並行なイベントにも、順序が付いてしまう
//  3. ベクタークロックは、a → b と 並行 を、正確に区別できる (正解と、完全に一致するはず)
//  4. ベクタークロックの大きさは、プロセス数に比例する
//
// 実行: go run ./ex1_logical_clocks
package main

import (
	"fmt"
	"math/rand"
)

type event struct {
	proc    int
	lamport int
	vc      []int
	prev    int // 同じプロセスの、直前のイベント (なければ -1)
	sendEv  int // 受信イベントなら、対応する送信イベント (なければ -1)
}

type msg struct {
	from, sendEv int
	lamport      int
	vc           []int
}

func simulate(n, steps int, seed int64) []event {
	rng := rand.New(rand.NewSource(seed))
	lam := make([]int, n)
	vcs := make([][]int, n)
	last := make([]int, n)
	for i := range vcs {
		vcs[i] = make([]int, n)
		last[i] = -1
	}
	inbox := make([][]msg, n)
	var evs []event
	add := func(p, sendEv int) int {
		lam[p]++
		vcs[p][p]++
		e := event{proc: p, lamport: lam[p], vc: append([]int(nil), vcs[p]...), prev: last[p], sendEv: sendEv}
		evs = append(evs, e)
		last[p] = len(evs) - 1
		return last[p]
	}
	for s := 0; s < steps; s++ {
		p := rng.Intn(n)
		switch r := rng.Float64(); {
		case r < 0.4 && len(inbox[p]) > 0: // 受信 (メッセージは、順不同で届く)
			k := rng.Intn(len(inbox[p]))
			m := inbox[p][k]
			inbox[p] = append(inbox[p][:k], inbox[p][k+1:]...)
			if m.lamport > lam[p] {
				lam[p] = m.lamport
			}
			for i := range vcs[p] {
				if m.vc[i] > vcs[p][i] {
					vcs[p][i] = m.vc[i]
				}
			}
			add(p, m.sendEv)
		case r < 0.7: // 送信
			q := rng.Intn(n - 1)
			if q >= p {
				q++
			}
			id := add(p, -1)
			inbox[q] = append(inbox[q], msg{p, id, lam[p], append([]int(nil), vcs[p]...)})
		default: // 内部イベント
			add(p, -1)
		}
	}
	return evs
}

func main() {
	const n, steps = 5, 700
	evs := simulate(n, steps, 1)
	m := len(evs)
	// 正解: 因果関係の推移閉包 (ビット集合)。reach[b] = b より前のイベントの集合
	words := (m + 63) / 64
	before := make([][]uint64, m)
	for b := 0; b < m; b++ {
		before[b] = make([]uint64, words)
		for _, a := range []int{evs[b].prev, evs[b].sendEv} {
			if a >= 0 {
				before[b][a/64] |= 1 << (a % 64)
				for w := range before[b] {
					before[b][w] |= before[a][w]
				}
			}
		}
	}
	hb := func(a, b int) bool { return before[b][a/64]>>(a%64)&1 == 1 }
	less := func(x, y []int) bool { // x < y (すべて <=、少なくとも 1 つ <)
		strict := false
		for i := range x {
			if x[i] > y[i] {
				return false
			}
			if x[i] < y[i] {
				strict = true
			}
		}
		return strict
	}
	var pairs, causal, concurrent int
	var lamportViolations, lamportOrderedNotCausal, lamportOrderedConcurrentTotal, vcMismatch int
	for a := 0; a < m; a++ {
		for b := a + 1; b < m; b++ { // a < b (生成順)。b → a はありえない
			pairs++
			c := hb(a, b)
			if c {
				causal++
				if !(evs[a].lamport < evs[b].lamport) {
					lamportViolations++
				}
			} else {
				concurrent++
			}
			if evs[a].lamport < evs[b].lamport {
				lamportOrderedConcurrentTotal++
				if !c {
					lamportOrderedNotCausal++
				}
			}
			if less(evs[a].vc, evs[b].vc) != c || less(evs[b].vc, evs[a].vc) {
				vcMismatch++
			}
		}
	}
	fmt.Printf("%d プロセス、%d 個のイベント、%d 対 (生成順の組)\n", n, m, pairs)
	fmt.Printf("  正解: 因果関係あり (a → b) %d 対 (%.1f%%)、並行 %d 対 (%.1f%%)\n", causal, pct(causal, pairs), concurrent, pct(concurrent, pairs))
	fmt.Printf("\n1. a → b なのに L(a) >= L(b) となった対: %d (0 のはず。Lamport 時計は因果関係と矛盾しない)\n", lamportViolations)
	fmt.Printf("2. L(a) < L(b) の対 %d 個のうち、実際には因果関係がない (並行) もの: %d (%.1f%%)\n", lamportOrderedConcurrentTotal, lamportOrderedNotCausal, pct(lamportOrderedNotCausal, lamportOrderedConcurrentTotal))
	fmt.Printf("   → Lamport 時計だけを見て「a が先」と言っても、%.1f%% は、因果関係とは無関係な順序\n", pct(lamportOrderedNotCausal, lamportOrderedConcurrentTotal))
	fmt.Printf("3. ベクタークロックの判定が、正解と食い違った対: %d (0 のはず)\n", vcMismatch)
	fmt.Println("\n4. ベクタークロックの大きさ (整数 = 8 バイトとして、1 イベントあたり)")
	for _, k := range []int{5, 100, 1000, 100000} {
		fmt.Printf("   プロセス数 %6d: %9d バイト  (Lamport 時計は常に 8 バイト)\n", k, 8*k)
	}
}

func pct(a, b int) float64 { return float64(a) / float64(b) * 100 }
