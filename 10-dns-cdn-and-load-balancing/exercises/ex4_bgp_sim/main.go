// ex4: BGP (経路制御) の簡易シミュレータ
//
// インターネットは、多数の AS (自律システム) の集まりで、BGP で、経路を交換する。
// 経路は、AS 番号の列 (AS パス) を持つ。各 AS は、「経済的な関係」に基づく方針で、経路を選び、広告する (Gao–Rexford)。
//
//	選ぶ順序: ① 顧客から学んだ経路 > ピアから > プロバイダから   ② AS パスが短い   ③ 隣の AS 番号が小さい
//	広告する規則: 顧客から学んだ経路 (と、自分の経路) は、誰にでも広告する。
//	              ピアやプロバイダから学んだ経路は、顧客にだけ広告する (谷を作らない)。
//
// 見るもの:
//  1. 正常な経路   2. 同じ長さのプレフィックスの乗っ取り   3. より具体的なプレフィックスの乗っ取り (最長一致)
//  4. ROV (RPKI による、起点の検証) を、一部の AS が行うと、どうなるか
//  5. エニーキャスト: 同じプレフィックスを、複数の場所から広告すると、どの場所に、どの AS が流れるか。1 つの場所が落ちたとき。
//  6. 経路のリーク: 顧客が、プロバイダから学んだ経路を、別のプロバイダへ、誤って広告する
//
// 注意: 簡易モデルである。実際の BGP の、多数の属性 (MED、コミュニティ、経路の集約など) や、
// 収束の時間 (数十秒〜数分)、実際のインターネットのトポロジー (約 10 万の AS) は、扱わない。
// 使うアドレスは、文書用の範囲 (203.0.113.0/24、198.51.100.0/24)。
// 実行: go run ./ex4_bgp_sim
package main

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

var names = map[int]string{1: "T1", 2: "T2", 3: "T3", 11: "A1", 12: "A2", 21: "B1", 31: "C1", 32: "C2", 101: "S1", 102: "S2", 103: "S3", 104: "S4", 105: "S5"}

// rel[a][b] = b は、a から見て何か ("customer" | "peer" | "provider")
var rel = map[int]map[int]string{}

func link(provider, customer int) {
	set(provider, customer, "customer")
	set(customer, provider, "provider")
}
func peer(a, b int) { set(a, b, "peer"); set(b, a, "peer") }
func set(a, b int, r string) {
	if rel[a] == nil {
		rel[a] = map[int]string{}
	}
	rel[a][b] = r
}

func init() {
	peer(1, 2)
	peer(2, 3)
	peer(1, 3) // 最上位 (Tier-1) は、互いにピア
	link(1, 11)
	link(1, 12)
	link(2, 21)
	link(3, 31)
	link(3, 32) // 地域の事業者
	peer(11, 12)
	peer(21, 32)
	link(11, 101)
	link(12, 102)
	link(21, 103)
	link(12, 103) // S3 は、2 つのプロバイダに接続 (マルチホーム)
	link(31, 104)
	link(32, 105) // 末端 (スタブ)
}

type Route struct {
	Prefix netip.Prefix
	Origin int
	Path   []int  // 隣から起点までの AS の列 (自分は含まない)
	Kind   string // "own" | "customer" | "peer" | "provider"
}

type Origin struct {
	AS     int
	Prefix netip.Prefix
}

type Opts struct {
	ROV     map[int]bool // 起点の検証を行う AS
	ROAAS   int          // ROA (正当な起点)
	ROAPfx  netip.Prefix
	Leakers map[int]bool // 経路を、規則に反して、全員に広告する AS
}

func pref(kind string) int {
	return map[string]int{"own": 4, "customer": 3, "peer": 2, "provider": 1}[kind]
}

func better(a, b Route, na, nb int) bool {
	if pref(a.Kind) != pref(b.Kind) {
		return pref(a.Kind) > pref(b.Kind)
	}
	if len(a.Path) != len(b.Path) {
		return len(a.Path) < len(b.Path)
	}
	return na < nb
}

func invalid(o Opts, pfx netip.Prefix, origin int) bool {
	if !o.ROAPfx.IsValid() || !o.ROAPfx.Overlaps(pfx) {
		return false
	}
	return origin != o.ROAAS || pfx.Bits() > o.ROAPfx.Bits() // 起点が違う、または、長すぎる
}

