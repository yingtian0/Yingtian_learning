// ex1: Dynamo 型の、分散キー・バリュー・ストア (簡易版、1 つのプロセスの中のシミュレーション)
//
// 部品: 一貫性ハッシュの輪 (仮想ノード付き、第10回)、N 個のレプリカ、クォーラム (R、W)、
//
//	ベクタークロックによる、並行な更新の検出 (兄弟、第12回)、読みの修復 (read repair)、
//	あいまいなクォーラム (sloppy quorum) と、ヒント付きの引き継ぎ (hinted handoff)。
//
// ネットワークの遅延は、モデル化していない (同期的)。ノードの故障と復旧だけを、起こす。
// ここで見えるのは、「故障の間の、可用性と古い読み」であり、ネットワークの遅延による、古い読み (第12回の ex4) ではない。
//
//	A. 可用性: 8 ノードのうち、k 個が落ちているとき、操作が成功する割合 (厳密なクォーラムと、あいまいなクォーラム)
//	B. 古い読み: ノードが、ランダムに落ちて、復旧する。書いた値が、読めなくなる割合
//	C. 並行な更新: 同じ文脈から、別々のコーディネータに書く。LWW だと失われ、兄弟として残せば、失われない
//
// 実行: go run ./ex1_dynamo_kv
package main

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
)

const (
	nNodes = 8
	vnodes = 64
	N      = 3
)

type vv map[int]int

func (a vv) clone() vv {
	c := vv{}
	for k, v := range a {
		c[k] = v
	}
	return c
}
func merge(a, b vv) vv {
	c := a.clone()
	for k, v := range b {
		if v > c[k] {
			c[k] = v
		}
	}
	return c
}
func dominates(a, b vv) bool { // a >= b (すべての成分)
	for k, v := range b {
		if a[k] < v {
			return false
		}
	}
	return true
}
func equal(a, b vv) bool { return dominates(a, b) && dominates(b, a) }

type version struct {
	val int
	vc  vv
}

// 版の集合を統合: 他の版に支配 (より古い) されたものを捨てる。等しいものは、1 つにする
func reconcile(vs []version) []version {
	var out []version
	for i, a := range vs {
		dominated := false
		for j, b := range vs {
			if i == j {
				continue
			}
			if dominates(b.vc, a.vc) && (!equal(a.vc, b.vc) || j < i) {
				dominated = true
				break
			}
		}
		if !dominated {
			out = append(out, a)
		}
	}
	return out
}

type hint struct {
	target, key int
	vs          []version
}
type node struct {
	up    bool
	data  map[int][]version
	hints []hint
}

type cluster struct {
	nodes []*node
	ring  []struct {
		h    uint64
		node int
	}
	sloppy      bool
	hintHandoff bool
	readRepair  bool
	R, W        int
}

func hash(s string) uint64 { h := fnv.New64a(); h.Write([]byte(s)); return h.Sum64() }

func newCluster(R, W int, sloppy, hh, rr bool) *cluster {
	c := &cluster{sloppy: sloppy, hintHandoff: hh, readRepair: rr, R: R, W: W}
	for i := 0; i < nNodes; i++ {
		c.nodes = append(c.nodes, &node{up: true, data: map[int][]version{}})
		for v := 0; v < vnodes; v++ {
			c.ring = append(c.ring, struct {
				h    uint64
				node int
			}{hash(fmt.Sprintf("n%d-v%d", i, v)), i})
		}
	}
	sort.Slice(c.ring, func(i, j int) bool { return c.ring[i].h < c.ring[j].h })
	return c
}

