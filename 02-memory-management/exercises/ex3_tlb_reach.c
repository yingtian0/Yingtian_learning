// ex3: TLB の届く範囲 (TLB reach) を測る
//
// 同じ個数のキャッシュラインを使う2通りの配置を比べる。
//   A. spread: 1ページに1ラインずつ。N ライン = N ページにまたがる。
//   B. packed: N ラインを連続して詰める。必要なページは N*64/ページサイズ 個だけ。
// 使うキャッシュラインの数は同じなので、速度差はおもにアドレス変換(TLB ミス)による。
// 毎回、直前の読み出し結果が次のアドレスになる (ポインタチェイス) ので、
// 先読みで隠せず、レイテンシがそのまま見える。
//
// build: cc -O2 -o ex3 ex3_tlb_reach.c
// run:   ./ex3
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/mman.h>
#include <time.h>
#include <unistd.h>

#ifndef MAP_ANON
#define MAP_ANON MAP_ANONYMOUS
#endif

#define LINE 64
#define STEPS 20000000UL

static double now_ns(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec * 1e9 + ts.tv_nsec;
}

// n 個の要素に対する巡回置換 (Sattolo)。サイクルが1本なので全要素を回る。
static void make_cycle(size_t *perm, size_t n) {
    for (size_t i = 0; i < n; i++) perm[i] = i;
    for (size_t i = n - 1; i > 0; i--) {
        size_t j = (size_t)rand() % i;
        size_t t = perm[i]; perm[i] = perm[j]; perm[j] = t;
    }
}

// addr[i] = i 番目のノードの置き場所。各ノードの先頭に「次のノードの置き場所」を書く。
static double chase(uint8_t **addr, size_t n) {
    size_t *perm = malloc(n * sizeof(size_t));
    make_cycle(perm, n);
    for (size_t i = 0; i < n; i++) *(uint8_t **)addr[i] = addr[perm[i]];
    free(perm);

    uint8_t *p = addr[0];
    for (size_t i = 0; i < n; i++) p = *(uint8_t **)p;  // ウォームアップ
    double t0 = now_ns();
    for (size_t i = 0; i < STEPS; i++) p = *(uint8_t **)p;
    double t1 = now_ns();
    uint8_t *volatile sink = p; (void)sink;  // ループが最適化で消えないようにする
    return (t1 - t0) / STEPS;
}

int main(void) {
    const size_t page = (size_t)sysconf(_SC_PAGESIZE);
    const size_t max_pages = 16384;
    srand(7);

    uint8_t *spread_mem = mmap(NULL, max_pages * page, PROT_READ | PROT_WRITE,
                               MAP_PRIVATE | MAP_ANON, -1, 0);
    uint8_t *packed_mem = mmap(NULL, max_pages * LINE, PROT_READ | PROT_WRITE,
                               MAP_PRIVATE | MAP_ANON, -1, 0);
    if (spread_mem == MAP_FAILED || packed_mem == MAP_FAILED) return 1;
    uint8_t **addr = malloc(max_pages * sizeof(uint8_t *));

    size_t lines_per_page = page / LINE;
    printf("page size = %zu bytes\n", page);
    printf("%8s %14s %14s %8s\n", "N", "spread(ns)", "packed(ns)", "ratio");
    for (size_t n = 8; n <= max_pages; n *= 2) {
        // A: i 番目のノードは i 番目のページに置く。ページ内の位置は少しずつずらし、
        //    キャッシュのセットが偏らないようにする。
        for (size_t i = 0; i < n; i++)
            addr[i] = spread_mem + i * page + (i % lines_per_page) * LINE;
        double a = chase(addr, n);

        // B: 同じ個数のノードを連続した領域に詰める。
        for (size_t i = 0; i < n; i++) addr[i] = packed_mem + i * LINE;
        double b = chase(addr, n);

        printf("%8zu %14.2f %14.2f %8.2f\n", n, a, b, a / b);
    }
    return 0;
}