func converge(origins []Origin, o Opts) map[int]map[netip.Prefix]Route {
	var asns []int
	for a := range names {
		asns = append(asns, a)
	}
	sort.Ints(asns)
	pfxs := map[netip.Prefix]bool{}
	for _, og := range origins {
		pfxs[og.Prefix] = true
	}
	cur := map[int]map[netip.Prefix]Route{}
	for _, a := range asns {
		cur[a] = map[netip.Prefix]Route{}
	}
	for round := 0; round < 50; round++ {
		next := map[int]map[netip.Prefix]Route{}
		changed := false
		for _, a := range asns {
			next[a] = map[netip.Prefix]Route{}
			for p := range pfxs {
				var best *Route
				bestN := 0
				for _, og := range origins {
					if og.AS == a && og.Prefix == p {
						r := Route{Prefix: p, Origin: a, Kind: "own"}
						best = &r
					}
				}
				if best == nil {
					for n, k := range rel[a] {
						nr, ok := cur[n][p]
						if !ok {
							continue
						}
						// n が、a に、この経路を広告するか
						exports := nr.Kind == "own" || nr.Kind == "customer" || rel[n][a] == "customer" || o.Leakers[n]
						if !exports {
							continue
						}
						loop := false
						for _, x := range nr.Path {
							if x == a {
								loop = true
							}
						}
						if loop || nr.Origin == a {
							continue
						}
						if o.ROV[a] && invalid(o, p, nr.Origin) {
							continue // ROV: 起点が不正な経路は、捨てる
						}
						cand := Route{Prefix: p, Origin: nr.Origin, Path: append([]int{n}, nr.Path...), Kind: k}
						if best == nil || better(cand, *best, n, bestN) {
							c := cand
							best, bestN = &c, n
						}
					}
				}
				if best != nil {
					next[a][p] = *best
					if old, ok := cur[a][p]; !ok || fmt.Sprint(old.Path, old.Origin) != fmt.Sprint(best.Path, best.Origin) {
						changed = true
					}
				} else if _, ok := cur[a][p]; ok {
					changed = true
				}
			}
		}
		cur = next
		if !changed {
			break
		}
	}
	return cur
}

// 宛先アドレスに対して、AS a が、どの起点へ送るか (最長一致)
func dest(tables map[int]map[netip.Prefix]Route, a int, addr netip.Addr) (Route, bool) {
	var best Route
	found := false
	for p, r := range tables[a] {
		if p.Contains(addr) && (!found || p.Bits() > best.Prefix.Bits()) {
			best, found = r, true
		}
	}
	return best, found
}

func pathStr(r Route, a int) string {
	var ps []string
	ps = append(ps, names[a])
	for _, x := range r.Path {
		ps = append(ps, names[x])
	}
	return strings.Join(ps, " → ")
}

func show(title string, tables map[int]map[netip.Prefix]Route, addr netip.Addr, origins ...int) {
	fmt.Printf("\n  %s\n", title)
	var asns []int
	for a := range names {
		asns = append(asns, a)
	}
	sort.Ints(asns)
	for _, a := range asns {
		r, ok := dest(tables, a, addr)
		if !ok {
			fmt.Printf("    %-3s 経路なし\n", names[a])
			continue
		}
		fmt.Printf("    %-3s → 行き先 %-3s  (%s)  プレフィックス %s、学んだ元: %s\n", names[a], names[r.Origin], pathStr(r, a), r.Prefix, r.Kind)
	}
}

func count(tables map[int]map[netip.Prefix]Route, addr netip.Addr, excluded map[int]bool) map[int]int {
	c := map[int]int{}
	for a := range names {
		if excluded[a] {
			continue
		}
		if r, ok := dest(tables, a, addr); ok {
			c[r.Origin]++
		} else {
			c[0]++
		}
	}
	return c
}

