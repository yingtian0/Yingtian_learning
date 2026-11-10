// ex5: CDN のキャッシュを、シミュレーションと実測で調べる
//
//	A. アクセスの偏り (Zipf 分布) と、キャッシュの大きさ・方式 (LRU / FIFO / LFU) で、ヒット率がどう変わるか。
//	   物件の大きさが、裾の重い分布のとき、「物件単位のヒット率」と「バイト単位のヒット率」は、違う。
//	B. 階層: エッジ 8 台だけ / エッジ + 共有層 (shield) / 物件を、エッジに、ハッシュで振り分ける。オリジンへの要求は、どう減るか。
//	C. スタンピード (実測): 同じ物件に、200 個の要求が同時に来たとき、オリジンへの要求は何回か。
//	   まとめない / リクエストの合流 (coalescing) / 合流 + 古いものを返しながら更新 (stale-while-revalidate)
//
// A、B はシミュレーション。C は、ループバックの、実際の HTTP のオリジン (処理に 100 ms) を使う。
// 実行: go run ./ex5_cdn_cache
package main

import (
	"container/heap"
	"container/list"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ---------------- キャッシュの実装 ----------------
type cache interface {
	// 物件 k (大きさ size) への要求。ヒットなら true
	access(k int, size int64) bool
}

type lru struct {
	cap, used int64
	ll        *list.List
	m         map[int]*list.Element
}
type entry struct {
	k    int
	size int64
}

func newLRU(cap int64) *lru { return &lru{cap: cap, ll: list.New(), m: map[int]*list.Element{}} }
func (c *lru) access(k int, size int64) bool {
	if e, ok := c.m[k]; ok {
		c.ll.MoveToFront(e)
		return true
	}
	if size > c.cap {
		return false
	}
	for c.used+size > c.cap {
		b := c.ll.Back()
		en := b.Value.(*entry)
		c.used -= en.size
		delete(c.m, en.k)
		c.ll.Remove(b)
	}
	c.m[k] = c.ll.PushFront(&entry{k, size})
	c.used += size
	return false
}

type fifo struct {
	cap, used int64
	q         []entry
	in        map[int]bool
}

func newFIFO(cap int64) *fifo { return &fifo{cap: cap, in: map[int]bool{}} }
func (c *fifo) access(k int, size int64) bool {
	if c.in[k] {
		return true
	}
	if size > c.cap {
		return false
	}
	for c.used+size > c.cap {
		e := c.q[0]
		c.q = c.q[1:]
		c.used -= e.size
		delete(c.in, e.k)
	}
	c.q = append(c.q, entry{k, size})
	c.in[k] = true
	c.used += size
	return false
}

// LFU: アクセス回数が最も少ないものを追い出す (回数は、キャッシュに入っている間だけ数える)
type lfuItem struct {
	k     int
	size  int64
	count int
	idx   int
	seq   int
}
type lfuHeap []*lfuItem

func (h lfuHeap) Len() int { return len(h) }
func (h lfuHeap) Less(i, j int) bool {
	if h[i].count != h[j].count {
		return h[i].count < h[j].count
	}
	return h[i].seq < h[j].seq
}
func (h lfuHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i]; h[i].idx = i; h[j].idx = j }
func (h *lfuHeap) Push(x any)   { it := x.(*lfuItem); it.idx = len(*h); *h = append(*h, it) }
func (h *lfuHeap) Pop() any     { o := *h; n := len(o); it := o[n-1]; *h = o[:n-1]; return it }

type lfu struct {
	cap, used int64
	h         lfuHeap
	m         map[int]*lfuItem
	seq       int
}

func newLFU(cap int64) *lfu { return &lfu{cap: cap, m: map[int]*lfuItem{}} }
func (c *lfu) access(k int, size int64) bool {
	c.seq++
	if it, ok := c.m[k]; ok {
		it.count++
		it.seq = c.seq
		heap.Fix(&c.h, it.idx)
		return true
	}
	if size > c.cap {
		return false
	}
	for c.used+size > c.cap {
		it := heap.Pop(&c.h).(*lfuItem)
		c.used -= it.size
		delete(c.m, it.k)
	}
	it := &lfuItem{k: k, size: size, count: 1, seq: c.seq}
	heap.Push(&c.h, it)
	c.m[k] = it
	c.used += size
	return false
}

// ---------------- 要求の生成 ----------------
type zipf struct {
	cdf []float64
	p   []float64
}

