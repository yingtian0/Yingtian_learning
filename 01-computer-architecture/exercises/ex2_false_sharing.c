// ex2: false sharing の計測
//
// 各スレッドは「自分専用」のカウンタだけを更新する。論理的には共有なし。
// しかしカウンタが同じキャッシュラインに載っていると、MESI の無効化合戦
// (cache line ping-pong) で劇的に遅くなる。
//
// build: cc -O2 -pthread -o ex2 ex2_false_sharing.c
// run:   ./ex2
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <time.h>

#define NTHREADS 4
#define ITERS 100000000UL
// Apple Silicon は 128B、多くの x86 は 64B。安全側で 128 にしておく。
#define PAD 128

typedef struct { volatile uint64_t v; } packed_t;                 // 隣接 (同一ライン)
typedef struct { volatile uint64_t v; char pad[PAD - 8]; } padded_t;  // 分離

static packed_t packed[NTHREADS];
static padded_t padded[NTHREADS];

static double now_s(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}

static void *work_packed(void *a) {
    long id = (long)a;
    for (uint64_t i = 0; i < ITERS; i++) packed[id].v++;
    return NULL;
}
static void *work_padded(void *a) {
    long id = (long)a;
    for (uint64_t i = 0; i < ITERS; i++) padded[id].v++;
    return NULL;
}

static double run(void *(*fn)(void *)) {
    pthread_t th[NTHREADS];
    double t0 = now_s();
    for (long i = 0; i < NTHREADS; i++) pthread_create(&th[i], NULL, fn, (void *)i);
    for (int i = 0; i < NTHREADS; i++) pthread_join(th[i], NULL);
    return now_s() - t0;
}

int main(void) {
    printf("threads=%d iters/thread=%lu\n", NTHREADS, ITERS);
    printf("packed (false sharing): %.3f s\n", run(work_packed));
    printf("padded (no sharing)   : %.3f s\n", run(work_padded));
    return 0;
}
