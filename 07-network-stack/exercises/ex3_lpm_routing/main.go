// ex3: 最長一致 (longest prefix match) のルーティングテーブル
//
// ルーターは、宛先の IP アドレスに対して、ルーティングテーブルの中の、
// 「最も長く一致するプレフィックス」を探して、次の転送先を決める。
// 3つの実装を比べる。
//
//	naive  : 全エントリを順に調べて、最長のものを選ぶ。          O(エントリ数)
//	trie   : 2進のトライ木。アドレスのビットを上から順にたどる。     O(32) (最大 32 回のメモリ参照)
//	dir248 : 上位 24 ビットで、2^24 個の表を一度で引く (DIR-24-8 の考え方)。 O(1) (/24 以下のみ対応)
//
// 正しさを、全く別の実装 (naive) との照合で確かめ、速度を比べる。
package main

import (
	"fmt"
	"math/rand"
	"time"
)

type Route struct {
	Prefix uint32
	Len    int
	NH     int32 // 次の転送先 (番号)
}

func mask(l int) uint32 {
	if l == 0 {
		return 0
	}
	return ^uint32(0) << (32 - l)
}

// ---- naive ----
type Naive struct{ routes []Route }

func (n *Naive) Lookup(ip uint32) int32 {
	best, bestLen := int32(-1), -1
	for _, r := range n.routes {
		if ip&mask(r.Len) == r.Prefix && r.Len > bestLen {
			best, bestLen = r.NH, r.Len
		}
	}
	return best
}

// ---- trie ----
type node struct {
	child [2]int32
	nh    int32
}

type Trie struct{ nodes []node }

func NewTrie() *Trie {
	return &Trie{nodes: []node{{child: [2]int32{-1, -1}, nh: -1}}}
}

func (t *Trie) Insert(r Route) {
	cur := int32(0)
	for i := 0; i < r.Len; i++ {
		bit := (r.Prefix >> (31 - i)) & 1
		next := t.nodes[cur].child[bit]
		if next < 0 {
			t.nodes = append(t.nodes, node{child: [2]int32{-1, -1}, nh: -1})
			next = int32(len(t.nodes) - 1)
			t.nodes[cur].child[bit] = next
		}
		cur = next
	}
	t.nodes[cur].nh = r.NH
}

func (t *Trie) Lookup(ip uint32) int32 {
	best := int32(-1)
	cur := int32(0)
	for i := 0; ; i++ {
		if t.nodes[cur].nh >= 0 {
			best = t.nodes[cur].nh // 途中で通った、最後の(= 最も長い)一致を覚えておく
		}
		if i == 32 {
			break
		}
		next := t.nodes[cur].child[(ip>>(31-i))&1]
		if next < 0 {
			break
		}
		cur = next
	}
	return best
}

// ---- DIR-24-8 の考え方 (/24 以下のみ) ----
type Dir24 struct{ table []int32 }

func NewDir24(routes []Route) *Dir24 {
	d := &Dir24{table: make([]int32, 1<<24)}
	for i := range d.table {
		d.table[i] = -1
	}
	// 短いプレフィックスから順に書き込み、長いものが上書きする
	sorted := append([]Route(nil), routes...)
	for l := 0; l <= 24; l++ {
		for _, r := range sorted {
			if r.Len != l {
				continue
			}
			start := r.Prefix >> 8
			n := uint32(1) << (24 - l)
			for k := uint32(0); k < n; k++ {
				d.table[start+k] = r.NH
			}
		}
	}
	return d
}

func (d *Dir24) Lookup(ip uint32) int32 { return d.table[ip>>8] }

// ---- ルートの生成: 長さは /24 が過半数で、残りは /8〜/23 ----
func genRoutes(n int, rng *rand.Rand) []Route {
	seen := map[[2]uint32]bool{}
	routes := []Route{{0, 0, 0}} // デフォルトルート 0.0.0.0/0
	seen[[2]uint32{0, 0}] = true
	for len(routes) < n {
		l := 24
		if rng.Intn(100) >= 60 {
			l = 8 + rng.Intn(16)
		}
		p := rng.Uint32() & mask(l)
		key := [2]uint32{p, uint32(l)}
		if seen[key] {
			continue
		}
		seen[key] = true
		routes = append(routes, Route{p, l, int32(len(routes))})
	}
	return routes
}

var sinkGlobal int32

func main() {
	rng := rand.New(rand.NewSource(1))
	fmt.Println("正しさ: 3つの実装が、naive と一致するか (エントリ 10,000 個、ランダムな宛先 200,000 個)")
	routes := genRoutes(10000, rng)
	naive := &Naive{routes}
	trie := NewTrie()
	for _, r := range routes {
		trie.Insert(r)
	}
	dir := NewDir24(routes)
	bad := 0
	for i := 0; i < 200000; i++ {
		ip := rng.Uint32()
		w := naive.Lookup(ip)
		if trie.Lookup(ip) != w || dir.Lookup(ip) != w {
			bad++
		}
	}
	fmt.Printf("  不一致 = %d\n\n", bad)

	fmt.Printf("速度 (1 回の検索あたり)\n  %10s %14s %14s %14s %14s\n", "エントリ数", "naive", "trie", "dir248", "trie のノード数")
	for _, n := range []int{100, 1000, 10000, 100000, 200000} {
		routes := genRoutes(n, rng)
		naive := &Naive{routes}
		trie := NewTrie()
		for _, r := range routes {
			trie.Insert(r)
		}
		dir := NewDir24(routes)
		ips := make([]uint32, 1000000)
		for i := range ips {
			ips[i] = rng.Uint32()
		}
		var sink int32 // 結果を使わないと、コンパイラがループを消してしまうので、最後に、大域変数へ書く

		t0 := time.Now()
		for _, ip := range ips {
			sink += trie.Lookup(ip)
		}
		tTrie := float64(time.Since(t0).Nanoseconds()) / float64(len(ips))

		t0 = time.Now()
		for _, ip := range ips {
			sink += dir.Lookup(ip)
		}
		tDir := float64(time.Since(t0).Nanoseconds()) / float64(len(ips))

		nn := 1000000 / n // 遅い方式は、検索の回数を減らす
		if nn < 2000 {
			nn = 2000
		}
		t0 = time.Now()
		for _, ip := range ips[:nn] {
			sink += naive.Lookup(ip)
		}
		tNaive := float64(time.Since(t0).Nanoseconds()) / float64(nn)
		sinkGlobal += sink
		fmt.Printf("  %10d %11.1f ns %11.1f ns %11.1f ns %14d\n", n, tNaive, tTrie, tDir, len(trie.nodes))
	}
	fmt.Println("\n(結果の確認用:", sinkGlobal, ")")
	fmt.Println("(dir248 は 2^24 個の表 = 64 MiB を使う。/25 以上のプレフィックスは、この簡易版では扱えない。)")
}
