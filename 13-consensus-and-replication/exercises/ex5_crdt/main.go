// ex5: CRDT (競合のない複製データ型) — 収束することの確認と、素朴な設計の失敗
//
// 状態ベースの CRDT: 各レプリカが、状態を持ち、他のレプリカの状態を、受け取って、merge する。
// merge が、交換則、結合則、冪等であれば、メッセージの順序の入れ替わりや、重複があっても、全レプリカは、同じ状態に収束する。
//
//  1. G-Counter (増えるだけのカウンタ)、PN-Counter、OR-Set (追加が勝つ集合)、LWW-Register
//  2. merge の 3 つの性質を、ランダムな状態で、検査する
//  3. 5 つのレプリカで、ランダムな操作と、ランダムな順序・重複の gossip を行い、最後に、収束し、意味が正しいことを確認する
//  4. 素朴な設計との比較: (a) 値の max で merge するカウンタ、(b) 削除を、ローカルにだけ反映する集合、(c) 2P-Set (削除したら、再追加できない)
//
// 実行: go run ./ex5_crdt
package main

import (
	"fmt"
	"math/rand"
	"sort"
)

const R = 5

// ---- G-Counter / PN-Counter ----
type GC [R]int

func (a GC) merge(b GC) GC {
	for i := range a {
		if b[i] > a[i] {
			a[i] = b[i]
		}
	}
	return a
}
func (a GC) value() int {
	s := 0
	for _, v := range a {
		s += v
	}
	return s
}

type PN struct{ P, N GC }

func (a PN) merge(b PN) PN { return PN{a.P.merge(b.P), a.N.merge(b.N)} }
func (a PN) value() int    { return a.P.value() - a.N.value() }

// ---- OR-Set: 要素ごとに、(追加のタグ集合、削除されたタグ集合)。タグ = (レプリカ, 番号)
type tag struct{ r, n int }
type ORS struct {
	adds map[int]map[tag]bool
	rems map[tag]bool
}

func newORS() ORS { return ORS{map[int]map[tag]bool{}, map[tag]bool{}} }
func (s ORS) clone() ORS {
	c := newORS()
	for e, ts := range s.adds {
		c.adds[e] = map[tag]bool{}
		for t := range ts {
			c.adds[e][t] = true
		}
	}
	for t := range s.rems {
		c.rems[t] = true
	}
	return c
}
func (s ORS) add(e int, t tag) {
	if s.adds[e] == nil {
		s.adds[e] = map[tag]bool{}
	}
	s.adds[e][t] = true
}
func (s ORS) remove(e int) { // 今、見えている追加のタグを、全て削除済みにする
	for t := range s.adds[e] {
		s.rems[t] = true
	}
}
func (s ORS) has(e int) bool {
	for t := range s.adds[e] {
		if !s.rems[t] {
			return true
		}
	}
	return false
}
func (s ORS) merge(o ORS) ORS {
	c := s.clone()
	for e, ts := range o.adds {
		for t := range ts {
			c.add(e, t)
		}
	}
	for t := range o.rems {
		c.rems[t] = true
	}
	return c
}
func (s ORS) elems() []int {
	var out []int
	for e := range s.adds {
		if s.has(e) {
			out = append(out, e)
		}
	}
	sort.Ints(out)
	return out
}
func eq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---- LWW-Register (論理時刻 + レプリカ番号で、全順序) ----
type LWW struct{ ts, rep, val int }

func (a LWW) merge(b LWW) LWW {
	if b.ts > a.ts || (b.ts == a.ts && b.rep > a.rep) {
		return b
	}
	return a
}

