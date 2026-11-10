// ex2: 「書いた」と「永続化した」の差 — write、fsync、F_FULLFSYNC のコスト
//
// 4KB を追記して、次の方式で、1回あたりの時間を測る。
//   A. write のみ              OS のページキャッシュに入った時点で返る。電源が落ちれば消える
//   B. write + fsync           ファイルの内容を、ストレージへ書き出すよう OS に要求する
//   C. write + F_FULLFSYNC     (macOS) ドライブ内の書き込みキャッシュまで、強制的に流す
//      write + fdatasync       (Linux) メタデータの一部を省いた fsync
//   D. 100000 回 write → 最後に 1 回 fsync   まとめて同期する (group commit の考え方)
//
// macOS の fsync() は、ドライブのキャッシュまでは流さない (man 2 fsync に記載)。
// 本当に永続化するには F_FULLFSYNC が必要で、その分、遅い。
//
// build: cc -O2 -Wall -Wextra -o ex2 ex2_fsync_cost.c
// run:   ./ex2           (TMPDIR のあるディスク上で測る)
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

static double now_us(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec * 1e6 + ts.tv_nsec * 1e-3;
}

static int cmp(const void *a, const void *b) {
    double x = *(const double *)a, y = *(const double *)b;
    return (x > y) - (x < y);
}

typedef enum { SYNC_NONE, SYNC_FSYNC, SYNC_FULL } mode_t_;

static void do_sync(int fd, mode_t_ m) {
    if (m == SYNC_FSYNC) {
        if (fsync(fd) != 0) perror("fsync");
    } else if (m == SYNC_FULL) {
#ifdef F_FULLFSYNC
        if (fcntl(fd, F_FULLFSYNC) != 0) perror("F_FULLFSYNC");
#else
        if (fdatasync(fd) != 0) perror("fdatasync");
#endif
    }
}

static void run(const char *label, mode_t_ m, int n, const char *dir) {
    char path[512];
    snprintf(path, sizeof path, "%s/ex2_%d.dat", dir, getpid());
    int fd = open(path, O_WRONLY | O_CREAT | O_TRUNC, 0644);
    if (fd < 0) { perror("open"); exit(1); }
    char *buf = malloc(4096);
    memset(buf, 'x', 4096);
    double *lat = malloc(sizeof(double) * n);

    double t0 = now_us();
    for (int i = 0; i < n; i++) {
        double s = now_us();
        if (write(fd, buf, 4096) != 4096) { perror("write"); exit(1); }
        do_sync(fd, m);
        lat[i] = now_us() - s;
    }
    double total = now_us() - t0;
    qsort(lat, n, sizeof(double), cmp);
    printf("%-34s n=%-7d 平均 %9.1f µs   p50 %9.1f   p99 %9.1f   最大 %10.1f   %9.0f 回/秒\n", label, n,
           total / n, lat[n / 2], lat[(int)(n * 0.99)], lat[n - 1], n / (total / 1e6));
    close(fd);
    unlink(path);
    free(buf);
    free(lat);
}

static void run_batched(int n, const char *dir) {
    char path[512];
    snprintf(path, sizeof path, "%s/ex2_%d.dat", dir, getpid());
    int fd = open(path, O_WRONLY | O_CREAT | O_TRUNC, 0644);
    char *buf = malloc(4096);
    memset(buf, 'x', 4096);
    double t0 = now_us();
    for (int i = 0; i < n; i++)
        if (write(fd, buf, 4096) != 4096) { perror("write"); exit(1); }
    double t_write = now_us() - t0;
    double t1 = now_us();
    do_sync(fd, SYNC_FULL);
    double t_sync = now_us() - t1;
    printf("%-34s n=%-7d 書き込み %9.1f µs/回、最後の同期 1 回 %9.1f ms (全体で %.1f µs/回)\n",
           "D. まとめて書いて、最後に1回だけ同期", n, t_write / n, t_sync / 1e3, (t_write + t_sync) / n);
    close(fd);
    unlink(path);
    free(buf);
}

int main(void) {
    const char *dir = getenv("TMPDIR") ? getenv("TMPDIR") : "/tmp";
    printf("測定場所: %s\n\n", dir);
    run("A. write のみ", SYNC_NONE, 100000, dir);
    run("B. write + fsync", SYNC_FSYNC, 1000, dir);
#ifdef F_FULLFSYNC
    run("C. write + F_FULLFSYNC", SYNC_FULL, 300, dir);
#else
    run("C. write + fdatasync", SYNC_FULL, 1000, dir);
#endif
    run_batched(100000, dir);
    return 0;
}
