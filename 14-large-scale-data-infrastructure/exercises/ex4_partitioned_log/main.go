// ex4: パーティションされたログ (Kafka 型) の、順序・配達の保証・並列度
//
// ログ = 追記だけの、順序づけられた列。トピックは、P 個のパーティション (それぞれが 1 つのログ)。キーのハッシュで、パーティションを決める。
// コンシューマは、オフセット (どこまで読んだか) を、コミットする。
//
//	A. 順序: パーティションの中では、順序が保たれる。パーティションをまたぐと、保たれない
//	B. 配達の保証: コンシューマが、クラッシュするとき、コミットの方式で、「失われる」か「重複する」かが決まる
//	C. 並列度と偏り: キーの分布が偏ると、特定のパーティションが、ボトルネックになる
//
// これは、単一のプロセス内の、模擬である。ブローカーの、複製、ディスク、ネットワークは、含まない。
// 実行: go run ./ex4_partitioned_log
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

type msg struct{ id, key, seq int } // seq: 全体の、生産の順

func partOf(key, P int) int {
	x := uint64(key)*0x9E3779B97F4A7C15 + 12345
	x ^= x >> 29
	return int(x % uint64(P))
}

func partA() {
	fmt.Println("A. 順序: 10 万個のメッセージ、1,000 個のキー。コンシューマは、パーティションを、順番に、20 個ずつ、読む")
	fmt.Printf("  %-14s %-34s %-34s\n", "パーティション数", "同じキーの、順序の逆転", "異なるキーの組 (ランダムに 100 万組) の、生産順と、消費順の逆転")
	rng := rand.New(rand.NewSource(1))
	const M = 100000
	for _, P := range []int{1, 4, 16} {
		logs := make([][]msg, P)
		for i := 0; i < M; i++ {
			k := rng.Intn(1000)
			p := partOf(k, P)
			logs[p] = append(logs[p], msg{i, k, i})
		}
		// 消費: パーティションを、巡回しながら、20 個ずつ
		pos := make([]int, P)
		var consumed []msg
		for done := false; !done; {
			done = true
			for p := 0; p < P; p++ {
				for c := 0; c < 20 && pos[p] < len(logs[p]); c++ {
					consumed = append(consumed, logs[p][pos[p]])
					pos[p]++
				}
				if pos[p] < len(logs[p]) {
					done = false
				}
			}
		}
		rank := make([]int, M)
		for i, m := range consumed {
			rank[m.id] = i
		}
		// 同じキーの順序
		last := map[int]int{}
		sameKeyInv := 0
		for _, m := range consumed {
			if l, ok := last[m.key]; ok && l > m.seq {
				sameKeyInv++
			}
			last[m.key] = m.seq
		}
		// 異なるキー
		keyOf := make([]int, M)
		for _, m := range consumed {
			keyOf[m.id] = m.key
		}
		inv, tot := 0, 0
		for t := 0; t < 1000000; t++ {
			a, b := rng.Intn(M), rng.Intn(M)
			if a == b || keyOf[a] == keyOf[b] {
				continue
			}
			if a > b {
				a, b = b, a
			}
			tot++
			if rank[a] > rank[b] {
				inv++
			}
		}
		fmt.Printf("  %-14d %-34s %-34s\n", P, fmt.Sprintf("%d", sameKeyInv), fmt.Sprintf("%.1f%% (%d / %d)", float64(inv)/float64(tot)*100, inv, tot))
	}
	fmt.Println("  → 同じキーは、同じパーティションに行くので、順序が保たれる。異なるキーの順序は、パーティションが多いほど、入れ替わる (全体の順序は、保証されない)。")
}

