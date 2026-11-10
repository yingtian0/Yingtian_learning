// ex3: イレイジャーコーディング (Reed–Solomon) を実装し、複製と、耐久性・容量・修復の通信量を、比べる
//
//	A. GF(256) 上の Reed–Solomon (Cauchy 行列) を、実装し、k + m 個のシャードの、どの k 個からでも、元に戻ることを、全ての組み合わせで確認する
//	B. 容量のオーバーヘッドと、修復の通信量 (壊れた 1 つを、作り直すために、読むバイト数)
//	C. 耐久性: 各シャードが、別のノードにあり、ノードが独立に、故障する、というモデルで、データを失うまでの、平均時間 (MTTDL) を、マルコフ連鎖で計算
//
// C のモデルは、強い仮定を置く: 故障は独立、故障率は一定、修復は、並列で、一定の時間。実際の障害は、相関する (同じラック、同じ電源、同じバグ) ので、
// ここで出る耐久性の数値は、実システムの、耐久性ではなく、「符号の構成による、相対的な違い」を見るためのものである。
// 実行: go run ./ex3_erasure_coding
package main

import (
	"fmt"
	"math"
	"math/rand"
)

var expT [512]byte
var logT [256]int

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		expT[i] = byte(x)
		logT[x] = i
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11d
		}
	}
	for i := 255; i < 512; i++ {
		expT[i] = expT[i-255]
	}
}

func gmul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return expT[logT[a]+logT[b]]
}
func ginv(a byte) byte { return expT[255-logT[a]] }

// 符号化行列: (k+m) × k。上 k 行は単位行列 (系統的)、下 m 行は Cauchy 行列
func encMatrix(k, m int) [][]byte {
	M := make([][]byte, k+m)
	for i := range M {
		M[i] = make([]byte, k)
		if i < k {
			M[i][i] = 1
		} else {
			for j := 0; j < k; j++ {
				M[i][j] = ginv(byte(i) ^ byte(j)) // x_i = i (k..k+m-1)、y_j = j (0..k-1) は、互いに異なる
			}
		}
	}
	return M
}

func encode(M [][]byte, k, m int, data [][]byte) [][]byte {
	n := len(data[0])
	shards := make([][]byte, k+m)
	copy(shards, data)
	for i := k; i < k+m; i++ {
		shards[i] = make([]byte, n)
		for j := 0; j < k; j++ {
			c := M[i][j]
			for b := 0; b < n; b++ {
				shards[i][b] ^= gmul(c, data[j][b])
			}
		}
	}
	return shards
}

// 行列の逆行列 (GF(256) 上のガウスの消去法)
func invert(A [][]byte) ([][]byte, bool) {
	n := len(A)
	W := make([][]byte, n)
	for i := range W {
		W[i] = make([]byte, 2*n)
		copy(W[i], A[i])
		W[i][n+i] = 1
	}
	for c := 0; c < n; c++ {
		p := -1
		for r := c; r < n; r++ {
			if W[r][c] != 0 {
				p = r
				break
			}
		}
		if p < 0 {
			return nil, false
		}
		W[c], W[p] = W[p], W[c]
		iv := ginv(W[c][c])
		for j := range W[c] {
			W[c][j] = gmul(W[c][j], iv)
		}
		for r := 0; r < n; r++ {
			if r != c && W[r][c] != 0 {
				f := W[r][c]
				for j := range W[r] {
					W[r][j] ^= gmul(f, W[c][j])
				}
			}
		}
	}
	R := make([][]byte, n)
	for i := range R {
		R[i] = W[i][n:]
	}
	return R, true
}

// 残っているシャードの添字 (k 個) から、元のデータを復元
func decode(M [][]byte, k int, shards [][]byte, have []int) ([][]byte, bool) {
	A := make([][]byte, k)
	for i, idx := range have {
		A[i] = M[idx]
	}
	inv, ok := invert(A)
	if !ok {
		return nil, false
	}
	n := len(shards[have[0]])
	out := make([][]byte, k)
	for i := 0; i < k; i++ {
		out[i] = make([]byte, n)
		for j := 0; j < k; j++ {
			c := inv[i][j]
			for b := 0; b < n; b++ {
				out[i][b] ^= gmul(c, shards[have[j]][b])
			}
		}
	}
	return out, true
}

func combos(n, k int, f func([]int)) {
	idx := make([]int, k)
	var rec func(start, d int)
	rec = func(start, d int) {
		if d == k {
			f(idx)
			return
		}
		for i := start; i < n; i++ {
			idx[d] = i
			rec(i+1, d+1)
		}
	}
	rec(0, 0)
}