func newZipf(n int, alpha float64) *zipf {
	z := &zipf{cdf: make([]float64, n), p: make([]float64, n)}
	var sum float64
	for i := 0; i < n; i++ {
		z.p[i] = 1 / math.Pow(float64(i+1), alpha)
		sum += z.p[i]
	}
	var acc float64
	for i := 0; i < n; i++ {
		z.p[i] /= sum
		acc += z.p[i]
		z.cdf[i] = acc
	}
	return z
}
func (z *zipf) sample(rng *rand.Rand) int {
	i := sort.SearchFloat64s(z.cdf, rng.Float64())
	if i >= len(z.cdf) {
		i = len(z.cdf) - 1
	}
	return i
}

const (
	catalog  = 100000
	requests = 1500000
)

func partA() {
	fmt.Printf("A. ヒット率 (物件 %d 個、要求 %d 回。物件はすべて同じ大きさ。キャッシュの大きさは、物件の個数で表す)\n", catalog, requests)
	fmt.Printf("  %-8s %-10s %8s %8s %8s   %s\n", "偏り α", "キャッシュ", "LRU", "FIFO", "LFU", "理論上の上限 (上位 N 個を、あらかじめ置いた場合)")
	for _, alpha := range []float64{0.8, 1.0, 1.2} {
		z := newZipf(catalog, alpha)
		for _, frac := range []float64{0.001, 0.01, 0.05, 0.10} {
			capN := int64(float64(catalog) * frac)
			caches := []cache{newLRU(capN), newFIFO(capN), newLFU(capN)}
			hits := make([]int, 3)
			rng := rand.New(rand.NewSource(1))
			for i := 0; i < requests; i++ {
				k := z.sample(rng)
				for j, c := range caches {
					if c.access(k, 1) && i >= requests/10 {
						hits[j]++
					}
				}
			}
			n := float64(requests - requests/10)
			var oracle float64
			for i := int64(0); i < capN; i++ {
				oracle += z.p[i]
			}
			fmt.Printf("  %-8.1f %-10s %7.1f%% %7.1f%% %7.1f%%   %.1f%%\n", alpha, fmt.Sprintf("%.1f%%", frac*100), float64(hits[0])/n*100, float64(hits[1])/n*100, float64(hits[2])/n*100, oracle*100)
		}
	}

	fmt.Println("\n  物件の大きさが、裾の重い分布のとき (対数正規、中央値 10 KB、σ=1.5)。α=1.0、LRU")
	z := newZipf(catalog, 1.0)
	rng := rand.New(rand.NewSource(2))
	sizes := make([]int64, catalog)
	var total int64
	for i := range sizes {
		sizes[i] = int64(10240 * math.Exp(1.5*rng.NormFloat64()))
		if sizes[i] < 100 {
			sizes[i] = 100
		}
		total += sizes[i]
	}
	fmt.Printf("  カタログ全体: %.1f GB (平均 %.0f KB)\n", float64(total)/1e9, float64(total)/catalog/1024)
	fmt.Printf("  %-12s %12s %14s\n", "キャッシュ", "物件のヒット率", "バイトのヒット率")
	for _, frac := range []float64{0.01, 0.05, 0.10} {
		c := newLRU(int64(float64(total) * frac))
		var hit, bhit, bytes int64
		n := 0
		r := rand.New(rand.NewSource(1))
		for i := 0; i < requests; i++ {
			k := z.sample(r)
			h := c.access(k, sizes[k])
			if i >= requests/10 {
				n++
				bytes += sizes[k]
				if h {
					hit++
					bhit += sizes[k]
				}
			}
		}
		fmt.Printf("  %-12s %11.1f%% %13.1f%%\n", fmt.Sprintf("全体の %.0f%%", frac*100), float64(hit)/float64(n)*100, float64(bhit)/float64(bytes)*100)
	}
}

func partB() {
	fmt.Println("\nB. 階層 (エッジ 8 台、それぞれ、カタログの 1% を置ける。α=1.0)")
	z := newZipf(catalog, 1.0)
	const edges = 8
	capEdge := int64(catalog / 100)
	type scenario struct {
		name   string
		shield int64 // 0 = なし
		hashed bool  // 物件を、エッジに、ハッシュで振り分ける
	}
	fmt.Printf("  %-52s %14s %16s\n", "構成", "エッジのヒット率", "オリジンへの要求の割合")
	for _, sc := range []scenario{
		{"エッジだけ (利用者は、どのエッジにも、ランダムに)", 0, false},
		{"エッジ + 共有層 (カタログの 5%)", int64(catalog / 20), false},
		{"物件を、ハッシュで、エッジに振り分ける (エッジだけ)", 0, true},
	} {
		es := make([]cache, edges)
		for i := range es {
			es[i] = newLRU(capEdge)
		}
		var shield cache
		if sc.shield > 0 {
			shield = newLRU(sc.shield)
		}
		rng := rand.New(rand.NewSource(1))
		var edgeHit, originReq, n int
		for i := 0; i < requests; i++ {
			k := z.sample(rng)
			e := rng.Intn(edges)
			if sc.hashed {
				e = k % edges
			}
			h := es[e].access(k, 1)
			miss := !h
			if miss && shield != nil {
				miss = !shield.access(k, 1)
			}
			if i >= requests/10 {
				n++
				if h {
					edgeHit++
				}
				if miss {
					originReq++
				}
			}
		}
		fmt.Printf("  %-52s %13.1f%% %15.1f%%\n", sc.name, float64(edgeHit)/float64(n)*100, float64(originReq)/float64(n)*100)
	}
	fmt.Println("  (1番目は、各エッジが、同じ人気の物件を、別々に覚える。共有層は、複数のエッジの失敗を、まとめて受ける。ハッシュ振り分けは、実質的に、エッジ全体の容量が 8 倍になる。)")
}

