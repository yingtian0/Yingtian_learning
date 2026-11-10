// ex5: ストレージの性能は「並列度 (キュー深さ)」で変わる
//
// 512 MiB のファイルを作り、次を測る。
//  1. 順次読み込み (1 MiB ずつ)               → MB/s
//  2. ランダム読み込み (4 KiB) を、同時に N 個の要求で出したとき → 1 秒あたりの回数 (IOPS)
//
// キュー深さ N を 1 → 64 と増やすと、IOPS はどう変わるか。
// 第1回の Little の法則 (必要な並列度 = レイテンシ × スループット) の、ストレージでの実例である。
//
// macOS では F_NOCACHE でページキャッシュを避ける。Linux では、この簡易版はキャッシュを避けないので、
// 結果がメモリの速度になりうる (root で drop_caches するか、O_DIRECT を使う改造が課題)。
package main

import (
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	fileSize = 512 << 20
	block    = 4096
	duration = 1500 * time.Millisecond
)

func noCache(f *os.File) {
	if runtime.GOOS == "darwin" {
		const fNoCache = 48 // F_NOCACHE
		syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), fNoCache, 1)
	}
}

func makeFile(path string) *os.File {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		panic(err)
	}
	noCache(f) // 書くときから、キャッシュを避ける (読み込みのときに、キャッシュに残っていないように)
	buf := make([]byte, 1<<20)
	for off := 0; off < fileSize; off += len(buf) {
		rand.Read(buf)
		if _, err := f.WriteAt(buf, int64(off)); err != nil {
			panic(err)
		}
	}
	f.Sync()
	return f
}

func main() {
	path := os.TempDir() + "/ex5_io.dat"
	fmt.Printf("ファイル作成中 (%d MiB)...\n", fileSize>>20)
	f := makeFile(path)
	defer os.Remove(path)
	defer f.Close()
	if runtime.GOOS != "darwin" {
		fmt.Println("注意: このプラットフォームでは、ページキャッシュを避けていない。結果は参考値。")
	}

	// 1. 順次読み込み
	buf := make([]byte, 1<<20)
	start := time.Now()
	for off := int64(0); off < fileSize; off += int64(len(buf)) {
		if _, err := f.ReadAt(buf, off); err != nil {
			panic(err)
		}
	}
	el := time.Since(start).Seconds()
	fmt.Printf("\n順次読み込み (1 MiB ずつ): %.0f MB/s\n", float64(fileSize)/1e6/el)

	// 2. ランダム 4 KiB 読み込み
	nblocks := int64(fileSize / block)
	fmt.Printf("\nランダム 4 KiB 読み込み (各 %.1f 秒)\n", duration.Seconds())
	fmt.Printf("%-10s %12s %16s %14s\n", "キュー深さ", "IOPS", "平均レイテンシ", "p99レイテンシ")
	for _, qd := range []int{1, 2, 4, 8, 16, 32, 64} {
		var ops atomic.Int64
		var mu sync.Mutex
		var lats []time.Duration
		stop := time.Now().Add(duration)
		var wg sync.WaitGroup
		for g := 0; g < qd; g++ {
			wg.Add(1)
			go func(seed int64) {
				defer wg.Done()
				rng := mrand.New(mrand.NewSource(seed)) // 乱数の生成がボトルネックにならないよう、goroutine ごとに持つ
				b := make([]byte, block)
				var local []time.Duration
				for time.Now().Before(stop) {
					off := rng.Int63n(nblocks) * block
					t0 := time.Now()
					if _, err := f.ReadAt(b, off); err != nil {
						panic(err)
					}
					local = append(local, time.Since(t0))
					ops.Add(1)
				}
				mu.Lock()
				lats = append(lats, local...)
				mu.Unlock()
			}(int64(g) + 1)
		}
		wg.Wait()
		sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
		var sum time.Duration
		for _, l := range lats {
			sum += l
		}
		iops := float64(ops.Load()) / duration.Seconds()
		fmt.Printf("%-10d %12.0f %13.1f µs %11.1f µs\n", qd, iops,
			float64(sum.Microseconds())/float64(len(lats)), float64(lats[len(lats)*99/100].Microseconds()))
	}
}
