// ex2: システムコールとコンテキストスイッチのコストを測る
//
// 3つを測って比べる。
//   A. システムコール 1 回 (getppid)   ← カーネルに入って戻るだけ。プロセスの切り替えなし
//   B. パイプのピンポン (2つのスレッド) ← 同じアドレス空間の中での切り替え
//   C. パイプのピンポン (2つのプロセス) ← 別のアドレス空間への切り替え
// B と C は、同じ手順 (write → read で相手を起こして待つ) なので、差がアドレス空間の切り替えの影響を表す。
// 1往復あたり、スレッドの切り替え(またはプロセスの切り替え) が 2 回起きる。
//
// 注意: macOS ではコアへの固定ができない。2つの実行主体が別々のコアで動くと、
//       「同じコア上で切り替える」ときとは別の経路 (他のコアを起こす) になる。
//       Linux では `taskset -c 0 ./ex2` で1コアに固定すると、純粋な切り替えが測れる。
//
// build: cc -O2 -pthread -o ex2 ex2_context_switch.c
// run:   ./ex2
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/resource.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

#define NSYSCALL 2000000
#define NROUND 200000

static double now_ns(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec * 1e9 + ts.tv_nsec;
}

static long voluntary_switches(void) {
    struct rusage ru;
    getrusage(RUSAGE_SELF, &ru);
    return ru.ru_nvcsw;
}

// ---- B, C 共通: 相手が書くのを待って、すぐ書き返す ----
static int p2c[2], c2p[2];

static void echo_loop(void) {
    char c;
    for (int i = 0; i < NROUND; i++) {
        if (read(p2c[0], &c, 1) != 1) break;
        if (write(c2p[1], &c, 1) != 1) break;
    }
}

static void *thread_main(void *arg) {
    (void)arg;
    echo_loop();
    return NULL;
}

static double ping_loop(void) {
    char c = 'x';
    double t0 = now_ns();
    for (int i = 0; i < NROUND; i++) {
        if (write(p2c[1], &c, 1) != 1) exit(1);
        if (read(c2p[0], &c, 1) != 1) exit(1);
    }
    return (now_ns() - t0) / NROUND;
}

int main(void) {
    // A. システムコール
    double t0 = now_ns();
    for (int i = 0; i < NSYSCALL; i++) getppid();
    double sys_ns = (now_ns() - t0) / NSYSCALL;
    printf("A. システムコール (getppid)        : %7.0f ns/回\n", sys_ns);

    // B. スレッド間のピンポン
    if (pipe(p2c) || pipe(c2p)) return 1;
    pthread_t th;
    pthread_create(&th, NULL, thread_main, NULL);
    long v0 = voluntary_switches();
    double rt_thread = ping_loop();
    long v1 = voluntary_switches();
    pthread_join(th, NULL);
    printf("B. スレッド間ピンポン (1往復)       : %7.0f ns   (切り替え1回あたり約 %.0f ns, 自発的切替 %.2f回/往復 ※2スレッド合計)\n",
           rt_thread, rt_thread / 2, (double)(v1 - v0) / NROUND);
    close(p2c[0]); close(p2c[1]); close(c2p[0]); close(c2p[1]);

    // C. プロセス間のピンポン
    if (pipe(p2c) || pipe(c2p)) return 1;
    pid_t pid = fork();
    if (pid == 0) { echo_loop(); _exit(0); }
    v0 = voluntary_switches();
    double rt_proc = ping_loop();
    v1 = voluntary_switches();
    waitpid(pid, NULL, 0);
    printf("C. プロセス間ピンポン (1往復)       : %7.0f ns   (切り替え1回あたり約 %.0f ns, 自発的切替 %.2f回/往復 ※親のみ)\n",
           rt_proc, rt_proc / 2, (double)(v1 - v0) / NROUND);

    printf("\n1往復には write と read が各2回 (計4回のシステムコール) と、2回の切り替えが含まれる。\n");
    printf("プロセス / スレッド = %.2f 倍\n", rt_proc / rt_thread);
    return 0;
}
