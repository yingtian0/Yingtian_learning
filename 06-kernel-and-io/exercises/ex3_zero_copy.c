// ex3: ファイルをソケットへ送る3つの方法 (read+write / mmap+write / sendfile)
//
//   A. read + write   カーネル → ユーザー空間のバッファ → カーネル、と2回コピーする
//   B. mmap + write   ファイルを mmap して、そのメモリを write に渡す (読み出しのコピーが無くなる)
//   C. sendfile       ユーザー空間を通さず、カーネルの中だけで送る
// 受信側は、別プロセスで、受け取って捨てるだけ。送信側の CPU 時間 (user / sys) と速度を比べる。
// ファイルは、ページキャッシュに載った状態の 512 MiB。ループバックの TCP で送る。
//
// 注意: sendfile の引数は OS で違う。Linux 版は書いてあるが、未確認。
//
// build: cc -O2 -Wall -Wextra -o ex3 ex3_zero_copy.c
// run:   ./ex3
#include <arpa/inet.h>
#include <fcntl.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/resource.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/uio.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>
#ifndef __APPLE__
#include <sys/sendfile.h>
#endif

#define FSIZE (512u << 20)

static pid_t g_receiver;  // 失敗したとき、受信側の子プロセスを残さないため

static double now_s(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}
static double tv_s(struct timeval t) { return t.tv_sec + t.tv_usec * 1e-6; }

static int send_read_write(int s, int f) {
    static char buf[64 << 10];
    lseek(f, 0, SEEK_SET);
    ssize_t n;
    while ((n = read(f, buf, sizeof buf)) > 0) {
        for (ssize_t off = 0; off < n;) {
            ssize_t w = write(s, buf + off, (size_t)(n - off));
            if (w <= 0) return -1;
            off += w;
        }
    }
    return 0;
}

static int send_mmap(int s, int f) {
    char *p = mmap(NULL, FSIZE, PROT_READ, MAP_SHARED, f, 0);
    if (p == MAP_FAILED) return -1;
    for (size_t off = 0; off < FSIZE;) {
        ssize_t w = write(s, p + off, FSIZE - off < (1u << 20) ? FSIZE - off : (1u << 20));
        if (w <= 0) { munmap(p, FSIZE); return -1; }
        off += (size_t)w;
    }
    munmap(p, FSIZE);
    return 0;
}

static int send_sendfile(int s, int f) {
#ifdef __APPLE__
    off_t off = 0;
    while (off < (off_t)FSIZE) {
        off_t len = 0;   // 0 = ファイルの終わりまで。戻り値として、実際に送れた量が入る
        int r = sendfile(f, s, off, &len, NULL, 0);
        off += len;
        if (r != 0 && len == 0) return -1;
    }
#else
    off_t off = 0;
    while (off < (off_t)FSIZE) {
        ssize_t w = sendfile(s, f, &off, FSIZE - (size_t)off);
        if (w <= 0) return -1;
    }
#endif
    return 0;
}

static void receiver(int lfd, int rounds) {
    static char buf[1 << 20];
    for (int r = 0; r < rounds; r++) {
        int c = accept(lfd, NULL, NULL);
        while (read(c, buf, sizeof buf) > 0) {}
        close(c);
    }
}

static void measure(const char *name, int (*fn)(int, int), int lfd_port, int f) {
    double best_mbs = 0, best_user = 0, best_sys = 0;
    for (int rep = 0; rep < 3; rep++) {
        int s = socket(AF_INET, SOCK_STREAM, 0);
        struct sockaddr_in a = { .sin_family = AF_INET, .sin_addr.s_addr = htonl(INADDR_LOOPBACK), .sin_port = htons((uint16_t)lfd_port) };
        if (connect(s, (struct sockaddr *)&a, sizeof a) != 0) { perror("connect"); kill(g_receiver, SIGKILL); exit(1); }
        struct rusage r0, r1;
        getrusage(RUSAGE_SELF, &r0);
        double t0 = now_s();
        if (fn(s, f) != 0) { perror(name); kill(g_receiver, SIGKILL); exit(1); }
        close(s);   // 受信側が読み終わる前に、送信側の時間を測り終える (送り切った時点)
        double el = now_s() - t0;
        getrusage(RUSAGE_SELF, &r1);
        double mbs = FSIZE / 1e6 / el;
        if (mbs > best_mbs) {
            best_mbs = mbs;
            best_user = tv_s(r1.ru_utime) - tv_s(r0.ru_utime);
            best_sys = tv_s(r1.ru_stime) - tv_s(r0.ru_stime);
        }
    }
    printf("  %-20s %8.0f MB/s   送信側の CPU: user %.3f 秒 + sys %.3f 秒 (3回のうち最速の回)\n", name, best_mbs, best_user, best_sys);
}

int main(void) {
    char path[512];
    snprintf(path, sizeof path, "%s/ex3_XXXXXX", getenv("TMPDIR") ? getenv("TMPDIR") : "/tmp");
    int f = mkstemp(path);
    if (f < 0) { perror("mkstemp"); return 1; }
    unlink(path);
    char *chunk = malloc(1 << 20);
    for (size_t i = 0; i < (1 << 20); i++) chunk[i] = (char)(i * 131);
    for (size_t off = 0; off < FSIZE; off += (1 << 20))
        if (write(f, chunk, 1 << 20) != (1 << 20)) { perror("write"); return 1; }

    int l = socket(AF_INET, SOCK_STREAM, 0);
    struct sockaddr_in a = { .sin_family = AF_INET, .sin_addr.s_addr = htonl(INADDR_LOOPBACK), .sin_port = 0 };
    if (bind(l, (struct sockaddr *)&a, sizeof a) != 0 || listen(l, 8) != 0) { perror("listen"); return 1; }
    socklen_t len = sizeof a;
    getsockname(l, (struct sockaddr *)&a, &len);
    int port = ntohs(a.sin_port);

    pid_t pid = fork();
    if (pid == 0) { receiver(l, 9); _exit(0); }
    g_receiver = pid;

    printf("ファイル %u MiB (ページキャッシュ上)、ループバック TCP\n\n", FSIZE >> 20);
    measure("A. read + write", send_read_write, port, f);
    measure("B. mmap + write", send_mmap, port, f);
    measure("C. sendfile", send_sendfile, port, f);
    waitpid(pid, NULL, 0);
    return 0;
}
