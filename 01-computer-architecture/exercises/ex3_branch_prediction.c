// ex3: 分岐予測の影響 (ソート済み vs ランダム)
//
// 同じ配列・同じ処理でも、要素がソート済みなら分岐が予測しやすく速い。
// 注意: 最適化が強いとコンパイラが分岐をなくす (cmov/csel / SIMD化) ため差が消える。
//       本コードは空の asm で分岐除去を防いでいる。asm 行を消して再計測し、
//       差が縮む/消えることも確認すること (objdump -d / otool -tV で csel を探す)。
//
// build: cc -O1 -o ex3 ex3_branch_prediction.c
// run:   ./ex3
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

#define N 32768
#define REPEAT 20000

static double now_s(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}
static int cmp(const void *a, const void *b) {
    return *(const int *)a - *(const int *)b;
}

static double bench(const int *data, long long *out) {
    long long sum = 0;
    double t0 = now_s();
    for (int r = 0; r < REPEAT; r++)
        for (int i = 0; i < N; i++)
            if (data[i] >= 128) {  // ← 予測対象の分岐
                sum += data[i];
                // 空のasmを置いてコンパイラによる分岐除去(cmov/csel化)を防ぐ
                __asm__ volatile("" ::: "memory");
            }
    *out = sum;
    return now_s() - t0;
}

int main(void) {
    static int data[N];
    srand(1);
    for (int i = 0; i < N; i++) data[i] = rand() % 256;

    long long s1, s2;
    double t_rand = bench(data, &s1);
    qsort(data, N, sizeof(int), cmp);
    double t_sorted = bench(data, &s2);

    printf("random: %.3f s (sum=%lld)\n", t_rand, s1);
    printf("sorted: %.3f s (sum=%lld)\n", t_sorted, s2);
    printf("ratio : %.2fx\n", t_rand / t_sorted);
    return 0;
}
