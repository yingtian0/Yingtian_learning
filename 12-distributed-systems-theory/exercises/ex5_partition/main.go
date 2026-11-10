// ex5: ネットワーク分断 (パーティション) の間の、CP と AP (シミュレーション)
//
// 5 ノード。時刻 10〜20 秒の間、{A, B, C} と {D, E} に、分断される (どちらの側も、生きているが、互いに通信できない)。
// 各ノードに、クライアントが、毎秒 1 回、20 個のキーのどれかに、書き込みを要求する (合計 30 秒)。
//
//	CP (多数派が必要): 書き込みは、多数派 (3 ノード以上) に届いたときだけ、成功。届かなければ、失敗 (拒否) を返す
//	AP (常に受け付ける): 自分の側に書き、成功を返す。分断が直ったあと、2 つの側の値を、統合する
//	  統合の方法: (1) LWW (タイムスタンプの大きい方を残す)、(2) 競合を保持 (兄弟として両方を残す。アプリが解決)
//
// 見るもの: 書き込みの成功率 (特に、少数派の側)、分断が直った後の競合の数、LWW で失われた書き込みの数。
// これは、概念を数えるためのモデルで、実際のデータベースの実装ではない。
// 実行: go run ./ex5_partition
package main

import (
	"fmt"
	"math/rand"
)

type write struct {
	node, key int
	t         float64
	id        int
}

func main() {
	rng := rand.New(rand.NewSource(1))
	const nodes, keys, secs = 5, 20, 30
	side := func(n int) int { // 0: {A,B,C}、1: {D,E}
		if n < 3 {
			return 0
		}
		return 1
	}
	inPartition := func(t float64) bool { return t >= 10 && t < 20 }

	var writes []write
	id := 0
	for s := 0; s < secs; s++ {
		for n := 0; n < nodes; n++ {
			id++
			writes = append(writes, write{n, rng.Intn(keys), float64(s) + rng.Float64(), id})
		}
	}
	total := len(writes)

	// CP
	okAll, okMin, totMin, okMaj, totMaj := 0, 0, 0, 0, 0
	inPart, inPartOK := 0, 0
	for _, w := range writes {
		ok := true
		if inPartition(w.t) {
			ok = side(w.node) == 0 // 多数派 (3 ノード) の側だけ、成功
			inPart++
			if ok {
				inPartOK++
			}
			if side(w.node) == 1 {
				totMin++
				if ok {
					okMin++
				}
			} else {
				totMaj++
				if ok {
					okMaj++
				}
			}
		}
		if ok {
			okAll++
		}
	}
	fmt.Printf("書き込み要求: 全体 %d 件 (うち、分断中 %d 件。多数派側 %d、少数派側 %d)\n\n", total, inPart, totMaj, totMin)
	fmt.Println("CP (多数派が必要)")
	fmt.Printf("  成功: 全体 %d / %d (%.1f%%)。分断中: %d / %d (%.1f%%)。うち多数派側 %.0f%%、少数派側 %.0f%%\n", okAll, total, pct(okAll, total), inPartOK, inPart, pct(inPartOK, inPart), pct(okMaj, totMaj), pct(okMin, totMin))
	fmt.Println("  整合性: 成功した書き込みは、全て、多数派に記録される。分断が直った後に、競合する値は、存在しない。")

	// AP: 分断中に、2 つの側が、それぞれ受け付けた書き込みが、同じキーに対して存在するか
	side0 := map[int][]write{}
	side1 := map[int][]write{}
	for _, w := range writes {
		if inPartition(w.t) {
			if side(w.node) == 0 {
				side0[w.key] = append(side0[w.key], w)
			} else {
				side1[w.key] = append(side1[w.key], w)
			}
		}
	}
	conflicts, siblings := 0, 0
	for k := 0; k < keys; k++ {
		if len(side0[k]) > 0 && len(side1[k]) > 0 {
			conflicts++
			// LWW: 両側の、最後の書き込みを比べ、負けた側の最後の書き込み 1 件が、失われる
			// (各側の内部の、上書きは、分断がなくても起きるので、数えない)
			// 競合を保持: 各側の最後の値が、兄弟として残る
			siblings += 2
		}
	}
	fmt.Println("\nAP (常に受け付ける)")
	fmt.Printf("  成功: 全体 %d / %d (100.0%%)。分断中も、100%% (少数派側も、書き込める)\n", total, total)
	fmt.Printf("  分断が直った後: 両側が、分断中に、書いたキーは、20 個のうち %d 個 → 競合\n", conflicts)
	fmt.Printf("  統合 (1) LWW: 競合したキー %d 個で、負けた側の最後の書き込みが、1 件ずつ、静かに失われる → **成功を返したのに、失われた書き込み: %d 件** (成功した 150 件のうち %.1f%%)\n", conflicts, conflicts, pct(conflicts, total))
	fmt.Printf("  統合 (2) 競合を保持 (兄弟): 競合したキーは、各側の最後の値が、兄弟として残る (%d 個の値)。アプリが、マージする必要がある\n", siblings)
	fmt.Println("\n  → CP は、少数派側の書き込みを、拒否する (可用性を犠牲)。AP は、全て受け付けるが、競合が起き、解決の責任が、アプリに移る (一貫性を犠牲)。")
	fmt.Println("    分断は、起きるかどうかを、選べない。選べるのは、起きたときに、何を諦めるか (本文 §7)。")
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b) * 100
}