func partA() {
	fmt.Println("A. Reed–Solomon の、正しさ: k + m 個のシャードの、どの k 個からでも、元のデータを、復元できるか (全ての組み合わせ)")
	rng := rand.New(rand.NewSource(1))
	for _, c := range []struct{ k, m int }{{3, 2}, {6, 3}, {10, 4}} {
		M := encMatrix(c.k, c.m)
		data := make([][]byte, c.k)
		for i := range data {
			data[i] = make([]byte, 256)
			rng.Read(data[i])
		}
		shards := encode(M, c.k, c.m, data)
		total, okc := 0, 0
		combos(c.k+c.m, c.k, func(have []int) {
			total++
			out, ok := decode(M, c.k, shards, have)
			if !ok {
				return
			}
			same := true
			for i := range out {
				for b := range out[i] {
					if out[i][b] != data[i][b] {
						same = false
					}
				}
			}
			if same {
				okc++
			}
		})
		fmt.Printf("   RS(k=%d, m=%d): %d / %d 通りの組み合わせで、復元に成功\n", c.k, c.m, okc, total)
		// k - 1 個では、復元できない (情報が足りない) ことを、確認: 行列が、特異になる
	}
}

// 平均のデータ喪失時間 (MTTDL)。状態 s = 壊れているシャードの数。s = m + 1 で喪失 (吸収)
// T_s = 状態 s から s + 1 に、初めて到達するまでの、平均時間。T_s = 1/f_s + (r_s/f_s) T_{s-1}  (f_s = (n-s)λ、r_s = sμ)
// MTTDL = T_0 + … + T_m。全ての項が、正で、引き算がないので、桁が大きくても、数値が安定する
func mttdl(n, m int, lambda, mu float64) float64 {
	sum, prev := 0.0, 0.0
	for s := 0; s <= m; s++ {
		f := float64(n-s) * lambda
		t := 1 / f
		if s > 0 {
			t += float64(s) * mu / f * prev
		}
		sum += t
		prev = t
	}
	return sum
}

func partBC() {
	fmt.Println("\nB・C. 構成の比較 (ノードの年間故障率 AFR = 4%、故障は独立、修復は並列で、一定の時間)")
	const afr = 0.04
	lambda := afr / 8760 // 1 時間あたり
	fmt.Printf("  %-18s %-8s %-10s %-14s %-12s | %-28s | %s\n", "構成", "ノード数", "容量の倍率", "耐えられる故障", "修復の読み出し", "年間のデータ喪失確率 (修復 1 時間)", "(修復 24 時間)")
	for _, c := range []struct {
		name string
		n, m int
		k    int
	}{
		{"複製 2 倍", 2, 1, 1},
		{"複製 3 倍", 3, 2, 1},
		{"RS(6, 3)", 9, 3, 6},
		{"RS(10, 4)", 14, 4, 10},
		{"RS(12, 4)", 16, 4, 12},
	} {
		row := fmt.Sprintf("  %-18s %-8d %-10.2f %-14d %-12s |", c.name, c.n, float64(c.n)/float64(c.k), c.m, fmt.Sprintf("%d 倍", c.k))
		for _, hours := range []float64{1, 24} {
			mu := 1 / hours
			t := mttdl(c.n, c.m, lambda, mu)
			p := -math.Expm1(-8760 / t)
			if hours == 1 {
				row += fmt.Sprintf(" %-34s |", fmt.Sprintf("%.2e (MTTDL %.1e 年)", p, t/8760))
			} else {
				row += fmt.Sprintf(" %.2e (MTTDL %.1e 年)", p, t/8760)
			}
		}
		fmt.Println(row)
	}
	fmt.Println("\n  「修復の読み出し」= 壊れた 1 シャードを、作り直すのに、読まなければならない量 (壊れたシャードの大きさに対する倍率)。複製は、1 つコピーすればよい。RS は、k 個のシャードを読む。")
	fmt.Println("  → RS(10, 4) は、容量が 1.4 倍で、4 つの故障に耐える (複製 3 倍は、容量 3 倍で、2 つ)。代償は、修復で、10 倍を読むこと (帯域)、符号化・復号の CPU、1 シャードの読みが、遅いと、全体が遅れること。")
	fmt.Println("  → 実際の障害は、独立でない (同じラック、同じバッチのディスク、同じソフトウェアのバグ) ので、この数値は、実システムの耐久性を、そのまま表さない。")
}

func main() {
	partA()
	partBC()
}
