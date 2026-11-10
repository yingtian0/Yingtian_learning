// ex1: システムコールのコストと、「1回で運ぶ量」の効果
//
// A. 1回あたりのコスト
//      関数呼び出し (基準) / getppid / clock_gettime / read(/dev/zero, 1B) / write(/dev/null, 1B) / pread(1B)
//    clock_gettime は、多くの OS でカーネルに入らずに答える (Linux の vDSO、macOS の commpage)。
// B. read() に渡すバッファの大きさを変えて、ページキャッシュ上のファイルを読む
//      1バイトずつ読めば、システムコールの回数がそのまま性能を決める。
//      大きく読めば、1回のコストが、運ぶ量で薄まる。
//
// build: cc -O2 -Wall -Wextra -o ex1 ex1_syscall_cost.c
// run:   ./ex1
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

static double now_ns(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec * 1e9 + ts.tv_nsec;
}

__attribute__((noinline)) static long plain_call(long x) { return x + 1; }

#define N 2000000

int main(void) {
    char c;
    volatile long sink = 0;
    int zero = open("/dev/zero", O_RDONLY), null = open("/dev/null", O_WRONLY);

    // 一時ファイル (256 MiB)。書いた直後なので、ページキャッシュに載っている。
    char path[512];
    snprintf(path, sizeof path, "%s/ex1_XXXXXX", getenv("TMPDIR") ? getenv("TMPDIR") : "/tmp");
    int fd = mkstemp(path);
    if (fd < 0) { perror("mkstemp"); return 1; }
    unlink(path);
    const size_t fsize = 256u << 20;
    char *chunk = malloc(1 << 20);
    for (size_t i = 0; i < (1 << 20); i++) chunk[i] = (char)(i * 31);
    for (size_t off = 0; off < fsize; off += (1 << 20))
        if (write(fd, chunk, 1 << 20) != (1 << 20)) { perror("write"); return 1; }

    printf("A. 1回あたりのコスト (%d 回の平均)\n", N);
    double t0 = now_ns();
    for (long i = 0; i < N; i++) sink += plain_call(i);
    printf("  %-34s %8.1f ns\n", "関数呼び出し (基準)", (now_ns() - t0) / N);

    t0 = now_ns();
    for (long i = 0; i < N; i++) sink += getppid();
    printf("  %-34s %8.1f ns\n", "getppid()", (now_ns() - t0) / N);

    t0 = now_ns();
    for (long i = 0; i < N; i++) { struct timespec ts; clock_gettime(CLOCK_MONOTONIC, &ts); sink += ts.tv_nsec; }
    printf("  %-34s %8.1f ns   (カーネルに入らない実装が多い)\n", "clock_gettime()", (now_ns() - t0) / N);

    t0 = now_ns();
    for (long i = 0; i < N; i++) sink += read(zero, &c, 1);
    printf("  %-34s %8.1f ns\n", "read(/dev/zero, 1 バイト)", (now_ns() - t0) / N);

    t0 = now_ns();
    for (long i = 0; i < N; i++) sink += write(null, &c, 1);
    printf("  %-34s %8.1f ns\n", "write(/dev/null, 1 バイト)", (now_ns() - t0) / N);

    t0 = now_ns();
    for (long i = 0; i < N; i++) sink += pread(fd, &c, 1, (off_t)(i % 4096));
    printf("  %-34s %8.1f ns   (ページキャッシュ上のファイル)\n", "pread(ファイル, 1 バイト)", (now_ns() - t0) / N);

    printf("\nB. バッファの大きさと、読み込み速度 (ページキャッシュ上の 256 MiB のファイル)\n");
    printf("  %10s %12s %12s %14s\n", "バッファ", "読んだ量", "速度", "1回あたり");
    size_t sizes[] = {1, 16, 256, 4096, 65536, 1u << 20};
    char *buf = malloc(1 << 20);
    for (size_t k = 0; k < sizeof sizes / sizeof sizes[0]; k++) {
        size_t bs = sizes[k];
        size_t total = bs * 3000000ull;  // 小さいバッファは、回数を抑えるため、全体を読まない
        if (total > fsize) total = fsize;
        size_t calls = total / bs;
        t0 = now_ns();
        for (size_t i = 0; i < calls; i++) sink += pread(fd, buf, bs, (off_t)(i * bs % fsize));
        double el = now_ns() - t0;
        printf("  %8zu B %9.1f MB %9.1f MB/s %11.1f ns\n", bs, calls * bs / 1e6, calls * bs / 1e6 / (el / 1e9), el / calls);
    }
    return 0;
}
