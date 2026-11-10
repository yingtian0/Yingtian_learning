// ex2: ロックの実装を比べる (スピンロック、TTAS、チケットロック、pthread mutex)
//
// 複数のスレッドが、決まった時間 (0.5 秒) のあいだ、
//   ロックを取る → 共有カウンタ (ふつうの long) を +1 → ロックを放す
// を繰り返す。見るのは次の3つ。
//   (a) 全体の処理回数 (Mops/s)                    ← 競合でどれだけ遅くなるか
//   (b) 公平さ = 最も少ないスレッド ÷ 最も多いスレッド  ← 飢えが起きていないか
//   (c) 正しさ = 共有カウンタ == 各スレッドの回数の合計    ← ロックが排他できているか
// スレッド数をコア数より多くすると、ロックを持ったスレッドが OS に止められて、
// 他のスレッドが空回りする (スピンロックの弱点)。
//
// build: cc -O2 -pthread -o ex2 ex2_locks.c
// run:   ./ex2
#include <pthread.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <unistd.h>

#if defined(__aarch64__)
#define cpu_relax() __asm__ volatile("yield")
#elif defined(__x86_64__)
#define cpu_relax() __asm__ volatile("pause")
#else
#define cpu_relax() ((void)0)
#endif

#define DURATION_NS 500000000L
#define MAXT 32

// ---- ロックの実装 ----
typedef struct {
    _Alignas(128) _Atomic int flag;                      // スピンロック / TTAS 用
    _Alignas(128) _Atomic unsigned next_ticket;          // チケットロック用
    _Alignas(128) _Atomic unsigned now_serving;
    _Alignas(128) pthread_mutex_t mutex;
} locks_t;

static locks_t L = { .mutex = PTHREAD_MUTEX_INITIALIZER };

typedef struct {
    const char *name;
    unsigned (*lock)(void);   // チケットロックは、放すときに番号が要るので戻り値で渡す
    void (*unlock)(unsigned);
} lock_ops;

static unsigned tas_lock(void) {
    while (atomic_exchange_explicit(&L.flag, 1, memory_order_acquire)) cpu_relax();
    return 0;
}
static void tas_unlock(unsigned t) { (void)t; atomic_store_explicit(&L.flag, 0, memory_order_release); }

static unsigned ttas_lock(void) {
    for (;;) {
        while (atomic_load_explicit(&L.flag, memory_order_relaxed)) cpu_relax();  // 読むだけで待つ (書かない)
        if (!atomic_exchange_explicit(&L.flag, 1, memory_order_acquire)) return 0;
    }
}

static unsigned ticket_lock(void) {
    unsigned t = atomic_fetch_add_explicit(&L.next_ticket, 1, memory_order_relaxed);
    while (atomic_load_explicit(&L.now_serving, memory_order_acquire) != t) cpu_relax();
    return t;
}
static void ticket_unlock(unsigned t) { atomic_store_explicit(&L.now_serving, t + 1, memory_order_release); }

static unsigned mutex_lock(void) { pthread_mutex_lock(&L.mutex); return 0; }
static void mutex_unlock(unsigned t) { (void)t; pthread_mutex_unlock(&L.mutex); }

static const lock_ops OPS[] = {
    {"TAS スピンロック", tas_lock, tas_unlock},
    {"TTAS スピンロック", ttas_lock, tas_unlock},
    {"チケットロック", ticket_lock, ticket_unlock},
    {"pthread mutex", mutex_lock, mutex_unlock},
};

// ---- ベンチマーク ----
static _Alignas(128) long shared_counter;      // ロックで守られる
static _Alignas(128) _Atomic int stop;
typedef struct { _Alignas(128) long count; const lock_ops *ops; } worker_t;
static worker_t workers[MAXT];

static long now_ns(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec * 1000000000L + ts.tv_nsec;
}

static void *worker(void *arg) {
    worker_t *w = arg;
    long c = 0;
    while (!atomic_load_explicit(&stop, memory_order_relaxed)) {
        unsigned t = w->ops->lock();
        shared_counter++;
        w->ops->unlock(t);
        c++;
    }
    w->count = c;
    return NULL;
}

static void bench(const lock_ops *ops, int nthreads) {
    shared_counter = 0;
    atomic_store(&stop, 0);
    pthread_t th[MAXT];
    for (int i = 0; i < nthreads; i++) {
        workers[i].count = 0;
        workers[i].ops = ops;
        pthread_create(&th[i], NULL, worker, &workers[i]);
    }
    long t0 = now_ns();
    struct timespec d = { 0, DURATION_NS };
    nanosleep(&d, NULL);
    atomic_store(&stop, 1);
    for (int i = 0; i < nthreads; i++) pthread_join(th[i], NULL);
    double sec = (now_ns() - t0) / 1e9;

    long total = 0, mn = workers[0].count, mx = workers[0].count;
    for (int i = 0; i < nthreads; i++) {
        total += workers[i].count;
        if (workers[i].count < mn) mn = workers[i].count;
        if (workers[i].count > mx) mx = workers[i].count;
    }
    printf("  %-18s %2d スレッド %8.2f Mops/s   公平さ(最少/最多) %5.3f   %s\n", ops->name, nthreads,
           total / sec / 1e6, mx ? (double)mn / mx : 0, shared_counter == total ? "正しい" : "★排他できていない");
}

int main(void) {
    int ncpu = (int)sysconf(_SC_NPROCESSORS_ONLN);
    printf("論理CPU = %d、1回 %.1f 秒\n", ncpu, DURATION_NS / 1e9);
    int counts[] = {1, 2, 4, 8, ncpu * 2};
    for (size_t k = 0; k < sizeof counts / sizeof counts[0]; k++) {
        printf("\n[%d スレッド%s]\n", counts[k], counts[k] > ncpu ? " (論理CPU より多い)" : "");
        for (size_t i = 0; i < sizeof OPS / sizeof OPS[0]; i++) bench(&OPS[i], counts[k]);
    }
    return 0;
}