func partPropertiesTest() {
	rng := rand.New(rand.NewSource(1))
	rgc := func() GC {
		var g GC
		for i := range g {
			g[i] = rng.Intn(10)
		}
		return g
	}
	rors := func() ORS {
		s := newORS()
		for i := 0; i < rng.Intn(6); i++ {
			s.add(rng.Intn(4), tag{rng.Intn(R), rng.Intn(5)})
		}
		for i := 0; i < rng.Intn(3); i++ {
			s.rems[tag{rng.Intn(R), rng.Intn(5)}] = true
		}
		return s
	}
	bad := map[string]int{}
	const N = 20000
	for i := 0; i < N; i++ {
		a, b, c := rgc(), rgc(), rgc()
		if a.merge(b) != b.merge(a) {
			bad["G-Counter 交換則"]++
		}
		if a.merge(b).merge(c) != a.merge(b.merge(c)) {
			bad["G-Counter 結合則"]++
		}
		if a.merge(a) != a {
			bad["G-Counter 冪等"]++
		}
		x, y, z := rors(), rors(), rors()
		if !eq(x.merge(y).elems(), y.merge(x).elems()) {
			bad["OR-Set 交換則"]++
		}
		if !eq(x.merge(y).merge(z).elems(), x.merge(y.merge(z)).elems()) {
			bad["OR-Set 結合則"]++
		}
		if !eq(x.merge(x).elems(), x.elems()) {
			bad["OR-Set 冪等"]++
		}
		mk := func() LWW { // 同じ (時刻, レプリカ) から、異なる値は作れない (各レプリカの時刻は、書き込みごとに増える)
			t, r := rng.Intn(5), rng.Intn(R)
			return LWW{t, r, t*10 + r}
		}
		l1, l2, l3 := mk(), mk(), mk()
		if l1.merge(l2) != l2.merge(l1) || l1.merge(l2).merge(l3) != l1.merge(l2.merge(l3)) || l1.merge(l1) != l1 {
			bad["LWW-Register"]++
		}
	}
	fmt.Printf("1. merge の性質 (交換則・結合則・冪等) を、ランダムな状態 %d 組で検査\n", N)
	if len(bad) == 0 {
		fmt.Println("   違反: なし (G-Counter、OR-Set、LWW-Register)")
	}
	for k, v := range bad {
		fmt.Printf("   違反: %s: %d\n", k, v)
	}
}

