// ex5: nice 値は CPU の取り分をどれだけ変えるか
//
// 論理 CPU の数と同じ個数の「nice 0 のプロセス」と「nice 10 のプロセス」を同時に走らせる。
// CPU は奪い合いになる。各プロセスは、一定時間、CPU を使った計算をして、回数を親へ報告する。
// グループごとの合計から、CPU の取り分の比がわかる。
//
// 参考: Linux の CFS では、nice 0 の重みは 1024、nice 10 の重みは 110。
//       単純計算だと取り分は 90.3% 対 9.7%。実際の値は、OS とスケジューラで違う。
//
// build: cc -O1 -o ex5 ex5_nice_hogs.c
// run:   ./ex5 [秒数]
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/resource.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

typedef struct { int nice; uint64_t iters; } report_t;

static double now_s(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}

static void hog(int nice_val, double seconds, int out_fd) {
    setpriority(PRIO_PROCESS, 0, nice_val);
    volatile uint64_t acc = 1;
    uint64_t iters = 0;
    double t0 = now_s();
    while (now_s() - t0 < seconds) {
        uint64_t a = acc;
        for (int k = 0; k < 100000; k++) a = a * 6364136223846793005ULL + 1442695040888963407ULL;
        acc = a;
        iters += 100000;
    }
    report_t r = { nice_val, iters };
    if (write(out_fd, &r, sizeof r) != (ssize_t)sizeof r) _exit(1);
    _exit(0);
}

int main(int argc, char **argv) {
    double seconds = argc > 1 ? atof(argv[1]) : 4.0;
    int ncpu = (int)sysconf(_SC_NPROCESSORS_ONLN);
    int fds[2];
    if (pipe(fds) != 0) return 1;

    int total = ncpu * 2;
    for (int i = 0; i < total; i++) {
        if (fork() == 0) {
            close(fds[0]);
            hog(i % 2 == 0 ? 0 : 10, seconds, fds[1]);
        }
    }
    close(fds[1]);

    uint64_t sum[2] = {0, 0};
    for (int i = 0; i < total; i++) {
        report_t r;
        if (read(fds[0], &r, sizeof r) != (ssize_t)sizeof r) break;
        sum[r.nice == 0 ? 0 : 1] += r.iters;
    }
    while (wait(NULL) > 0) {}

    double all = (double)(sum[0] + sum[1]);
    printf("論理CPU = %d、各 nice のプロセス = %d 個、%.1f 秒\n", ncpu, ncpu, seconds);
    printf("nice  0 のグループ: %6.1f%%  (%.2e 回)\n", sum[0] / all * 100, (double)sum[0]);
    printf("nice 10 のグループ: %6.1f%%  (%.2e 回)\n", sum[1] / all * 100, (double)sum[1]);
    return 0;
}
