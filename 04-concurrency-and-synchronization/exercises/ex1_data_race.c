// ex1: データ競合 (lost update) と、それを防ぐ4つの方法
//
// 8 スレッドが、それぞれ 200 万回ずつカウンタを +1 する。正しい合計は 1600 万。
//   1. 保護なし (volatile の long)      → 更新が失われる。結果は毎回違う
//   2. pthread_mutex で保護
//   3. アトミック加算 (fetch_add)
//   4. スレッドごとのカウンタ (最後に合計) ← 共有をなくす
//
// `counter++` は、1つの操作に見えて、実際は「読む → 足す → 書く」の3手順である。
// 2つのスレッドが同時に読むと、片方の加算が上書きされて消える。
// (注意: 保護なしの版は C の規格ではデータ競合 = 未定義動作。ここでは動作を見るためだけに使う。)
//
// build: cc -O2 -pthread -o ex1 ex1_data_race.c
// run:   ./ex1
#include <pthread.h>
#include <stdatomic.h>
#include <stdio.h>
#include <time.h>

#define NT 8
#define N 2000000L

static volatile long plain_counter;
static long mutex_counter;
static pthread_mutex_t mu = PTHREAD_MUTEX_INITIALIZER;
static _Atomic long atomic_counter;
// キャッシュラインを分ける (第1回: false sharing)。volatile は、最適化でループが消えて計測が0になるのを防ぐため
typedef struct { volatile long v; char pad[120]; } shard_t;
static shard_t shards[NT];

static double now_s(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}

static void *w_plain(void *a) {
    (void)a;
    for (long i = 0; i < N; i++) plain_counter++;
    return NULL;
}
static void *w_mutex(void *a) {
    (void)a;
    for (long i = 0; i < N; i++) {
        pthread_mutex_lock(&mu);
        mutex_counter++;
        pthread_mutex_unlock(&mu);
    }
    return NULL;
}
static void *w_atomic(void *a) {
    (void)a;
    for (long i = 0; i < N; i++) atomic_fetch_add_explicit(&atomic_counter, 1, memory_order_relaxed);
    return NULL;
}
static void *w_shard(void *a) {
    long id = (long)a;
    for (long i = 0; i < N; i++) shards[id].v++;
    return NULL;
}

static double run(void *(*fn)(void *)) {
    pthread_t th[NT];
    double t0 = now_s();
    for (long i = 0; i < NT; i++) pthread_create(&th[i], NULL, fn, (void *)i);
    for (int i = 0; i < NT; i++) pthread_join(th[i], NULL);
    return now_s() - t0;
}

int main(void) {
    const long expect = NT * N;
    printf("スレッド数 %d、各 %ld 回、正しい合計 = %ld\n\n", NT, N, expect);
    printf("%-26s %12s %10s %12s\n", "方式", "結果", "失われた数", "時間");

    double t = run(w_plain);
    printf("%-26s %12ld %10ld %9.3f s\n", "1. 保護なし", (long)plain_counter, expect - plain_counter, t);

    t = run(w_mutex);
    printf("%-26s %12ld %10ld %9.3f s\n", "2. mutex", mutex_counter, expect - mutex_counter, t);

    t = run(w_atomic);
    printf("%-26s %12ld %10ld %9.3f s\n", "3. アトミック加算", (long)atomic_counter, expect - atomic_counter, t);

    t = run(w_shard);
    long sum = 0;
    for (int i = 0; i < NT; i++) sum += shards[i].v;
    printf("%-26s %12ld %10ld %9.3f s\n", "4. スレッドごとのカウンタ", sum, expect - sum, t);
    return 0;
}
