// 小さな LSM-tree 型のキーバリューストア。仕組みを見るための実装で、実用品ではない。
//
// 構成:
//
//	WAL (wal.log)  : 書き込みを、まず追記する。クラッシュしても、再起動時に再生できる。各レコードに CRC を付ける。
//	メモリテーブル : 最近の書き込みを、メモリ上の map に持つ。
//	SSTable (*.sst): メモリテーブルが大きくなったら、キー順に並べて、不変のファイルとして書き出す。
//	コンパクション : SSTable が増えたら、全部を1つにマージする (古い版と削除済みを捨てる)。
//
// 読み出しは、メモリテーブル → 新しい SSTable → 古い SSTable の順に探す。
package main

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

type SyncPolicy int

const (
	SyncNone  SyncPolicy = iota // write() だけ。OS のページキャッシュに入った時点で返る
	SyncBatch                   // BatchN 件ごとに fsync
	SyncEvery                   // 1 件ごとに fsync
	SyncFull                    // 1 件ごとに f.Sync() (macOS では F_FULLFSYNC。Linux では fsync)
)

type Options struct {
	MemLimit  int // メモリテーブルが、このバイト数を超えたら SSTable に書き出す
	CompactAt int // SSTable がこの数に達したらコンパクションする (0 なら、しない)
	Sync      SyncPolicy
	BatchN    int
	UseBloom  bool
}

type Stats struct {
	UserBytes, WALBytes, SSTBytes int64 // 利用者が書いたバイト数、WAL に書いた量、SSTable に書いた量
	BlockReads, BloomSkips        int64 // SSTable のブロックを読んだ回数、ブルームフィルタで読まずに済んだ回数
	Flushes, Compactions          int
	Recovered                     int  // 起動時に、WAL から再生したレコード数
	TornTail                      bool // 起動時に、WAL の末尾の壊れたレコードを見つけたか
}

const (
	tombstone  = 0xFFFFFFFF // 値の長さがこれなら、削除の印
	blockEvery = 16         // スパースインデックス: 16 レコードごとに、キーとファイル位置を覚える
)

type entry struct {
	val []byte
	del bool
}

type DB struct {
	dir       string
	opt       Options
	wal       *os.File
	walFd     int
	sinceSync int
	mem       map[string]entry
	memBytes  int
	tables    []*sstable // 古い → 新しい
	nextSeq   int
	Stats     Stats
}

// ---- レコードの符号化 ----

func appendRecord(buf []byte, k string, e entry) []byte {
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:], uint32(len(k)))
	vl := uint32(len(e.val))
	if e.del {
		vl = tombstone
	}
	binary.LittleEndian.PutUint32(hdr[4:], vl)
	buf = append(buf, hdr[:]...)
	buf = append(buf, k...)
	if !e.del {
		buf = append(buf, e.val...)
	}
	return buf
}

// data の先頭から順にレコードを読み、fn に渡す。
// 読み終えた位置と、途中で壊れた/切れたレコードを見つけたか (torn) を返す。
func parseRecords(data []byte, withCRC bool, fn func(k string, e entry, off int64)) (int, bool) {
	pos := 0
	for pos < len(data) {
		start := pos
		var crc uint32
		if withCRC {
			if len(data)-pos < 4 {
				return start, true
			}
			crc = binary.LittleEndian.Uint32(data[pos:])
			pos += 4
		}
		if len(data)-pos < 8 {
			return start, true
		}
		kl := int(binary.LittleEndian.Uint32(data[pos:]))
		vl := binary.LittleEndian.Uint32(data[pos+4:])
		bodyStart := pos
		pos += 8
		del := vl == tombstone
		n := kl
		if !del {
			n += int(vl)
		}
		if n < 0 || len(data)-pos < n {
			return start, true
		}
		key := string(data[pos : pos+kl])
		var val []byte
		if !del {
			val = append([]byte(nil), data[pos+kl:pos+n]...)
		}
		pos += n
		if withCRC && crc32.ChecksumIEEE(data[bodyStart:pos]) != crc {
			return start, true
		}
		fn(key, entry{val, del}, int64(start))
	}
	return pos, false
}

// ---- ファイルの安全な書き出し (第5回の本文: 一時ファイル → fsync → rename → ディレクトリを fsync) ----

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// ---- ブルームフィルタ ----

type bloom struct {
	bits []uint64
	m    uint64
	k    int
}