// ---------------- C: スタンピード ----------------
type origin struct {
	hits atomic.Int64
	ln   net.Listener
}

func startOrigin(latency time.Duration) *origin {
	o := &origin{}
	o.ln, _ = net.Listen("tcp", "127.0.0.1:0")
	go http.Serve(o.ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o.hits.Add(1)
		time.Sleep(latency)
		w.Write([]byte("コンテンツ"))
	}))
	return o
}

type item struct {
	val     string
	fetched time.Time
}

type inflight struct {
	done chan struct{}
	val  string
}

type edgeCache struct {
	mu       sync.Mutex
	items    map[string]item
	flights  map[string]*inflight
	ttl      time.Duration
	origin   string
	coalesce bool
	swr      bool
	client   *http.Client
}

func (c *edgeCache) fetch(key string) string {
	resp, err := c.client.Get("http://" + c.origin + "/" + key)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func (c *edgeCache) load(key string) string {
	if !c.coalesce {
		v := c.fetch(key)
		c.mu.Lock()
		c.items[key] = item{v, time.Now()}
		c.mu.Unlock()
		return v
	}
	c.mu.Lock()
	if f, ok := c.flights[key]; ok { // すでに、誰かが、取りに行っている → 待つ
		c.mu.Unlock()
		<-f.done
		return f.val
	}
	f := &inflight{done: make(chan struct{})}
	c.flights[key] = f
	c.mu.Unlock()
	f.val = c.fetch(key)
	c.mu.Lock()
	c.items[key] = item{f.val, time.Now()}
	delete(c.flights, key)
	c.mu.Unlock()
	close(f.done)
	return f.val
}

func (c *edgeCache) Get(key string) string {
	c.mu.Lock()
	it, ok := c.items[key]
	c.mu.Unlock()
	if ok && time.Since(it.fetched) < c.ttl {
		return it.val
	}
	if ok && c.swr { // 期限切れだが、古いものを返しつつ、裏で更新する
		go c.load(key)
		return it.val
	}
	return c.load(key)
}

func partC() {
	fmt.Println("\nC. スタンピード (実測): 同じ物件に、200 個の要求が、同時に来る。オリジンの処理は 100 ms")
	fmt.Printf("  %-52s %14s %12s %12s\n", "方式", "オリジンへの要求", "クライアントの平均", "最大")
	for _, cs := range []struct {
		name      string
		coalesce  bool
		swr       bool
		expiredIn bool // キャッシュに、期限切れの古いものがある状態から始める
	}{
		{"1. 冷えたキャッシュ。まとめない", false, false, false},
		{"2. 冷えたキャッシュ。リクエストを合流させる", true, false, false},
		{"3. 期限切れの古いものがある。合流させる", true, false, true},
		{"4. 期限切れの古いものがある。合流 + 古いものを返しながら更新", true, true, true},
	} {
		o := startOrigin(100 * time.Millisecond)
		c := &edgeCache{items: map[string]item{}, flights: map[string]*inflight{}, ttl: time.Second, origin: o.ln.Addr().String(),
			coalesce: cs.coalesce, swr: cs.swr, client: &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 256}}}
		if cs.expiredIn {
			c.items["x"] = item{"古いコンテンツ", time.Now().Add(-time.Hour)}
		}
		var wg sync.WaitGroup
		var mu sync.Mutex
		var sum, max time.Duration
		start := make(chan struct{})
		for i := 0; i < 200; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				t0 := time.Now()
				c.Get("x")
				d := time.Since(t0)
				mu.Lock()
				sum += d
				if d > max {
					max = d
				}
				mu.Unlock()
			}()
		}
		close(start)
		wg.Wait()
		time.Sleep(300 * time.Millisecond) // 裏の更新が終わるのを待つ
		fmt.Printf("  %-52s %14d %9.1f ms %9.1f ms\n", cs.name, o.hits.Load(), float64(sum.Microseconds())/1000/200, float64(max.Microseconds())/1000)
		o.ln.Close()
	}
}

func main() {
	partA()
	partB()
	partC()
}