// gossip: 各ステップで、ランダムな 2 つのレプリカの間で、(古い可能性のある) 状態を、受け取って merge する。重複や順序の入れ替わりを、含む
func partConvergence() {
	fmt.Println("\n2. 5 つのレプリカ、各 100 回のランダムな操作と、ランダムな gossip (古い状態の再送、重複を含む)。200 回の実行")
	const runs = 200
	okGC, okPN, okORS, okLWW := 0, 0, 0, 0
	naiveMaxWrong, resurrect, twoPLost := 0, 0, 0
	addWinsOK := 0
	for seed := int64(0); seed < runs; seed++ {
		rng := rand.New(rand.NewSource(seed))
		gc := make([]GC, R)
		pn := make([]PN, R)
		ors := make([]ORS, R)
		lww := make([]LWW, R)
		naive := make([]int, R)         // (a) 値の max で merge
		nset := make([]map[int]bool, R) // (b) 集合: 追加の和集合。削除は、ローカルの集合からだけ消す
		tpA := make([]map[int]bool, R)  // (c) 2P-Set: 追加集合と、削除集合 (どちらも、和集合で merge)
		tpR := make([]map[int]bool, R)
		for i := 0; i < R; i++ {
			ors[i] = newORS()
			nset[i], tpA[i], tpR[i] = map[int]bool{}, map[int]bool{}, map[int]bool{}
		}
		totalInc, totalPN := 0, 0
		clock := 0
		counter := make([]int, R)
		history := []([]ORS){}
		_ = history
		// 参照: 因果的に、追加が削除に先行したか、を厳密に追うのは大変なので、最終的な意味の検査は、単純な追加勝ちの場面 (並行な add と remove) で、別に行う
		snapshots := make([][]GC, 0)
		for step := 0; step < 100*R; step++ {
			r := rng.Intn(R)
			switch rng.Intn(6) {
			case 0, 1: // カウンタを増やす
				gc[r][r]++
				pn[r].P[r]++
				naive[r]++
				totalInc++
				totalPN++
			case 2: // PN: 減らす
				pn[r].N[r]++
				totalPN--
			case 3: // 集合に追加 (要素 0〜5)
				e := rng.Intn(6)
				counter[r]++
				ors[r].add(e, tag{r, counter[r]})
				nset[r][e] = true
				tpA[r][e] = true
			case 4: // 集合から削除
				e := rng.Intn(6)
				ors[r].remove(e)
				delete(nset[r], e)
				if tpA[r][e] {
					tpR[r][e] = true
				}
			case 5: // LWW 書き込み
				clock++
				lww[r] = lww[r].merge(LWW{clock, r, rng.Intn(1000)})
			}
			// gossip: ランダムに、1〜2 回。送り元の状態は、現在のものか、過去のスナップショット (古い再送)
			for g := 0; g < 1+rng.Intn(2); g++ {
				a, b := rng.Intn(R), rng.Intn(R)
				if a == b {
					continue
				}
				snapshots = append(snapshots, append([]GC(nil), gc...))
				srcGC := gc[a]
				if len(snapshots) > 3 && rng.Float64() < 0.3 {
					srcGC = snapshots[rng.Intn(len(snapshots))][a] // 古い状態の再送
				}
				gc[b] = gc[b].merge(srcGC)
				pn[b] = pn[b].merge(pn[a])
				ors[b] = ors[b].merge(ors[a])
				lww[b] = lww[b].merge(lww[a])
				if naive[a] > naive[b] {
					naive[b] = naive[a]
				}
				for e := range nset[a] {
					nset[b][e] = true
				}
				for e := range tpA[a] {
					tpA[b][e] = true
				}
				for e := range tpR[a] {
					tpR[b][e] = true
				}
			}
		}
		// 最後に、全員が、全員と、同期する (数回)
		for round := 0; round < 3; round++ {
			for a := 0; a < R; a++ {
				for b := 0; b < R; b++ {
					gc[b] = gc[b].merge(gc[a])
					pn[b] = pn[b].merge(pn[a])
					ors[b] = ors[b].merge(ors[a])
					lww[b] = lww[b].merge(lww[a])
					if naive[a] > naive[b] {
						naive[b] = naive[a]
					}
					for e := range nset[a] {
						nset[b][e] = true
					}
					for e := range tpA[a] {
						tpA[b][e] = true
					}
					for e := range tpR[a] {
						tpR[b][e] = true
					}
				}
			}
		}
		allGC, allPN, allORS, allLWW := true, true, true, true
		for i := 1; i < R; i++ {
			allGC = allGC && gc[i] == gc[0]
			allPN = allPN && pn[i] == pn[0]
			allORS = allORS && eq(ors[i].elems(), ors[0].elems())
			allLWW = allLWW && lww[i] == lww[0]
		}
		if allGC && gc[0].value() == totalInc {
			okGC++
		}
		if allPN && pn[0].value() == totalPN {
			okPN++
		}
		if allORS {
			okORS++
		}
		if allLWW {
			okLWW++
		}
		if naive[0] != totalInc {
			naiveMaxWrong++
		}
		// (b): 削除を、ローカルにだけ反映する集合 → 最終的に、OR-Set には無い要素が、残る (復活) か
		orsE := map[int]bool{}
		for _, e := range ors[0].elems() {
			orsE[e] = true
		}
		for e := range nset[0] {
			if !orsE[e] {
				resurrect++
				break
			}
		}
		// (c): 2P-Set で、削除済みのため、見えない要素のうち、OR-Set には、(再追加により) 見えているもの = 再追加が失われた
		for e := range tpR[0] {
			if orsE[e] {
				twoPLost++
				break
			}
		}
		// 追加勝ち: 並行な add と remove を、直接作って検査
		a, b := newORS(), newORS()
		a.add(1, tag{0, 1})
		b = b.merge(a)
		b.remove(1)         // b が、見えている追加を削除
		a.add(1, tag{0, 2}) // a が、並行に、再度、追加
		if a.merge(b).has(1) && b.merge(a).has(1) {
			addWinsOK++
		}
	}
	fmt.Printf("   G-Counter: 全レプリカが同じ状態、かつ、値 = 増やした回数の合計: %d / %d\n", okGC, runs)
	fmt.Printf("   PN-Counter: 同上 (増減の合計): %d / %d\n", okPN, runs)
	fmt.Printf("   OR-Set: 全レプリカが、同じ集合に収束: %d / %d\n", okORS, runs)
	fmt.Printf("   LWW-Register: 全レプリカが、同じ値に収束: %d / %d\n", okLWW, runs)
	fmt.Printf("   OR-Set の『追加が勝つ』(並行な追加と削除 → 要素は残る): %d / %d\n", addWinsOK, runs)
	fmt.Println("\n3. 素朴な設計との比較 (同じ操作と gossip で)")
	fmt.Printf("   (a) 値の max で merge するカウンタ: 最終的な値が、増やした合計と、食い違った実行: %d / %d (並行な増加が、失われる)\n", naiveMaxWrong, runs)
	fmt.Printf("   (b) 削除を、ローカルにだけ反映する集合 (和集合で merge): 削除した要素が、復活した実行: %d / %d\n", resurrect, runs)
	fmt.Printf("   (c) 2P-Set (削除集合は、和集合。削除後は、再追加できない): 削除後の再追加が、失われた実行: %d / %d\n", twoPLost, runs)
}

func main() {
	partPropertiesTest()
	partConvergence()
	fmt.Println("\n  → CRDT は、merge が、交換則・結合則・冪等なので、順序の入れ替わりや重複があっても、調整なしで収束する (強い結果整合性)。")
	fmt.Println("    ただし、収束した結果の『意味』が、アプリの望むものかは、別の問題 (例: 追加勝ちか、削除勝ちか。残高のような、不変条件は、CRDT だけでは守れない)。")
}