func fmtCount(c map[int]int) string {
	var ks []int
	for k := range c {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	var parts []string
	for _, k := range ks {
		n := "経路なし"
		if k != 0 {
			n = names[k]
		}
		parts = append(parts, fmt.Sprintf("%s へ: %d 個", n, c[k]))
	}
	return strings.Join(parts, "、")
}

func main() {
	P := netip.MustParsePrefix("203.0.113.0/24")
	P25 := netip.MustParsePrefix("203.0.113.0/25")
	addr := netip.MustParseAddr("203.0.113.10")
	fmt.Println("トポロジー (13 個の AS): T1・T2・T3 は最上位 (互いにピア)。A1・A2・B1・C1・C2 は地域の事業者。S1〜S5 は末端。")
	fmt.Println("  T1 の顧客: A1、A2   T2 の顧客: B1   T3 の顧客: C1、C2   A1–A2 はピア   B1–C2 はピア")
	fmt.Println("  A1 の顧客: S1   A2 の顧客: S2、S3   B1 の顧客: S3   C1 の顧客: S4   C2 の顧客: S5   (S3 は A2 と B1 の両方に接続)")

	fmt.Println("\n1. 正常: S4 が、203.0.113.0/24 を広告する")
	t1 := converge([]Origin{{104, P}}, Opts{})
	show("各 AS が、203.0.113.10 宛てに選ぶ経路", t1, addr)

	fmt.Println("\n2. 乗っ取り (同じ長さ): S1 も、同じ 203.0.113.0/24 を広告する")
	t2 := converge([]Origin{{104, P}, {101, P}}, Opts{})
	c2 := count(t2, addr, map[int]bool{104: true, 101: true})
	fmt.Printf("  S4 (正当)、S1 (攻撃者) 以外の 11 個の AS の行き先: %s\n", fmtCount(c2))
	show("各 AS の選んだ経路", t2, addr)

	fmt.Println("\n3. 乗っ取り (より具体的): S1 が、203.0.113.0/25 を広告する (正当な S4 は /24 のまま)")
	t3 := converge([]Origin{{104, P}, {101, P25}}, Opts{})
	c3 := count(t3, addr, map[int]bool{104: true, 101: true})
	fmt.Printf("  11 個の AS の行き先 (203.0.113.10 宛て): %s\n", fmtCount(c3))
	fmt.Println("  → 最長一致で、/25 が /24 に勝つ。AS パスの長さ、関係の優先は、関係ない (別のプレフィックスだから)。")

	fmt.Println("\n4. ROV (RPKI による、起点の検証): ROA は「203.0.113.0/24 の起点は S4、最大の長さは /24」")
	for _, d := range []struct {
		name string
		rov  []int
	}{
		{"ROV なし", nil},
		{"最上位 (T1、T2、T3) だけが ROV", []int{1, 2, 3}},
		{"すべての事業者 (最上位 + 地域) が ROV (末端は行わない)", []int{1, 2, 3, 11, 12, 21, 31, 32}},
	} {
		rov := map[int]bool{}
		for _, a := range d.rov {
			rov[a] = true
		}
		o := Opts{ROV: rov, ROAAS: 104, ROAPfx: P}
		a := count(converge([]Origin{{104, P}, {101, P25}}, o), addr, map[int]bool{104: true, 101: true})
		b := count(converge([]Origin{{104, P}, {101, P}}, o), addr, map[int]bool{104: true, 101: true})
		fmt.Printf("  %-52s /25 の乗っ取り → %s\n  %-52s /24 の乗っ取り → %s\n", d.name, fmtCount(a), "", fmtCount(b))
	}

	fmt.Println("\n5. エニーキャスト: 198.51.100.0/24 を、S1、S3、S5 の 3 か所から、同時に広告する")
	Q := netip.MustParsePrefix("198.51.100.0/24")
	qaddr := netip.MustParseAddr("198.51.100.1")
	t5 := converge([]Origin{{101, Q}, {103, Q}, {105, Q}}, Opts{})
	show("各 AS が、どの場所に流れるか", t5, qaddr)
	fmt.Printf("  3 か所以外の 10 個の AS: %s\n", fmtCount(count(t5, qaddr, map[int]bool{101: true, 103: true, 105: true})))
	t5b := converge([]Origin{{101, Q}, {105, Q}}, Opts{})
	fmt.Printf("  S3 が落ちた (広告を取り下げた) あと、11 個の AS (S3 を含む): %s\n", fmtCount(count(t5b, qaddr, map[int]bool{101: true, 105: true})))

	fmt.Println("\n6. 経路のリーク: S3 (A2 と B1 の顧客) が、プロバイダから学んだ経路を、もう一方のプロバイダにも広告してしまう")
	for _, leak := range []bool{false, true} {
		o := Opts{}
		if leak {
			o.Leakers = map[int]bool{103: true}
		}
		t6 := converge([]Origin{{104, P}}, o)
		through := 0
		var who []string
		for a := range names {
			if a == 103 || a == 104 {
				continue
			}
			if r, ok := dest(t6, a, addr); ok {
				for _, x := range r.Path {
					if x == 103 {
						through++
						who = append(who, names[a])
					}
				}
			}
		}
		sort.Strings(who)
		fmt.Printf("  リーク%s: S4 宛ての経路が、S3 (末端 AS) を通る AS = %d 個 %v\n", map[bool]string{false: "なし", true: "あり"}[leak], through, who)
	}
	fmt.Println("  → 末端の AS が、transit (他人の通信を運ぶ) の役を、意図せず、引き受けてしまう。その回線は、通信量に耐えられない。")
}
