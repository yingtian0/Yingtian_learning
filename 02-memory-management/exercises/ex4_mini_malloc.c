// ex4: 自作 malloc (first-fit + 分割 + 結合)
//
// mmap で確保した 64MiB の「アリーナ」を、ブロックの双方向リストで管理する。
//   - 各ブロックの先頭にヘッダ (サイズ、前後のブロック、空きフラグ) を置く。
//   - alloc: 先頭から探して最初に入る空きブロックを使う。余りが大きければ分割。
//   - free : 空きにして、前後が空きなら結合 (coalesce)。
// 仕組みが見えることを優先した実装で、速度は実用の malloc に遠く及ばない。
// マルチスレッドには対応していない。
//
// build: cc -O2 -o ex4 ex4_mini_malloc.c
// run:   ./ex4
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <time.h>

#ifndef MAP_ANON
#define MAP_ANON MAP_ANONYMOUS
#endif

#define ALIGN 16

typedef struct blk {
    size_t size;            // ペイロードのバイト数
    struct blk *prev, *next;  // アドレス順のリスト
    int free;
} __attribute__((aligned(ALIGN))) blk;  // sizeof = 32、ペイロードも 16B 境界に載る

static blk *head;
static void *arena;
static size_t arena_size;

// 新しいアリーナを用意する。以前のアリーナがあれば捨てる (中のポインタは無効になる)。
static void mm_init(size_t size) {
    if (arena) munmap(arena, arena_size);
    void *a = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANON, -1, 0);
    if (a == MAP_FAILED) { perror("mmap"); exit(1); }
    arena = a;
    arena_size = size;
    head = a;
    head->size = size - sizeof(blk);
    head->prev = head->next = NULL;
    head->free = 1;
}

static void *mm_alloc(size_t n) {
    if (n == 0) return NULL;
    n = (n + ALIGN - 1) & ~(size_t)(ALIGN - 1);
    for (blk *b = head; b; b = b->next) {
        if (!b->free || b->size < n) continue;
        if (b->size >= n + sizeof(blk) + ALIGN) {  // 余りが十分あれば分割
            blk *r = (blk *)((uint8_t *)(b + 1) + n);
            r->size = b->size - n - sizeof(blk);
            r->free = 1;
            r->prev = b;
            r->next = b->next;
            if (b->next) b->next->prev = r;
            b->next = r;
            b->size = n;
        }
        b->free = 0;
        return b + 1;
    }
    return NULL;  // 入る空きがない
}

static void mm_free(void *p) {
    if (!p) return;
    blk *b = (blk *)p - 1;
    b->free = 1;
    if (b->next && b->next->free) {  // 後ろと結合
        b->size += sizeof(blk) + b->next->size;
        b->next = b->next->next;
        if (b->next) b->next->prev = b;
    }
    if (b->prev && b->prev->free) {  // 前と結合
        b->prev->size += sizeof(blk) + b->size;
        b->prev->next = b->next;
        if (b->next) b->next->prev = b->prev;
    }
}

typedef struct { size_t used, free_total, free_largest, free_blocks, used_blocks; } stats;

static stats get_stats(void) {
    stats s = {0};
    for (blk *b = head; b; b = b->next) {
        if (b->free) {
            s.free_total += b->size;
            s.free_blocks++;
            if (b->size > s.free_largest) s.free_largest = b->size;
        } else {
            s.used += b->size;
            s.used_blocks++;
        }
    }
    return s;
}

static void print_stats(const char *label) {
    stats s = get_stats();
    double frag = s.free_total ? 1.0 - (double)s.free_largest / s.free_total : 0;
    printf("%-26s 使用中 %7zu B (%zu個)  空き合計 %9zu B  最大の空き %9zu B  空き塊 %zu個  外部断片化 %.1f%%\n",
           label, s.used, s.used_blocks, s.free_total, s.free_largest, s.free_blocks, frag * 100);
}

static double now_s(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}

