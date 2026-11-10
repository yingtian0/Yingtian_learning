// ex4: 空間局所性 — 行優先/列優先アクセスと、ストライド幅の影響
//
// C は行優先 (row-major)。同じ N×N 行列の総和でも、走査順で速度が大きく変わる。
//
// build: cc -O1 -o ex4 ex4_stride_matrix.c
// run:   ./ex4
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

#define N 4096  // 4096*4096*4B = 64MB (L2/L3 を超える)

static double now_s(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}

int main(void) {
    int *m = malloc((size_t)N * N * sizeof(int));
    if (!m) return 1;
    for (size_t i = 0; i < (size_t)N * N; i++) m[i] = 1;

    long long s = 0;
    double t0 = now_s();
    for (int i = 0; i < N; i++)
        for (int j = 0; j < N; j++) s += m[(size_t)i * N + j];  // 連続アクセス
    double t_row = now_s() - t0;

    long long s2 = 0;
    t0 = now_s();
    for (int j = 0; j < N; j++)
        for (int i = 0; i < N; i++) s2 += m[(size_t)i * N + j];  // N*4B 飛び
    double t_col = now_s() - t0;

    printf("row-major   : %.3f s (sum=%lld)\n", t_row, s);
    printf("column-major: %.3f s (sum=%lld)\n", t_col, s2);
    printf("ratio       : %.2fx\n", t_col / t_row);

    // ストライドを変えながら、同じ要素数(1<<24回)だけ読む
    size_t total = (size_t)N * N;
    printf("\n%8s %12s\n", "stride", "ns/access");
    for (size_t stride = 1; stride <= 1024; stride *= 2) {
        long long acc = 0;
        size_t count = 1 << 24;
        t0 = now_s();
        size_t idx = 0;
        for (size_t k = 0; k < count; k++) {
            acc += m[idx];
            idx += stride;
            if (idx >= total) idx -= total;
        }
        double t = now_s() - t0;
        volatile long long sink = acc; (void)sink;
        printf("%8zu %12.2f\n", stride, t * 1e9 / count);
    }
    free(m);
    return 0;
}
