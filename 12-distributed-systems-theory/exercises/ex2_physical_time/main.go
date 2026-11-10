// ex2: 物理時計のずれと、それを扱う 3 つの方法 (すべてシミュレーション)
//
//	A. 最後の書き込みが勝つ (LWW) を、各ノードの物理時計の時刻で決めると、本当は後の書き込みが、負けることがある
//	B. HLC (ハイブリッド論理時計): 物理時計 + 論理カウンタ。因果的な順序は、時計がずれても守られる
//	C. TrueTime 風: 時刻を「区間 [earliest, latest]」で扱い、コミット待ち (commit wait) で、外部整合性を守る
//
// 時計のずれ: 各ノードの時計は、真の時刻に対して、一定のずれ (オフセット) を持つ。オフセットは、正規分布 N(0, σ)。
// これは、挙動を理解するためのモデルで、実際の NTP や GPS/原子時計の、誤差の特性ではない。
// 実行: go run ./ex2_physical_time
package main

import (
	"fmt"
	"math/rand"
)

func partA() {
	fmt.Println("A. LWW (物理時計の時刻で決める): 真の時刻で、Δ ms 後に、別のノードが、同じキーに書いた。後の書き込みが、負ける確率")
	fmt.Printf("  %-12s", "Δ \\ σ")
	sigmas := []float64{0.1, 1, 10, 50}
	for _, s := range sigmas {
		fmt.Printf(" %8.1fms", s)
	}
	fmt.Println("   (σ = ノード間の時計のオフセットの標準偏差)")
	rng := rand.New(rand.NewSource(1))
	for _, d := range []float64{0.5, 2, 10, 50, 200} {
		fmt.Printf("  %-9.1f ms", d)
		for _, s := range sigmas {
			lost, N := 0, 200000
			for i := 0; i < N; i++ {
				o1, o2 := rng.NormFloat64()*s, rng.NormFloat64()*s // 2 ノードのオフセット
				t1 := 0 + o1
				t2 := d + o2
				if t2 < t1 {
					lost++
				}
			}
			fmt.Printf(" %9.2f%%", float64(lost)/float64(N)*100)
		}
		fmt.Println()
	}
	fmt.Println("  → 書き込みの間隔が、時計のずれと同程度かそれ以下だと、数 % 〜 数十 % で、順序が逆転する。逆転した書き込みは、静かに失われる。")
}

// HLC: (l, c)。l = max(物理時刻, 見た中で最大の l)、同じ l の間は c を増やす
type hlc struct {
	l int64
	c int
}

func (h *hlc) send(pt int64) hlc {
	if pt > h.l {
		h.l, h.c = pt, 0
	} else {
		h.c++
	}
	return *h
}

func (h *hlc) recv(m hlc, pt int64) hlc {
	switch {
	case pt > h.l && pt > m.l:
		h.l, h.c = pt, 0
	case m.l > h.l:
		h.l, h.c = m.l, m.c+1
	case h.l > m.l:
		h.c++
	default:
		if m.c > h.c {
			h.c = m.c
		}
		h.c++
	}
	return *h
}

func (a hlc) less(b hlc) bool { return a.l < b.l || (a.l == b.l && a.c < b.c) }

