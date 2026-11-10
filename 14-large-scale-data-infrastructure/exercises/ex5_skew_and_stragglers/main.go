// ex5: 大規模処理の 2 つの敵 — 偏り (skew) と、遅いタスク (straggler)
//
//	A. パーティションの偏り: ハッシュ分割と、範囲分割で、キーが「連番 (時刻)」や「偏った分布 (Zipf)」のとき、負荷が、どれだけ偏るか。ホットキーを、分割する (ソルト)
//	B. MapReduce 型のジョブ: 1,000 個のタスクを、100 台で処理。一部のマシンが遅い。完了までの時間 (makespan) と、投機的実行 (遅いタスクの、予備を起動) の効果と、代償
//
// すべてモデルである。B の「遅いマシン」の分布は、仮定であり、実際のクラスタの測定ではない。
// 実行: go run ./ex5_skew_and_stragglers
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

func partA() {
	const P = 16
	const M = 1000000
	fmt.Printf("A. %d 個の書き込みを、%d 個のパーティションに分ける。負荷の偏り = 最も忙しいパーティションの件数 ÷ 平均\n", M, P)
	fmt.Printf("  %-52s %-12s %s\n", "キーの分布と、分割の方法", "偏り (max/平均)", "最も忙しい 1 つの割合")
	rng := rand.New(rand.NewSource(1))
	hashP := func(k uint64) int {
		k ^= k >> 33
		k *= 0xff51afd7ed558ccd
		k ^= k >> 33
		return int(k % P)
	}
	report := func(name string, cnt []int) {
		mx, sum := 0, 0
		for _, c := range cnt {
			sum += c
			if c > mx {
				mx = c
			}
		}
		fmt.Printf("  %-52s %-12.2f %.1f%%\n", name, float64(mx)/(float64(sum)/P), float64(mx)/float64(sum)*100)
	}
	// 1. 連番 (時刻順) のキー
	{
		cnt := make([]int, P)
		for i := 0; i < M; i++ {
			cnt[hashP(uint64(i))]++
		}
		report("連番のキー (時刻順): ハッシュ分割", cnt)
		cnt = make([]int, P)
		const window = 10000
		for i := 0; i < M; i++ {
			_ = i % window
			cnt[P-1]++ // 常に、最後の範囲 (現在の時刻の範囲) に書く
		}
		report("連番のキー (時刻順): 範囲分割 (現在の範囲に集中)", cnt)
	}
	// 2. Zipf のキー
	for _, s := range []float64{0.8, 1.2} {
		const K = 100000
		w := make([]float64, K)
		sum := 0.0
		for i := range w {
			w[i] = 1 / math.Pow(float64(i+1), s)
			sum += w[i]
		}
		cdf := make([]float64, K)
		acc := 0.0
		for i := range w {
			acc += w[i] / sum
			cdf[i] = acc
		}
		draw := func() int { return sort.SearchFloat64s(cdf, rng.Float64()) }
		cnt := make([]int, P)
		cntSalt := make([]int, P)
		cntRange := make([]int, P)
		for i := 0; i < M; i++ {
			k := draw()
			if k >= K {
				k = K - 1
			}
			cnt[hashP(uint64(k))]++
			cntRange[k*P/K]++ // 範囲分割 (キーの順序で)
			// ソルト: 最も人気の 100 個のキーだけ、キーに 0〜15 の乱数を付けて、16 個に分ける (読みは、16 か所を、集める)
			if k < 100 {
				cntSalt[hashP(uint64(k)*131+uint64(rng.Intn(16)))]++
			} else {
				cntSalt[hashP(uint64(k))]++
			}
		}
		report(fmt.Sprintf("Zipf(%.1f) のキー: ハッシュ分割", s), cnt)
		report(fmt.Sprintf("Zipf(%.1f) のキー: 範囲分割 (人気のキーが、前の範囲に集まる)", s), cntRange)
		report(fmt.Sprintf("Zipf(%.1f) のキー: ハッシュ分割 + 上位 100 キーを、ソルトで 16 分割", s), cntSalt)
	}
	fmt.Println("  → ハッシュ分割は、連番のキーの偏りを、解消するが、(1) 範囲検索ができなくなる、(2) 1 つのキーが、人気なら、そのキーを持つパーティションが、ボトルネックのまま。")
	fmt.Println("    範囲分割は、範囲検索に向くが、時刻順のキーや、人気のキーの、偏りに弱い。ホットキーは、ソルトで分割できるが、読みが、複数のパーティションの集約になる (代償)。")
}

type task struct {
	base      float64
	done      bool
	start     float64
	copies    int
	runningOn []int
}