// 輪を時計回りに歩いて、異なるノードを、順に返す
func (c *cluster) walk(key int) []int {
	h := hash(fmt.Sprintf("k%d", key))
	i := sort.Search(len(c.ring), func(i int) bool { return c.ring[i].h >= h })
	seen := map[int]bool{}
	var out []int
	for k := 0; k < len(c.ring) && len(out) < nNodes; k++ {
		n := c.ring[(i+k)%len(c.ring)].node
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

func (c *cluster) store(n int, key int, vs []version) {
	c.nodes[n].data[key] = reconcile(append(append([]version(nil), c.nodes[n].data[key]...), vs...))
}

// 書き込み。coord < 0 なら、優先リストの、最初の生きているノード
func (c *cluster) put(key, val int, ctx vv, coord int) bool {
	w := c.walk(key)
	pref := w[:N]
	var up []int
	for _, n := range pref {
		if c.nodes[n].up {
			up = append(up, n)
		}
	}
	if !c.sloppy && len(up) < c.W {
		return false
	}
	if coord < 0 {
		if len(up) > 0 {
			coord = up[0]
		} else if c.sloppy {
			for _, n := range w {
				if c.nodes[n].up {
					coord = n
					break
				}
			}
		}
		if coord < 0 {
			return false
		}
	}
	// 新しいベクタークロック: 文脈 + コーディネータが、ローカルで見ている版 + 自分の成分を増やす
	vc := ctx.clone()
	for _, v := range c.nodes[coord].data[key] {
		vc = merge(vc, v.vc)
	}
	vc[coord]++
	nv := []version{{val, vc}}
	acks := 0
	next := N // あいまいなクォーラムの、代わりのノードを、輪の続きから探す
	for _, p := range pref {
		if c.nodes[p].up {
			c.store(p, key, nv)
			acks++
		} else if c.sloppy {
			for next < len(w) && !c.nodes[w[next]].up {
				next++
			}
			if next < len(w) {
				st := w[next]
				next++
				if c.hintHandoff {
					c.nodes[st].hints = append(c.nodes[st].hints, hint{p, key, nv})
				}
				acks++ // 代わりのノードも、ack を返す (ヒントを預かる)
			}
		}
	}
	return acks >= c.W
}

func (c *cluster) get(key int) ([]version, bool) {
	w := c.walk(key)
	var targets []int
	if c.sloppy {
		for _, n := range w { // 輪の順に、生きている最初の R 個 (代わりのノードを、含みうる)
			if c.nodes[n].up {
				targets = append(targets, n)
				if len(targets) == c.R {
					break
				}
			}
		}
	} else {
		for _, n := range w[:N] {
			if c.nodes[n].up {
				targets = append(targets, n)
				if len(targets) == c.R {
					break
				}
			}
		}
	}
	if len(targets) < c.R {
		return nil, false
	}
	var all []version
	for _, n := range targets {
		all = append(all, c.nodes[n].data[key]...)
	}
	rec := reconcile(all)
	if c.readRepair { // 読みの修復: 応答を返した後に、優先リストの、生きている全レプリカを読み、統合して、書き戻す (バックグラウンドの処理を想定)
		var upPref []int
		var every []version
		for _, n := range w[:N] {
			if c.nodes[n].up {
				upPref = append(upPref, n)
				every = append(every, c.nodes[n].data[key]...)
			}
		}
		full := reconcile(every)
		for _, n := range upPref {
			c.store(n, key, full)
		}
	}
	return rec, true
}

func (c *cluster) setUp(n int, up bool) {
	was := c.nodes[n].up
	c.nodes[n].up = up
	if up && !was && c.hintHandoff { // 復旧: 他のノードが預かっていたヒントを、受け取る
		for _, o := range c.nodes {
			if !o.up {
				continue
			}
			var rest []hint
			for _, h := range o.hints {
				if h.target == n {
					c.store(n, h.key, h.vs)
				} else {
					rest = append(rest, h)
				}
			}
			o.hints = rest
		}
	}
}

func partA() {
	fmt.Println("A. 可用性: 8 ノードのうち k 個が、ランダムに落ちている。1 万回の試行 × (書き込み 1 回 + 読み 1 回)。N = 3")
	fmt.Printf("  %-34s", "設定 (成功した操作の割合)")
	for k := 0; k <= 4; k++ {
		fmt.Printf("  k=%d落ち", k)
	}
	fmt.Println()
	for _, v := range []struct {
		name   string
		R, W   int
		sloppy bool
	}{
		{"厳密 R=2 W=2", 2, 2, false},
		{"厳密 R=3 W=3", 3, 3, false},
		{"厳密 R=1 W=1", 1, 1, false},
		{"あいまい R=2 W=2 (ヒントあり)", 2, 2, true},
		{"あいまい R=1 W=3 (ヒントあり)", 1, 3, true},
	} {
		fmt.Printf("  %-34s", v.name)
		for k := 0; k <= 4; k++ {
			rng := rand.New(rand.NewSource(1))
			ok, tot := 0, 0
			for t := 0; t < 10000; t++ {
				c := newCluster(v.R, v.W, v.sloppy, true, true)
				for _, n := range rng.Perm(nNodes)[:k] {
					c.nodes[n].up = false
				}
				key := rng.Intn(1000)
				if c.put(key, 1, vv{}, -1) {
					ok++
				}
				tot++
				if _, g := c.get(key); g {
					ok++
				}
				tot++
			}
			fmt.Printf("  %6.1f%%", float64(ok)/float64(tot)*100)
		}
		fmt.Println()
	}
	fmt.Println("  → 厳密なクォーラムは、優先リストの 3 ノードのうち、R や W 個が落ちると、失敗する。あいまいなクォーラムは、輪の続きのノードで、代わる (書き込みの可用性が高い)。")
}

func partB() {
	fmt.Println("\nB. 古い読み: 20 万ステップ。各ステップで、ノードが落ちる (確率 1%)・復旧する (落ちているノードは、確率 2%)。200 個のキーに、書き込み 30%、読み 70%。書き込みは、直前の値を読んだ文脈で行う (兄弟は、生じない)")
	fmt.Printf("  %-52s %-12s %-12s\n", "設定", "操作の失敗", "古い読み (読みの成功に対して)")
	for _, v := range []struct {
		name           string
		R, W           int
		sloppy, hh, rr bool
	}{
		{"厳密 R=1 W=1、読みの修復なし", 1, 1, false, false, false},
		{"厳密 R=1 W=1、読みの修復あり", 1, 1, false, false, true},
		{"厳密 R=2 W=2、読みの修復なし", 2, 2, false, false, false},
		{"あいまい R=2 W=2、ヒントなし", 2, 2, true, false, false},
		{"あいまい R=2 W=2、ヒントあり", 2, 2, true, true, false},
		{"あいまい R=2 W=2、ヒントあり + 読みの修復", 2, 2, true, true, true},
		{"厳密 R=3 W=3", 3, 3, false, false, false},
	} {
		rng := rand.New(rand.NewSource(1))
		c := newCluster(v.R, v.W, v.sloppy, v.hh, v.rr)
		latest := map[int]int{}
		latestVC := map[int]vv{}
		counter := 0
		fail, ops, stale, reads := 0, 0, 0, 0
		for step := 0; step < 200000; step++ {
			for n := 0; n < nNodes; n++ {
				if c.nodes[n].up && rng.Float64() < 0.01 {
					c.setUp(n, false)
				} else if !c.nodes[n].up && rng.Float64() < 0.02 {
					c.setUp(n, true)
				}
			}
			key := rng.Intn(200)
			ops++
			if rng.Float64() < 0.3 {
				counter++
				ctx := latestVC[key]
				if ctx == nil {
					ctx = vv{}
				}
				if c.put(key, counter, ctx, -1) {
					latest[key] = counter
					// 文脈: この書き込みのベクタークロック (読んだ版の統合)。簡単のため、最新の版の vc を、オラクルが追う
					w := c.walk(key)
					best := vv{}
					for _, n := range w {
						for _, ver := range c.nodes[n].data[key] {
							if ver.val == counter {
								best = ver.vc
							}
						}
					}
					for _, n := range w { // 保存が、ヒントの中だけの場合は、全ノードのヒントを探す
						for _, h := range c.nodes[n].hints {
							if h.key == key {
								for _, ver := range h.vs {
									if ver.val == counter {
										best = ver.vc
									}
								}
							}
						}
					}
					latestVC[key] = best
				} else {
					fail++
				}
			} else {
				vs, ok := c.get(key)
				if !ok {
					fail++
					continue
				}
				reads++
				fresh := latest[key] == 0
				for _, ver := range vs {
					if ver.val == latest[key] {
						fresh = true
					}
				}
				if !fresh {
					stale++
				}
			}
		}
		fmt.Printf("  %-52s %-12s %-12s\n", v.name, fmt.Sprintf("%.2f%%", float64(fail)/float64(ops)*100), fmt.Sprintf("%.2f%%", float64(stale)/float64(reads)*100))
	}
	fmt.Println("  → R=1 は、故障から復旧して、まだ追いついていないノードを読むと、古い値を返す。読みの修復が、それを減らす。")
	fmt.Println("    あいまいなクォーラムは、可用性を上げる代わりに、(R + W > N でも) 古い読みを許す。ヒントの引き継ぎが、それを減らす。")
}

func partC() {
	fmt.Println("\nC. 並行な更新 (買い物かごに、商品を追加する。ユーザーが、2 つの端末から、同時に)")
	fmt.Println("   この模擬は、同期的に複製するので、並行な更新は、自然には起きない。ここでは、2 つの更新が、互いに、複製される前に、別々のコーディネータに書かれた状況 (非同期の複製、または分断) を、直接作る。")
	const trials = 2000
	lwwLost, siblingsKept, siblingsSeen, merged := 0, 0, 0, 0
	for t := 0; t < trials; t++ {
		c := newCluster(3, 2, false, false, false)
		key := 1000 + t
		c.put(key, 0, vv{}, -1) // 空のかご
		vs, _ := c.get(key)
		ctx := vv{}
		for _, v := range vs {
			ctx = merge(ctx, v.vc)
		}
		w := c.walk(key)
		// 端末 1 は、w[0] に、商品 1 を書く。端末 2 は、同じ文脈で、w[1] に、商品 2 を書く。互いの複製は、まだ届いていない
		vc1, vc2 := ctx.clone(), ctx.clone()
		vc1[w[0]]++
		vc2[w[1]]++
		c.store(w[0], key, []version{{1, vc1}})
		c.store(w[1], key, []version{{2, vc2}})
		// その後、読み (R = 3: 全レプリカ) で、両方が見える
		got, _ := c.get(key)
		if len(got) > 1 {
			siblingsSeen++
		}
		has := map[int]bool{}
		best := 0
		for _, v := range got {
			has[v.val] = true
			if v.val > best {
				best = v.val
			}
		}
		if has[1] && has[2] {
			siblingsKept++
			merged++  // アプリが、和集合でマージして、両方の商品を、残せる
			lwwLost++ // LWW (値の大きい方だけを残す) なら、商品 1 が失われる
		}
	}
	fmt.Printf("   %d 回の試行: 兄弟 (並行な版) が、検出された: %d 回\n", trials, siblingsSeen)
	fmt.Printf("   LWW (値の大きい方だけを残す) だと、商品 1 が、失われた: %d 回\n", lwwLost)
	fmt.Printf("   兄弟として、両方を残した: %d 回 (アプリが、和集合でマージすれば、失われない)\n", siblingsKept)
	_ = merged
}

func main() {
	partA()
	partB()
	partC()
}
