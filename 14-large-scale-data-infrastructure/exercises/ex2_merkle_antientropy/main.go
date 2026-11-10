// ex2: Merkle 木による、アンチエントロピー (2 つのレプリカの、違いを、少ない通信で見つける)
//
// 2 つのレプリカが、10 万個のキーを持つ。k 個のキーだけが、食い違っている (値が違う、片方にない)。
// 素朴な方法: 全キーの (キー、版) を送って比べる。 Merkle 木: キーの範囲 (ハッシュの上位ビット) ごとに、ハッシュを作り、木にする。
// 根から比べ、違う枝だけを、下りていく。違う葉のキーだけを、交換する。
// 通信量は、「比べたハッシュの数 × 8 バイト」+「葉で交換したキーの数 × 16 バイト (キー 8 + 版 8)」として、数える (実際のプロトコルの、オーバーヘッドは、含まない)。
//
// 実行: go run ./ex2_merkle_antientropy
package main

import (
	"fmt"
	"math/rand"
)

const nKeys = 100000

type replica struct {
	ver map[uint64]uint64 // キー → 版
}

func mix(x uint64) uint64 {
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33
	return x
}

func bucket(key uint64, bits int) int { return int(mix(key) >> (64 - uint(bits))) }

// 葉のハッシュ = 葉に属するキーの、(キー、版) のハッシュの和 (順序に依存しない)。内部のハッシュ = 子のハッシュの混合
func build(r *replica, bits int) [][]uint64 {
	levels := make([][]uint64, bits+1)
	levels[bits] = make([]uint64, 1<<bits)
	for k, v := range r.ver {
		levels[bits][bucket(k, bits)] += mix(k ^ mix(v+1))
	}
	for l := bits - 1; l >= 0; l-- {
		levels[l] = make([]uint64, 1<<l)
		for i := range levels[l] {
			levels[l][i] = mix(levels[l+1][2*i]*31 + levels[l+1][2*i+1])
		}
	}
	return levels
}

type stats struct {
	hashesCompared, leafKeys, rounds, leavesDiff int
}

func sync(a, b *replica, bits int) (stats, int) {
	ta, tb := build(a, bits), build(b, bits)
	var st stats
	// 葉ごとの、キー一覧
	leafKeys := func(r *replica, leaf int) map[uint64]uint64 {
		m := map[uint64]uint64{}
		for k, v := range r.ver {
			if bucket(k, bits) == leaf {
				m[k] = v
			}
		}
		return m
	}
	// 葉の、キー一覧を作る代わりに、あらかじめ葉ごとに、索引を作っておく (比較のコストには含めない: 実システムでは、葉は、ソートされた範囲)
	idxA, idxB := map[int][]uint64{}, map[int][]uint64{}
	for k := range a.ver {
		idxA[bucket(k, bits)] = append(idxA[bucket(k, bits)], k)
	}
	for k := range b.ver {
		idxB[bucket(k, bits)] = append(idxB[bucket(k, bits)], k)
	}
	_ = leafKeys
	fixed := 0
	var walk func(level, i int)
	maxDepth := 0
	walk = func(level, i int) {
		st.hashesCompared += 2 // 双方のハッシュを、交換
		if level+1 > maxDepth {
			maxDepth = level + 1
		}
		if ta[level][i] == tb[level][i] {
			return
		}
		if level == bits {
			st.leavesDiff++
			keys := map[uint64]bool{}
			for _, k := range idxA[i] {
				keys[k] = true
			}
			for _, k := range idxB[i] {
				keys[k] = true
			}
			st.leafKeys += len(idxA[i]) + len(idxB[i]) // 葉のキー一覧 (キーと版) を、双方向に交換
			for k := range keys {
				va, oka := a.ver[k]
				vb, okb := b.ver[k]
				switch {
				case oka && (!okb || va > vb):
					b.ver[k] = va
					fixed++
				case okb && (!oka || vb > va):
					a.ver[k] = vb
					fixed++
				}
			}
			return
		}
		walk(level+1, 2*i)
		walk(level+1, 2*i+1)
	}
	walk(0, 0)
	st.rounds = maxDepth
	return st, fixed
}

func main() {
	fmt.Printf("%d 個のキー。k 個が食い違う。通信量 = 比べたハッシュ数 × 8 バイト + 葉で交換したキー数 × 16 バイト\n", nKeys)
	naive := nKeys * 16 * 2
	fmt.Printf("素朴な方法 (全キーの (キー, 版) を、双方向に送る): %d バイト (%.1f MB)\n\n", naive, float64(naive)/1e6)
	fmt.Printf("  %-8s %-10s | %-10s %-12s %-12s %-12s %-10s %s\n", "k", "葉の数", "往復 (深さ)", "ハッシュ比較", "違う葉", "葉のキー", "通信量", "素朴な方法に対して")
	for _, bits := range []int{10, 14, 17} {
		for _, k := range []int{1, 10, 100, 1000, 10000} {
			rng := rand.New(rand.NewSource(1))
			a := &replica{ver: map[uint64]uint64{}}
			b := &replica{ver: map[uint64]uint64{}}
			keys := make([]uint64, nKeys)
			for i := range keys {
				keys[i] = rng.Uint64()
				a.ver[keys[i]] = 1
				b.ver[keys[i]] = 1
			}
			for _, i := range rng.Perm(nKeys)[:k] { // k 個を、片方だけ更新 / 削除
				switch rng.Intn(3) {
				case 0:
					a.ver[keys[i]] = 2
				case 1:
					b.ver[keys[i]] = 2
				default:
					delete(b.ver, keys[i])
				}
			}
			st, fixed := sync(a, b, bits)
			ok := len(a.ver) == len(b.ver)
			for key, v := range a.ver {
				if b.ver[key] != v {
					ok = false
				}
			}
			bytes := st.hashesCompared*8 + st.leafKeys*16
			chk := ""
			if !ok {
				chk = " 【同期に失敗】"
			}
			fmt.Printf("  %-8d 2^%-8d | %-10d %-12d %-12d %-12d %-10d %.1f%% (修正 %d)%s\n", k, bits, st.rounds, st.hashesCompared, st.leavesDiff, st.leafKeys, bytes, float64(bytes)/float64(naive)*100, fixed, chk)
		}
		fmt.Println()
	}
	fmt.Println("  → 違いが少ないほど、Merkle 木は、素朴な方法より、桁違いに少ない通信で済む。違いが多い (k が全体の 1 割) と、利点は薄れる。")
	fmt.Println("    葉が大きすぎる (葉の数が少ない) と、葉のキーの交換が増え、葉が小さすぎると、木が深く、ハッシュの比較が増える。")
	fmt.Println("    ただし、実システムでは、木の構築 (全データの走査と、更新のたびの、更新) のコストが、別にかかる。この実験は、通信量だけを数えている。")
}