func partB() {
	fmt.Println("\nB. 配達の保証: 100 万個のメッセージ、1 パーティション。コンシューマは、各メッセージの処理で、確率 p でクラッシュし、最後にコミットしたオフセットから、再開する")
	const M = 1000000
	fmt.Printf("  %-52s %-10s %-12s %-12s\n", "コミットの方式", "p", "失われた", "重複して処理")
	for _, p := range []float64{0.0001, 0.001} {
		for _, v := range []struct {
			name   string
			before bool // 処理の前に、コミット
			batch  int  // バッチごとにコミット
			dedupe bool
		}{
			{"処理の前に、1 件ずつコミット (at-most-once)", true, 1, false},
			{"処理の後に、1 件ずつコミット (at-least-once)", false, 1, false},
			{"処理の後に、100 件ごとにコミット", false, 100, false},
			{"処理の後に、100 件ごとにコミット + 冪等な処理 (id で、重複を無視)", false, 100, true},
		} {
			rng := rand.New(rand.NewSource(1))
			committed := 0            // 次に読むオフセット (コミット済み)
			applied := make([]int, M) // 効果が、実際に反映された回数 (冪等な処理では、重複を無視)
			effect := func(i int) {
				if v.dedupe {
					applied[i] = 1
				} else {
					applied[i]++
				}
			}
			pos, sinceCommit := 0, 0
			for pos < M {
				crash := rng.Float64() < p
				if v.before {
					// 先にコミットしてから、処理する。クラッシュは、処理の前 (コミットの後) に起きる → このメッセージは、再読み込みされず、失われる
					committed = pos + 1
					if crash {
						pos = committed
						continue
					}
					effect(pos)
					pos++
					continue
				}
				// 処理 (効果を反映) してから、コミットする。クラッシュは、効果の反映の後、コミットの前 → 最後のコミットから、再読み込み (重複)
				effect(pos)
				if crash {
					pos = committed
					sinceCommit = 0
					continue
				}
				pos++
				sinceCommit++
				if sinceCommit >= v.batch {
					committed = pos
					sinceCommit = 0
				}
			}
			lost, dup := 0, 0
			for i := 0; i < M; i++ {
				if applied[i] == 0 {
					lost++
				}
				if applied[i] > 1 {
					dup += applied[i] - 1
				}
			}
			fmt.Printf("  %-52s %-10g %-12d %-12d\n", v.name, p, lost, dup)
		}
	}
	fmt.Println("  → コミットを先にすると、クラッシュで、メッセージが失われる。後にすると、失われないが、重複する (バッチが大きいほど、多い)。")
	fmt.Println("    『ちょうど 1 回』は、メッセージの配達だけでは、作れない。処理を冪等にする (id で、重複を無視する)、または、効果とオフセットを、1 つのトランザクションで、反映する。")
	fmt.Println("    模擬の仮定: クラッシュは、「先にコミット」では、コミットの直後 (処理の前)、「後にコミット」では、効果の反映の直後 (コミットの前) に起きる、最悪の位置とした。")
}

func partC() {
	fmt.Println("\nC. 並列度と偏り: 到着 3 メッセージ/ms、コンシューマ 1 つが 1 メッセージ/ms で処理。コンシューマは、パーティション単位で割り当てる (1 つのパーティションは、1 つのコンシューマだけ)")
	fmt.Printf("  %-8s %-8s %-10s | %-26s %-22s %s\n", "P", "C", "キーの偏り", "最も忙しいパーティションの到着", "最も忙しいコンシューマの負荷", "安定か (負荷 < 1 か)")
	for _, zipf := range []float64{0, 1.2} {
		for _, pc := range [][2]int{{4, 4}, {16, 4}, {16, 16}, {64, 16}, {4, 16}} {
			P, C := pc[0], pc[1]
			rng := rand.New(rand.NewSource(1))
			// キーの確率: zipf = 0 なら一様、それ以外は、Zipf
			const K = 10000
			w := make([]float64, K)
			sum := 0.0
			for i := range w {
				if zipf == 0 {
					w[i] = 1
				} else {
					w[i] = 1 / math.Pow(float64(i+1), zipf)
				}
				sum += w[i]
			}
			share := make([]float64, P)
			for i := range w {
				share[partOf(i, P)] += w[i] / sum
			}
			// パーティションを、コンシューマに、割り当てる (ラウンドロビン。順番は、ランダム)
			order := rng.Perm(P)
			load := make([]float64, C)
			for i, p := range order {
				load[i%C] += share[p] * 3
			}
			maxShare := 0.0
			for _, s := range share {
				if s > maxShare {
					maxShare = s
				}
			}
			maxLoad := 0.0
			for _, l := range load {
				if l > maxLoad {
					maxLoad = l
				}
			}
			sorted := append([]float64(nil), load...)
			sort.Float64s(sorted)
			label := "一様"
			if zipf > 0 {
				label = fmt.Sprintf("Zipf %.1f", zipf)
			}
			ok := "安定"
			if maxLoad >= 1 {
				ok = "不安定 (遅れが、増え続ける)"
			}
			fmt.Printf("  %-8d %-8d %-10s | %-26s %-22.2f %s\n", P, C, label, fmt.Sprintf("%.1f%% (%.2f /ms)", maxShare*100, maxShare*3), maxLoad, ok)
		}
	}
	fmt.Println("  → 並列度の上限は、パーティション数 (C > P のコンシューマは、遊ぶ)。キーが偏ると、1 つのパーティションが、1 つのコンシューマの能力を超えることがあり、コンシューマを増やしても、解決しない。")
	fmt.Println("    パーティション数は、後から増やすと、キーとパーティションの対応が変わる (順序の保証が、一時的に壊れる) ので、最初に、余裕を持って決める。")
}

func main() {
	partA()
	partB()
	partC()
}
