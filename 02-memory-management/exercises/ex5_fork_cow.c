// ex5: fork と Copy-on-Write (COW)
//
// fork() は親のメモリ全体をコピーするように見えるが、実際には
// ページテーブルを複製し、ページは親子で共有する。どちらかが書いた瞬間に、
// そのページだけがコピーされる。ここでは次の3つを測る。
//   1. fork() 自体の時間 (常駐メモリが大きいほど、ページテーブルのコピーで長くなる)
//   2. 子が読むだけの間のコスト
//   3. 子が書いたときのコスト (ページごとにフォルト + コピー)
// また、子が書いても親のデータは変わらないことを確認する。
// 注意: 「読むだけ」でフォルトが起きるかは OS による。macOS では子が最初に読むときも
//       フォルトが起き、Linux ではページテーブルが複製済みなので通常は起きない。
//
// build: cc -O1 -o ex5 ex5_fork_cow.c
// run:   ./ex5
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/mman.h>
#include <sys/resource.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

#ifndef MAP_ANON
#define MAP_ANON MAP_ANONYMOUS
#endif

static double now_s(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}
static long minor_faults(void) {
    struct rusage ru;
    getrusage(RUSAGE_SELF, &ru);
    return ru.ru_minflt;
}

int main(void) {
    const size_t page = (size_t)sysconf(_SC_PAGESIZE);

    printf("--- fork() 自体の時間と常駐メモリの関係 ---\n");
    printf("%10s %12s\n", "resident", "fork time");
    for (size_t mb = 0; mb <= 512; mb = mb ? mb * 4 : 16) {
        uint8_t *p = NULL;
        if (mb) {
            p = mmap(NULL, mb << 20, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANON, -1, 0);
            for (size_t i = 0; i < (mb << 20); i += page) p[i] = 1;   // 全ページを常駐させる
        }
        fflush(stdout);  // 子が未出力のバッファを重ねて出力しないように
        double t0 = now_s();
        pid_t pid = fork();
        if (pid == 0) _exit(0);
        double t = now_s() - t0;
        waitpid(pid, NULL, 0);
        printf("%7zu MB %9.3f ms\n", mb, t * 1e3);
        if (p) munmap(p, mb << 20);
    }

    printf("\n--- 子が読む / 書くときのコスト (256 MB) ---\n");
    const size_t total = 256ul << 20;
    const size_t npages = total / page;
    uint8_t *p = mmap(NULL, total, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANON, -1, 0);
    for (size_t i = 0; i < npages; i++) p[i * page] = 1;

    int pipefd[2];
    if (pipe(pipefd) != 0) return 1;
    fflush(stdout);
    pid_t pid = fork();
    if (pid == 0) {
        close(pipefd[0]);
        long f0 = minor_faults();
        double t0 = now_s();
        unsigned sum = 0;
        for (size_t i = 0; i < npages; i++) sum += p[i * page];   // 読むだけ
        double t_read = now_s() - t0;
        long f_read = minor_faults() - f0;

        f0 = minor_faults();
        t0 = now_s();
        for (size_t i = 0; i < npages; i++) p[i * page] = 2;      // 書く → COW
        double t_write = now_s() - t0;
        long f_write = minor_faults() - f0;

        printf("子: 読むだけ  : %8.2f ms, faults +%ld (sum=%u)\n", t_read * 1e3, f_read, sum);
        printf("子: 書く(COW) : %8.2f ms, faults +%ld  → 1ページあたり %.0f ns\n",
               t_write * 1e3, f_write, t_write / npages * 1e9);
        fflush(stdout);
        char ok = 1;
        if (write(pipefd[1], &ok, 1) != 1) _exit(1);
        _exit(0);
    }
    close(pipefd[1]);
    char c;
    if (read(pipefd[0], &c, 1) != 1) return 1;   // 子が書き終わるのを待つ
    waitpid(pid, NULL, 0);

    int intact = 1;
    for (size_t i = 0; i < npages; i++) if (p[i * page] != 1) { intact = 0; break; }
    printf("親: 子が書いた後でも親のデータは元のまま: %s\n", intact ? "はい (値は全て 1)" : "いいえ");
    return 0;
}