func simulate(W, M int, slowFrac, slowFactor float64, spec bool, rng *rand.Rand) (makespan, work float64) {
	speed := make([]float64, W)
	for i := range speed {
		speed[i] = 1
		if rng.Float64() < slowFrac {
			speed[i] = 1 / slowFactor
		}
	}
	tasks := make([]*task, M)
	bases := make([]float64, M)
	for i := range tasks {
		tasks[i] = &task{base: math.Exp(rng.NormFloat64() * 0.3)}
		bases[i] = tasks[i].base
	}
	sort.Float64s(bases)
	median := bases[M/2]
	type worker struct {
		task   int
		finish float64
		busy   bool
		began  float64
	}
	ws := make([]worker, W)
	next := 0
	now := 0.0
	completed := 0
	dispatch := func() {
		for w := range ws {
			if ws[w].busy {
				continue
			}
			if next < M {
				t := tasks[next]
				t.start, t.copies = now, 1
				t.runningOn = append(t.runningOn, w)
				ws[w] = worker{next, now + t.base/speed[w], true, now}
				next++
			} else if spec {
				// 実行中で、まだ予備のない、最も長く走っているタスクに、予備を起動 (経過時間が、基準の中央値の 1.3 倍を超えたもの)
				best, bestEl := -1, 0.0
				for i := 0; i < next; i++ {
					t := tasks[i]
					if !t.done && t.copies == 1 && now-t.start > 1.3*median && now-t.start > bestEl {
						best, bestEl = i, now-t.start
					}
				}
				if best >= 0 {
					t := tasks[best]
					t.copies++
					t.runningOn = append(t.runningOn, w)
					ws[w] = worker{best, now + t.base/speed[w], true, now}
				}
			}
		}
	}
	dispatch()
	for completed < M {
		// 次に終わるワーカー
		bw, bf := -1, math.Inf(1)
		for w := range ws {
			if ws[w].busy && ws[w].finish < bf {
				bw, bf = w, ws[w].finish
			}
		}
		if bw < 0 {
			break
		}
		now = bf
		t := tasks[ws[bw].task]
		work += (now - ws[bw].began) * speed[bw] // 進んだ仕事の量 (遅いワーカーは、同じ時間で、少ししか進まない)
		ws[bw].busy = false
		if !t.done {
			t.done = true
			completed++
			for _, o := range t.runningOn { // 他のコピーは、打ち切る
				if o != bw && ws[o].busy && ws[o].task == ws[bw].task {
					work += (now - ws[o].began) * speed[o]
					ws[o].busy = false
				}
			}
		}
		dispatch()
	}
	return now, work
}

func partB() {
	const W, M = 100, 1000
	fmt.Printf("\nB. %d 個のタスク (処理時間は、対数正規。中央値 約 1)、%d 台のワーカー。一部のワーカーは、遅い。各条件 200 回の平均\n", M, W)
	fmt.Printf("  %-34s %-26s | %-26s | %s\n", "遅いワーカーの割合・遅さ", "投機的実行なし", "投機的実行あり", "追加の仕事量 (進んだ仕事の量。重複と打ち切りを含む)")
	fmt.Printf("  %-34s %-26s | %-26s |\n", "", "完了時間 平均 / p99", "完了時間 平均 / p99")
	for _, c := range []struct{ frac, factor float64 }{{0, 1}, {0.02, 10}, {0.05, 5}, {0.05, 10}, {0.2, 3}} {
		var a, b []float64
		var wa, wb float64
		for r := 0; r < 200; r++ {
			m1, w1 := simulate(W, M, c.frac, c.factor, false, rand.New(rand.NewSource(int64(r))))
			m2, w2 := simulate(W, M, c.frac, c.factor, true, rand.New(rand.NewSource(int64(r))))
			a, b = append(a, m1), append(b, m2)
			wa += w1
			wb += w2
		}
		mean := func(x []float64) float64 {
			s := 0.0
			for _, v := range x {
				s += v
			}
			return s / float64(len(x))
		}
		sort.Float64s(a)
		sort.Float64s(b)
		fmt.Printf("  %-34s %-26s | %-26s | +%.1f%%\n", func() string {
			if c.frac == 0 {
				return "遅いワーカーなし"
			}
			return fmt.Sprintf("%.0f%% のワーカーが、%.0f 倍遅い", c.frac*100, c.factor)
		}(),
			fmt.Sprintf("%.1f / %.1f", mean(a), a[198]), fmt.Sprintf("%.1f / %.1f", mean(b), b[198]), (wb/wa-1)*100)
	}
	fmt.Printf("  (ワーカーが遅い割合 0 のとき、理想の完了時間 ≈ タスクの総量 ÷ ワーカー数 ≈ %.1f)\n", 1000.0*1.05/100)
	fmt.Println("  → 遅いワーカーに当たったタスクが、ジョブ全体の完了を、引き延ばす (最後の 1 つを待つ)。投機的実行は、遅いタスクの予備を、空いたワーカーで走らせ、速い方を採る。")
	fmt.Println("    代償は、追加の仕事量 (重複した計算)。遅いのが、マシンでなく、データの偏り (そのタスクだけ重い) なら、予備も遅く、効かない (本文 §7)。")
}

func main() {
	partA()
	partB()
}