func newBloom(n int) *bloom {
	m := uint64(n * 10) // 1 キーあたり 10 ビット、ハッシュ 7 個 → 偽陽性率は約 1%
	if m < 64 {
		m = 64
	}
	return &bloom{bits: make([]uint64, (m+63)/64), m: m, k: 7}
}

func hash2(s string) (uint64, uint64) {
	h := fnv.New64a()
	h.Write([]byte(s))
	a := h.Sum64()
	return a, (a>>33 | a<<31) | 1
}

func (b *bloom) add(k string) {
	a, c := hash2(k)
	for i := 0; i < b.k; i++ {
		idx := (a + uint64(i)*c) % b.m
		b.bits[idx/64] |= 1 << (idx % 64)
	}
}

func (b *bloom) mayContain(k string) bool {
	a, c := hash2(k)
	for i := 0; i < b.k; i++ {
		idx := (a + uint64(i)*c) % b.m
		if b.bits[idx/64]&(1<<(idx%64)) == 0 {
			return false // 確実に無い
		}
	}
	return true // あるかもしれない
}

// ---- SSTable ----

type sstable struct {
	path       string
	size       int64
	sparseKeys []string
	sparseOff  []int64
	bloom      *bloom
	f          *os.File
}

// 簡単のため、開くときにファイル全体を走査して、スパースインデックスとブルームフィルタをメモリに作る。
// (実際のシステムは、これらをファイルの末尾に保存しておき、全体は読まない。)
func openSSTable(path string) (*sstable, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	t := &sstable{path: path, size: int64(len(data))}
	var keys []string
	parseRecords(data, false, func(k string, e entry, off int64) {
		if len(keys)%blockEvery == 0 {
			t.sparseKeys = append(t.sparseKeys, k)
			t.sparseOff = append(t.sparseOff, off)
		}
		keys = append(keys, k)
	})
	t.bloom = newBloom(len(keys))
	for _, k := range keys {
		t.bloom.add(k)
	}
	t.f, err = os.Open(path)
	return t, err
}

func (t *sstable) get(db *DB, k string) (entry, bool) {
	if db.opt.UseBloom && !t.bloom.mayContain(k) {
		db.Stats.BloomSkips++
		return entry{}, false
	}
	idx := sort.SearchStrings(t.sparseKeys, k) // k 以上の最初の位置
	if idx == len(t.sparseKeys) || t.sparseKeys[idx] != k {
		idx--
	}
	if idx < 0 {
		return entry{}, false // どのブロックの先頭キーよりも小さい
	}
	start := t.sparseOff[idx]
	end := t.size
	if idx+1 < len(t.sparseOff) {
		end = t.sparseOff[idx+1]
	}
	buf := make([]byte, end-start)
	if _, err := t.f.ReadAt(buf, start); err != nil {
		return entry{}, false
	}
	db.Stats.BlockReads++
	var res entry
	found := false
	parseRecords(buf, false, func(kk string, e entry, _ int64) {
		if kk == k {
			res, found = e, true
		}
	})
	return res, found
}

// ---- DB ----

func Open(dir string, opt Options) (*DB, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if opt.BatchN == 0 {
		opt.BatchN = 100
	}
	db := &DB{dir: dir, opt: opt, mem: map[string]entry{}}
	files, _ := filepath.Glob(filepath.Join(dir, "*.sst"))
	sort.Strings(files)
	for _, f := range files {
		t, err := openSSTable(f)
		if err != nil {
			return nil, err
		}
		db.tables = append(db.tables, t)
		var seq int
		fmt.Sscanf(filepath.Base(f), "%d.sst", &seq)
		if seq >= db.nextSeq {
			db.nextSeq = seq + 1
		}
	}
	// WAL を再生する。末尾が壊れていたら (書き込みの途中で止まった)、そこで打ち切って、ファイルも切り詰める。
	walPath := filepath.Join(dir, "wal.log")
	if data, err := os.ReadFile(walPath); err == nil {
		good, torn := parseRecords(data, true, func(k string, e entry, _ int64) {
			db.mem[k] = e
			db.memBytes += len(k) + len(e.val) + 16
			db.Stats.Recovered++
		})
		db.Stats.TornTail = torn
		if torn {
			if err := os.Truncate(walPath, int64(good)); err != nil {
				return nil, err
			}
		}
	}
	return db, db.openWAL()
}

