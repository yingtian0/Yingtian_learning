// ex5: メモリ順序のリトマステスト (Store Buffering, "SB")
//
//   初期値 X = Y = 0
//   Thread0:  X = 1;  r0 = Y;
//   Thread1:  Y = 1;  r1 = X;
//
// 逐次一貫性 (SC) なら (r0, r1) = (0, 0) は絶対に起きない。
// しかし store buffer を持つ x86-TSO や、さらに弱い ARM では起こりうる。
//
// build (relaxed):  cc -O2 -pthread -o ex5_relaxed ex5_litmus_store_buffer.c
// build (seq_cst):  cc -O2 -pthread -DSEQ_CST -o ex5_sc ex5_litmus_store_buffer.c
// run: ./ex5_relaxed ; ./ex5_sc
#include <pthread.h>
#include <stdatomic.h>
#include <stdio.h>

#ifdef SEQ_CST
#define MO memory_order_seq_cst
#define MO_NAME "seq_cst"
#else
#define MO memory_order_relaxed
#define MO_NAME "relaxed"
#endif

#define N 500000

static _Atomic int X, Y;
static _Atomic int start_flag, finished;
static int r0_arr, r1_arr;

static void *t0(void *arg) {
    (void)arg;
    for (int i = 1; i <= N; i++) {
        while (atomic_load_explicit(&start_flag, memory_order_acquire) != i) {}
        atomic_store_explicit(&X, 1, MO);
        r0_arr = atomic_load_explicit(&Y, MO);
        atomic_fetch_add_explicit(&finished, 1, memory_order_acq_rel);
    }
    return NULL;
}
static void *t1(void *arg) {
    (void)arg;
    for (int i = 1; i <= N; i++) {
        while (atomic_load_explicit(&start_flag, memory_order_acquire) != i) {}
        atomic_store_explicit(&Y, 1, MO);
        r1_arr = atomic_load_explicit(&X, MO);
        atomic_fetch_add_explicit(&finished, 1, memory_order_acq_rel);
    }
    return NULL;
}

int main(void) {
    pthread_t a, b;
    pthread_create(&a, NULL, t0, NULL);
    pthread_create(&b, NULL, t1, NULL);

    long both_zero = 0, one_one = 0, mixed = 0;
    for (int i = 1; i <= N; i++) {
        atomic_store_explicit(&X, 0, memory_order_relaxed);
        atomic_store_explicit(&Y, 0, memory_order_relaxed);
        atomic_store_explicit(&finished, 0, memory_order_relaxed);
        atomic_store_explicit(&start_flag, i, memory_order_release);  // 発射
        while (atomic_load_explicit(&finished, memory_order_acquire) != 2) {}
        if (r0_arr == 0 && r1_arr == 0) both_zero++;
        else if (r0_arr == 1 && r1_arr == 1) one_one++;
        else mixed++;
    }
    pthread_join(a, NULL);
    pthread_join(b, NULL);

    printf("mode=%s  trials=%d\n", MO_NAME, N);
    printf("(0,0) : %ld   <-- SC では不可能な結果\n", both_zero);
    printf("(1,1) : %ld\n", one_one);
    printf("(0,1)/(1,0): %ld\n", mixed);
    return 0;
}
