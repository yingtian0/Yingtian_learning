// ex6: Go のメモリ確保 — スタックに載るか、ヒープに載るか
//
// Go では、変数がスタックとヒープのどちらに置かれるかをコンパイラが決める
// (エスケープ解析)。関数の外から参照され続ける可能性がある値はヒープに置かれ、
// ヒープに置かれた分は GC の対象になる。
//
// 実行:
//
//	go run .
//	go build -gcflags='-m' . 2>&1 | grep -E 'escapes|does not escape|moved to heap'
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
)

type point struct{ x, y int }

//go:noinline
func sumByValue() int {
	p := point{1, 2} // 関数の外に出ない → スタック
	return p.x + p.y
}

//go:noinline
func newPointer() *point {
	p := point{1, 2} // ポインタを返す → ヒープへ移動 (moved to heap)
	return &p
}

var sink interface{}

//go:noinline
func toInterface(i int) {
	sink = point{i, i} // 実行時の値を interface に入れて大域変数に保存 → ヒープ
	// (定数だけの値なら、コンパイラが読み取り専用の静的領域を使うため確保は起きない)
}

//go:noinline
func makeSmallSlice() int {
	s := make([]int, 16) // 大きさが定数で小さい → スタックに載せられる
	s[0] = 1
	return s[0]
}

//go:noinline
func makeSliceDynamic(n int) int {
	s := make([]int, n) // 大きさが実行時に決まる → ヒープ
	s[0] = 1
	return s[0]
}

func heapStats(label string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("%-34s HeapAlloc=%6.1f MB  HeapInuse=%6.1f MB  HeapIdle=%6.1f MB  HeapReleased=%6.1f MB\n",
		label, mb(m.HeapAlloc), mb(m.HeapInuse), mb(m.HeapIdle), mb(m.HeapReleased))
}

func mb(b uint64) float64 { return float64(b) / (1 << 20) }

func main() {
	fmt.Println("--- 1回の呼び出しあたりのヒープ確保回数 (allocs/op) ---")
	fmt.Printf("%-24s %.0f\n", "sumByValue", testing.AllocsPerRun(1000, func() { sumByValue() }))
	fmt.Printf("%-24s %.0f\n", "newPointer", testing.AllocsPerRun(1000, func() { newPointer() }))
	fmt.Printf("%-24s %.0f\n", "toInterface", testing.AllocsPerRun(1000, func() { toInterface(7) }))
	fmt.Printf("%-24s %.0f\n", "makeSmallSlice(const)", testing.AllocsPerRun(1000, func() { makeSmallSlice() }))
	fmt.Printf("%-24s %.0f\n", "makeSliceDynamic(16)", testing.AllocsPerRun(1000, func() { makeSliceDynamic(16) }))
	fmt.Printf("%-24s %.0f\n", "makeSliceDynamic(100000)", testing.AllocsPerRun(1000, func() { makeSliceDynamic(100000) }))

	fmt.Println("\n--- ヒープの増減と OS への返却 ---")
	heapStats("[start]")
	var keep [][]byte
	for i := 0; i < 256; i++ {
		b := make([]byte, 1<<20) // 1MiB × 256 = 256MiB
		b[0] = 1
		keep = append(keep, b)
	}
	heapStats("[256MiB を確保して保持]")
	keep = nil
	runtime.GC()
	heapStats("[参照を捨てて GC]")
	debug.FreeOSMemory()
	heapStats("[FreeOSMemory 後]")
	runtime.KeepAlive(keep)
}