func (db *DB) openWAL() error {
	f, err := os.OpenFile(filepath.Join(db.dir, "wal.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	db.wal, db.walFd, db.sinceSync = f, int(f.Fd()), 0
	return nil
}

func (db *DB) Put(k string, v []byte) error { return db.write(k, entry{val: v}) }
func (db *DB) Delete(k string) error        { return db.write(k, entry{del: true}) }

func (db *DB) write(k string, e entry) error {
	body := appendRecord(nil, k, e)
	rec := make([]byte, 4, 4+len(body))
	binary.LittleEndian.PutUint32(rec, crc32.ChecksumIEEE(body))
	rec = append(rec, body...)
	if _, err := db.wal.Write(rec); err != nil {
		return err
	}
	db.Stats.WALBytes += int64(len(rec))
	db.Stats.UserBytes += int64(len(k) + len(e.val))
	switch db.opt.Sync {
	case SyncEvery:
		if err := syscall.Fsync(db.walFd); err != nil {
			return err
		}
	case SyncFull:
		if err := db.wal.Sync(); err != nil {
			return err
		}
	case SyncBatch:
		db.sinceSync++
		if db.sinceSync >= db.opt.BatchN {
			if err := syscall.Fsync(db.walFd); err != nil {
				return err
			}
			db.sinceSync = 0
		}
	}
	db.mem[k] = e
	db.memBytes += len(k) + len(e.val) + 16
	if db.memBytes >= db.opt.MemLimit {
		return db.flush()
	}
	return nil
}

func (db *DB) Get(k string) ([]byte, bool) {
	if e, ok := db.mem[k]; ok {
		return e.val, !e.del
	}
	for i := len(db.tables) - 1; i >= 0; i-- { // 新しいものから
		if e, ok := db.tables[i].get(db, k); ok {
			return e.val, !e.del
		}
	}
	return nil, false
}

func sortedKeys(m map[string]entry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (db *DB) newTablePath() string {
	p := filepath.Join(db.dir, fmt.Sprintf("%06d.sst", db.nextSeq))
	db.nextSeq++
	return p
}

// メモリテーブルを SSTable として書き出し、WAL を作り直す。
func (db *DB) flush() error {
	if len(db.mem) == 0 {
		return nil
	}
	var buf []byte
	for _, k := range sortedKeys(db.mem) {
		buf = appendRecord(buf, k, db.mem[k])
	}
	path := db.newTablePath()
	if err := writeFileAtomic(path, buf); err != nil {
		return err
	}
	db.Stats.SSTBytes += int64(len(buf))
	db.Stats.Flushes++
	t, err := openSSTable(path)
	if err != nil {
		return err
	}
	db.tables = append(db.tables, t)
	db.mem, db.memBytes = map[string]entry{}, 0

	// SSTable が永続化された「後」に、WAL を捨てる (逆順だと、間でクラッシュしたときに、書き込みが失われる)
	db.wal.Close()
	os.Remove(filepath.Join(db.dir, "wal.log"))
	if err := db.openWAL(); err != nil {
		return err
	}
	if db.opt.CompactAt > 0 && len(db.tables) >= db.opt.CompactAt {
		return db.compact()
	}
	return nil
}

// 全 SSTable を1つにマージする。新しい版が勝ち、削除済みのキーは捨てる (これより古いデータは無いので)。
func (db *DB) compact() error {
	merged := map[string]entry{}
	for _, t := range db.tables { // 古い → 新しい の順に上書き
		data, err := os.ReadFile(t.path)
		if err != nil {
			return err
		}
		parseRecords(data, false, func(k string, e entry, _ int64) { merged[k] = e })
	}
	var live []string
	for k, e := range merged {
		if !e.del {
			live = append(live, k)
		}
	}
	sort.Strings(live)
	old := db.tables
	db.tables = nil
	if len(live) > 0 {
		var buf []byte
		for _, k := range live {
			buf = appendRecord(buf, k, merged[k])
		}
		path := db.newTablePath()
		if err := writeFileAtomic(path, buf); err != nil {
			return err
		}
		db.Stats.SSTBytes += int64(len(buf))
		t, err := openSSTable(path)
		if err != nil {
			return err
		}
		db.tables = []*sstable{t}
	}
	for _, t := range old { // 新しいテーブルが永続化された後に、古いものを消す
		t.f.Close()
		os.Remove(t.path)
	}
	db.Stats.Compactions++
	return nil
}

// 後片付けをせずに閉じる (クラッシュの模擬。メモリテーブルは WAL にしか残っていない)。
func (db *DB) Close() {
	db.wal.Close()
	for _, t := range db.tables {
		t.f.Close()
	}
}
