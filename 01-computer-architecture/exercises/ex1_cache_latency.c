// ex1: ポインタチェイシングでキャッシュ階層のレイテンシを測る
//
// 各ノードを 64B (キャッシュライン1本) に1つ置き、Sattolo のシャッフルで
// 「全ノードを1周する単一サイクル」を作る。次のアドレスが直前のロード結果に
// 依存するため、CPUは先読み(OoO/プリフェッチ)で隠せず、純粋なレイテンシが出る。
//
// build: cc -O2 -o ex1 ex1_cache_latency.c
// run:   ./ex1
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

#define LINE 64
#define STEPS 20000000UL

static double now_ns(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec * 1e9 + ts.tv_nsec;
}

int main(void) {
    srand(12345);
    printf("%10s %12s\n", "size(KB)", "ns/access");
    for (size_t kb = 4; kb <= 131072; kb *= 2) {
        size_t nodes = kb * 1024 / LINE;
        size_t stride = LINE / sizeof(size_t);  // 1ノード = 8要素 = 64B
        size_t *buf = aligned_alloc(LINE, nodes * LINE);
        size_t *perm = malloc(nodes * sizeof(size_t));
        if (!buf || !perm) return 1;

        for (size_t i = 0; i < nodes; i++) perm[i] = i;
        // Sattolo: 長さ nodes の巡回置換 (サイクルがちょうど1本)
        for (size_t i = nodes - 1; i > 0; i--) {
            size_t j = (size_t)rand() % i;
            size_t t = perm[i]; perm[i] = perm[j]; perm[j] = t;
        }
        for (size_t i = 0; i < nodes; i++)
            buf[i * stride] = perm[i] * stride;  // next のインデックス

        size_t p = 0;
        for (size_t i = 0; i < nodes; i++) p = buf[p];  // ウォームアップ
        double t0 = now_ns();
        for (size_t i = 0; i < STEPS; i++) p = buf[p];
        double t1 = now_ns();

        volatile size_t sink = p; (void)sink;  // 最適化で消されないように
        printf("%10zu %12.2f\n", kb, (t1 - t0) / STEPS);
        free(buf); free(perm);
    }
    return 0;
}