// --- テスト1: 正しさ (パターンを書いて、最後まで壊れていないか確認) ---
#define NLIVE 1000
static void test_correctness(void) {
    static void *ptr[NLIVE];
    static size_t sz[NLIVE];
    srand(1);
    for (int i = 0; i < NLIVE; i++) { ptr[i] = NULL; sz[i] = 0; }
    for (int step = 0; step < 200000; step++) {
        int i = rand() % NLIVE;
        if (ptr[i]) {
            unsigned char want = (unsigned char)(i & 0xff);
            for (size_t k = 0; k < sz[i]; k++)
                if (((unsigned char *)ptr[i])[k] != want) { printf("破損を検出 (i=%d)\n", i); exit(1); }
            mm_free(ptr[i]);
            ptr[i] = NULL;
        } else {
            sz[i] = 1 + (size_t)rand() % 2000;
            ptr[i] = mm_alloc(sz[i]);
            if (!ptr[i]) { printf("確保失敗\n"); exit(1); }
            if ((uintptr_t)ptr[i] % ALIGN) { printf("アラインメント違反\n"); exit(1); }
            memset(ptr[i], i & 0xff, sz[i]);
        }
    }
    for (int i = 0; i < NLIVE; i++) mm_free(ptr[i]);
    print_stats("[テスト1 全部解放後]");
    stats s = get_stats();
    puts(s.free_blocks == 1 ? "  → 空きが1つに結合された (結合が正しく動いている)"
                            : "  → 空きが複数に分かれたまま (結合の不具合)");
}

// --- テスト2: 外部断片化 ---
// 小さいアリーナを 64B と 1024B の交互の確保で埋め尽くし、64B の方だけ解放する。
// 空きの合計は大きいのに、連続した空きは 64B しかないので、4096B が確保できなくなる。
static void test_fragmentation(void) {
    mm_init(4u << 20);
    size_t cap = (4u << 20) / 1000;
    void **small = malloc(cap * sizeof(void *)), **big = malloc(cap * sizeof(void *));
    size_t n = 0;
    while (n < cap) {
        small[n] = mm_alloc(64);
        if (!small[n]) break;
        big[n] = mm_alloc(1024);
        if (!big[n]) { mm_free(small[n]); break; }
        n++;
    }
    print_stats("[テスト2 アリーナを満杯に]");
    for (size_t i = 0; i < n; i++) mm_free(small[i]);
    print_stats("[テスト2 64B の方を解放]");
    stats s = get_stats();
    void *x = mm_alloc(4096);
    printf("  → 空きの合計は %zu B あるのに、4096B の確保は%s\n", s.free_total, x ? "成功した" : "失敗した");
    for (size_t i = 0; i < n; i++) mm_free(big[i]);
    print_stats("[テスト2 全部解放]");
    free(small); free(big);
}

// --- テスト3: 速度比較 ---
static void bench(const char *name, void *(*al)(size_t), void (*fr)(void *)) {
    enum { LIVE = 2000, OPS = 400000 };
    static void *ptr[LIVE];
    for (int i = 0; i < LIVE; i++) ptr[i] = NULL;
    srand(2);
    double t0 = now_s();
    for (int k = 0; k < OPS; k++) {
        int i = rand() % LIVE;
        if (ptr[i]) { fr(ptr[i]); ptr[i] = NULL; }
        else ptr[i] = al(16 + (size_t)rand() % 512);
    }
    double t = now_s() - t0;
    for (int i = 0; i < LIVE; i++) if (ptr[i]) fr(ptr[i]);
    printf("  %-14s %.1f ns/op\n", name, t / OPS * 1e9);
}

int main(void) {
    mm_init(64u << 20);
    print_stats("[初期]");
    test_correctness();
    test_fragmentation();
    mm_init(64u << 20);
    printf("\n速度比較 (ランダムな確保/解放):\n");
    bench("自作 mm_alloc", mm_alloc, mm_free);
    bench("システム malloc", malloc, free);
    return 0;
}
