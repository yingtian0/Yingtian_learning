// ex4: read() と mmap() — ページキャッシュ上のファイルを読む
//
//   1. 順次: read() (64KiB ずつ) と mmap (全体を走査) で、全バイトの和を取る
//   2. ランダム 4KiB: pread() と、mmap したメモリへのアクセス
// ファイルは、ページキャッシュに載った状態の 512 MiB。
// mmap は、初めて触れるページでページフォルトが起きる (第2回)。2回目の走査では起きない。
// ページフォルトの回数も、getrusage で数える。
//
// build: cc -O2 -Wall -Wextra -o ex4 ex4_mmap_vs_read.c
// run:   ./ex4
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/resource.h>
#include <time.h>
#include <unistd.h>

#define FSIZE (512u << 20)

static double now_s(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}
static long faults(void) {
    struct rusage r;
    getrusage(RUSAGE_SELF, &r);
    return r.ru_minflt;
}

static uint64_t sum_words(const void *p, size_t n) {
    const uint64_t *w = p;
    uint64_t s = 0;
    for (size_t i = 0; i < n / 8; i++) s += w[i];
    return s;
}

int main(void) {
    char path[512];
    snprintf(path, sizeof path, "%s/ex4_XXXXXX", getenv("TMPDIR") ? getenv("TMPDIR") : "/tmp");
    int f = mkstemp(path);
    if (f < 0) { perror("mkstemp"); return 1; }
    unlink(path);
    uint64_t *chunk = malloc(1 << 20);
    for (size_t i = 0; i < (1 << 20) / 8; i++) chunk[i] = i * 2654435761u;
    for (size_t off = 0; off < FSIZE; off += (1 << 20))
        if (write(f, chunk, 1 << 20) != (1 << 20)) { perror("write"); return 1; }

    volatile uint64_t sink = 0;
    printf("ファイル %u MiB (ページキャッシュ上)\n\n1. 順次 (全バイトの和)\n", FSIZE >> 20);

    static char buf[64 << 10];
    long f0 = faults();
    double t0 = now_s();
    for (off_t off = 0; off < FSIZE; off += sizeof buf) {
        if (pread(f, buf, sizeof buf, off) != (ssize_t)sizeof buf) return 1;
        sink += sum_words(buf, sizeof buf);
    }
    double t = now_s() - t0;
    printf("  %-28s %7.0f MB/s  ページフォルト +%ld\n", "read (64 KiB ずつ)", FSIZE / 1e6 / t, faults() - f0);

    char *p = mmap(NULL, FSIZE, PROT_READ, MAP_SHARED, f, 0);
    if (p == MAP_FAILED) { perror("mmap"); return 1; }
    for (int pass = 1; pass <= 2; pass++) {
        f0 = faults();
        t0 = now_s();
        sink += sum_words(p, FSIZE);
        t = now_s() - t0;
        printf("  %-28s %7.0f MB/s  ページフォルト +%ld\n", pass == 1 ? "mmap (1回目の走査)" : "mmap (2回目の走査)", FSIZE / 1e6 / t, faults() - f0);
    }
    munmap(p, FSIZE);

    printf("\n2. ランダム 4 KiB (20万回)\n");
    enum { OPS = 200000 };
    uint64_t seed = 12345;
    f0 = faults();
    t0 = now_s();
    for (int i = 0; i < OPS; i++) {
        seed = seed * 6364136223846793005ULL + 1442695040888963407ULL;
        off_t off = (off_t)((seed >> 20) % (FSIZE / 4096)) * 4096;
        if (pread(f, buf, 4096, off) != 4096) return 1;
        sink += buf[0];
    }
    t = now_s() - t0;
    printf("  %-28s %7.0f ns/回  ページフォルト +%ld\n", "pread", t / OPS * 1e9, faults() - f0);

    p = mmap(NULL, FSIZE, PROT_READ, MAP_SHARED, f, 0);
    for (int pass = 1; pass <= 2; pass++) {
        seed = 12345;
        f0 = faults();
        t0 = now_s();
        for (int i = 0; i < OPS; i++) {
            seed = seed * 6364136223846793005ULL + 1442695040888963407ULL;
            off_t off = (off_t)((seed >> 20) % (FSIZE / 4096)) * 4096;
            memcpy(buf, p + off, 4096);
            sink += buf[0];
        }
        t = now_s() - t0;
        printf("  %-28s %7.0f ns/回  ページフォルト +%ld\n", pass == 1 ? "mmap (1回目: 初めて触れる)" : "mmap (2回目: 同じ場所)", t / OPS * 1e9, faults() - f0);
    }
    return 0;
}
