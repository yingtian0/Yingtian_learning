// ex4: 小さな LSM-tree 型 KV ストアで、次のことを確かめる。
//  1. 正しさ      : ランダムな操作を、素朴な map と比べる。再起動後も同じか。
//  2. クラッシュ回復: WAL の末尾を壊して (書き込みの途中で止まった状態)、再起動したとき、何が残るか。
//  3. 同期の方針  : 書き込みごとの fsync は、どれくらい遅いか。
//  4. ブルームフィルタ: 存在しないキーの検索で、ディスク読み出しがどれだけ減るか。
//  5. 書き込み増幅 : 利用者が書いた量に対して、実際にディスクへ書いた量は何倍か。
package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

func key(i int) string { return fmt.Sprintf("key%08d", i) }

func value(rng *rand.Rand, n int) []byte {
	b := make([]byte, n)
	rng.Read(b)
	return b
}

func tmpDir() string {
	d, err := os.MkdirTemp("", "lsm")
	if err != nil {
		panic(err)
	}
	return d
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func demoCorrectness() {
	fmt.Println("[1] 正しさ: 5 万回のランダムな操作を、素朴な map と比べる")
	dir := tmpDir()
	defer os.RemoveAll(dir)
	opt := Options{MemLimit: 64 << 10, CompactAt: 4, UseBloom: true}
	db, err := Open(dir, opt)
	must(err)
	ref := map[string][]byte{}
	rng := rand.New(rand.NewSource(1))
	bad := 0
	for i := 0; i < 50000; i++ {
		k := key(rng.Intn(2000))
		switch r := rng.Intn(10); {
		case r < 6:
			v := value(rng, 50+rng.Intn(100))
			must(db.Put(k, v))
			ref[k] = v
		case r < 8:
			must(db.Delete(k))
			delete(ref, k)
		default:
			got, ok := db.Get(k)
			want, wok := ref[k]
			if ok != wok || !bytes.Equal(got, want) {
				bad++
			}
		}
	}
	verify := func(d *DB) int {
		n := 0
		for i := 0; i < 2000; i++ {
			got, ok := d.Get(key(i))
			want, wok := ref[key(i)]
			if ok != wok || !bytes.Equal(got, want) {
				n++
			}
		}
		return n
	}
	fmt.Printf("    操作中の不一致 = %d、最後の全キー確認での不一致 = %d\n", bad, verify(db))
	fmt.Printf("    書き出し %d 回、コンパクション %d 回、現在の SSTable %d 個\n", db.Stats.Flushes, db.Stats.Compactions, len(db.tables))
	db.Close()
	db2, err := Open(dir, opt)
	must(err)
	fmt.Printf("    再起動 (WAL から %d 件を再生) 後の不一致 = %d\n", db2.Stats.Recovered, verify(db2))
	db2.Close()
}

func demoCrash() {
	fmt.Println("\n[2] クラッシュ回復: WAL の末尾が、書き込みの途中で切れた状態を作る")
	dir := tmpDir()
	defer os.RemoveAll(dir)
	opt := Options{MemLimit: 1 << 30, Sync: SyncEvery}
	db, err := Open(dir, opt)
	must(err)
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < 1000; i++ {
		must(db.Put(key(i), value(rng, 100)))
	}
	db.Close() // メモリテーブルは書き出さない (クラッシュの模擬)。データは WAL にしかない
	walPath := filepath.Join(dir, "wal.log")
	st, _ := os.Stat(walPath)
	must(os.Truncate(walPath, st.Size()-7)) // 最後のレコードを、途中で切る

	db2, err := Open(dir, opt)
	must(err)
	_, has998 := db2.Get(key(998))
	_, has999 := db2.Get(key(999))
	fmt.Printf("    1000 件を書いた後、WAL の末尾 7 バイトを切って再起動\n")
	fmt.Printf("    再生できたレコード = %d、末尾の壊れたレコードを検出 = %v\n", db2.Stats.Recovered, db2.Stats.TornTail)
	fmt.Printf("    key998 (完全なレコード) = %v、key999 (切れたレコード) = %v\n", has998, has999)
	db2.Close()
	db3, _ := Open(dir, opt)
	fmt.Printf("    もう一度再起動: 再生 %d 件、壊れた末尾 = %v (前回に、ファイルを切り詰めたので、もう壊れていない)\n", db3.Stats.Recovered, db3.Stats.TornTail)
	db3.Close()
}

func demoSync() {
	fmt.Println("\n[3] 同期の方針ごとの書き込み速度 (100 バイトの値)")
	fmt.Printf("    %-34s %8s %14s %14s\n", "方針", "件数", "1件あたり", "1秒あたり")
	cases := []struct {
		name string
		opt  Options
		n    int
	}{
		{"同期なし (write のみ)", Options{MemLimit: 1 << 30, Sync: SyncNone}, 200000},
		{"100 件ごとに fsync", Options{MemLimit: 1 << 30, Sync: SyncBatch, BatchN: 100}, 20000},
		{"1 件ごとに fsync", Options{MemLimit: 1 << 30, Sync: SyncEvery}, 2000},
		{"1 件ごとに Sync() (macOS は F_FULLFSYNC)", Options{MemLimit: 1 << 30, Sync: SyncFull}, 300},
	}
	for _, c := range cases {
		dir := tmpDir()
		db, err := Open(dir, c.opt)
		must(err)
		rng := rand.New(rand.NewSource(3))
		v := value(rng, 100)
		start := time.Now()
		for i := 0; i < c.n; i++ {
			must(db.Put(key(i), v))
		}
		el := time.Since(start)
		fmt.Printf("    %-34s %8d %11.1f µs %11.0f 件\n", c.name, c.n, float64(el.Microseconds())/float64(c.n), float64(c.n)/el.Seconds())
		db.Close()
		os.RemoveAll(dir)
	}
}

func demoBloom() {
	fmt.Println("\n[4] ブルームフィルタ: 存在しないキー 2 万個を検索したときの、SSTable ブロックの読み出し回数")
	dir := tmpDir()
	defer os.RemoveAll(dir)
	db, err := Open(dir, Options{MemLimit: 1 << 20, CompactAt: 0, Sync: SyncNone}) // コンパクションなし → SSTable が増える
	must(err)
	rng := rand.New(rand.NewSource(4))
	v := value(rng, 100)
	for i := 0; i < 200000; i++ {
		must(db.Put(key(i*2), v)) // 偶数のキーだけを入れる
	}
	must(db.flush())
	fmt.Printf("    SSTable の数 = %d\n", len(db.tables))
	for _, useBloom := range []bool{false, true} {
		db.opt.UseBloom = useBloom
		db.Stats.BlockReads, db.Stats.BloomSkips = 0, 0
		qrng := rand.New(rand.NewSource(99)) // 全体の範囲から選ぶ (範囲が偏ると、多くの SSTable は、元々読まれない)
		start := time.Now()
		for i := 0; i < 20000; i++ {
			if _, ok := db.Get(key(qrng.Intn(200000)*2 + 1)); ok { // 奇数のキーは、存在しない
				panic("存在しないはずのキーが見つかった")
			}
		}
		el := time.Since(start)
		fmt.Printf("    ブルームフィルタ=%-5v ブロック読み出し %8d 回、スキップ %8d 回、検索 %.2f µs/回\n",
			useBloom, db.Stats.BlockReads, db.Stats.BloomSkips, float64(el.Microseconds())/20000)
	}
	db.Close()
}

func demoAmplification() {
	fmt.Println("\n[5] 書き込み増幅: 20 万回の上書き (キー空間 5 万)")
	dir := tmpDir()
	defer os.RemoveAll(dir)
	db, err := Open(dir, Options{MemLimit: 1 << 20, CompactAt: 4, Sync: SyncNone})
	must(err)
	rng := rand.New(rand.NewSource(5))
	v := value(rng, 100)
	for i := 0; i < 200000; i++ {
		must(db.Put(key(rng.Intn(50000)), v))
	}
	s := db.Stats
	total := s.WALBytes + s.SSTBytes
	fmt.Printf("    利用者が書いたバイト数        %10d\n", s.UserBytes)
	fmt.Printf("    WAL に書いたバイト数          %10d  (%.2f 倍)\n", s.WALBytes, float64(s.WALBytes)/float64(s.UserBytes))
	fmt.Printf("    SSTable に書いたバイト数      %10d  (%.2f 倍)  書き出し %d 回、コンパクション %d 回\n", s.SSTBytes, float64(s.SSTBytes)/float64(s.UserBytes), s.Flushes, s.Compactions)
	fmt.Printf("    合計 = 利用者の書き込みの %.2f 倍 (書き込み増幅)\n", float64(total)/float64(s.UserBytes))
	db.Close()
}

func main() {
	demoCorrectness()
	demoCrash()
	demoSync()
	demoBloom()
	demoAmplification()
}