func partB() {
	fmt.Println("\nB. 因果的に順序づけられた書き込み (ノード 1 が書く → その結果を、ノード 2 が読んで、次の書き込みをする)。2 つ目のタイムスタンプが、1 つ目より小さくなる割合")
	fmt.Printf("  %-8s %-18s %-18s\n", "σ (ms)", "物理時計だけ (LWW)", "HLC")
	rng := rand.New(rand.NewSource(1))
	for _, s := range []float64{1, 10, 50} {
		badPhys, badHLC, N := 0, 0, 200000
		for i := 0; i < N; i++ {
			o1, o2 := rng.NormFloat64()*s, rng.NormFloat64()*s
			trueT1 := 100.0
			trueT2 := trueT1 + 1 + rng.Float64() // 因果的に後 (1〜2 ms 後。読んで、書く)
			p1, p2 := int64((trueT1+o1)*1000), int64((trueT2+o2)*1000)
			if p2 <= p1 {
				badPhys++
			}
			var n1, n2 hlc
			ts1 := n1.send(p1) // ノード 1 の書き込み。タイムスタンプが、読んだ側に渡る
			n2.recv(ts1, p2)
			ts2 := n2.send(p2)
			if !ts1.less(ts2) {
				badHLC++
			}
		}
		fmt.Printf("  %-8.0f %-18s %-18s\n", s, fmt.Sprintf("%.2f%%", float64(badPhys)/float64(N)*100), fmt.Sprintf("%.2f%%", float64(badHLC)/float64(N)*100))
	}
	fmt.Println("  → HLC は、メッセージで受け取ったタイムスタンプより大きい値を作るので、時計がずれても、因果的な順序は守られる (0%)。")
	fmt.Println("    ただし、HLC は、並行な書き込みの『本当の時刻の順序』を、保証するものではない (ex1 の、Lamport 時計の性質と同じ)。")
}

func partC() {
	fmt.Println("\nC. TrueTime 風: 外部整合性 (T1 が、コミットを完了してから、T2 が始まったなら、T2 のタイムスタンプ > T1 のタイムスタンプ)")
	fmt.Println("   モデル: 各ノードは、自分の時計 (真の時刻 + オフセット) と、不確かさ ε を持つ。区間は [now−ε, now+ε]。真の時刻は、必ず、区間の中。")
	fmt.Println("   (オフセットは |offset| <= ε を満たす、一様分布。これが TrueTime の保証に当たる)")
	fmt.Printf("  %-8s | %-30s | %-30s\n", "ε (ms)", "コミット待ちなし", "コミット待ちあり (ts まで待つ)")
	fmt.Printf("  %-8s | %-14s %-15s | %-14s %-15s\n", "", "順序の逆転", "待ち時間", "順序の逆転", "平均の待ち")
	rng := rand.New(rand.NewSource(1))
	for _, eps := range []float64{1, 4, 7, 20} {
		var bad1, bad2 int
		var wait float64
		N := 200000
		for i := 0; i < N; i++ {
			o1, o2 := (rng.Float64()*2-1)*eps, (rng.Float64()*2-1)*eps
			// T1: ノード 1 が、タイムスタンプ ts1 = latest = (自分の now) + ε を選ぶ。コミット (真の時刻 0)
			now1 := 0 + o1
			ts1 := now1 + eps
			// (a) コミット待ちなし: すぐに完了を、クライアントに返す (真の時刻 0)。T2 は直後 (真の時刻 0.001) に、ノード 2 で始まる
			now2 := 0.001 + o2
			ts2 := now2 + eps // T2 の ts も latest
			if ts2 <= ts1 {
				bad1++
			}
			// (b) コミット待ち: 自分の区間の earliest (now − ε) が ts1 を越えるまで待ってから、完了を返す
			//     自分の now が ts1 + ε になるまで = 真の時刻で、(ts1 + ε − o1)。
			doneT := ts1 + eps - o1
			wait += doneT
			now2b := doneT + 0.001 + o2
			ts2b := now2b + eps
			if ts2b <= ts1 {
				bad2++
			}
		}
		fmt.Printf("  %-8.0f | %-14s %-15s | %-14s %-6.2f ms\n", eps, fmt.Sprintf("%.2f%%", float64(bad1)/float64(N)*100), "0", fmt.Sprintf("%.2f%%", float64(bad2)/float64(N)*100), wait/float64(N))
	}
	fmt.Println("  → 待たないと、不確かさ ε の範囲で、順序が逆転する。ts より前に完了を返さない (待つ) と、逆転は 0 になる。代償は、平均 約 2ε の待ち時間。")
	fmt.Println("  → 同じキーへの、直列なトランザクションの、上限の速度 ≈ 1 / (2ε)。ε = 7 ms なら、約 70 回/秒 (モデルの計算。実際の Spanner の値ではない)。")
}

func main() {
	partA()
	partB()
	partC()
}
