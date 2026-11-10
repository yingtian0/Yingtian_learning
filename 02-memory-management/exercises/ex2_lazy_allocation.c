// ex2: 遅延割り当て (demand paging) とページフォルトのコスト
//
// 1GiB を mmap しても、物理メモリはまだほとんど使われない。
// ページに初めて触れた瞬間にページフォルトが起き、OS が物理ページを割り当てる。
// ここでは「確保しただけ」「触った」「もう一度触った」で何が変わるかを測る。
// macOS (Apple Silicon) では、最初の数回のパスが遅く、徐々に速くなる。
// フォルト回数だけでは説明できない差なので、結果の読み方は講義本文を参照。
//
// build: cc -O1 -o ex2 ex2_lazy_allocation.c
// run:   ./ex2
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/mman.h>
#include <sys/resource.h>
#include <time.h>
#include <unistd.h>

#ifdef __APPLE__
#include <mach/mach.h>
#endif
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

// 現在の常駐メモリ (RSS) を MB で返す
static double rss_mb(void) {
#ifdef __APPLE__
    mach_task_basic_info_data_t info;
    mach_msg_type_number_t cnt = MACH_TASK_BASIC_INFO_COUNT;
    task_info(mach_task_self(), MACH_TASK_BASIC_INFO, (task_info_t)&info, &cnt);
    return info.resident_size / 1048576.0;
#else
    long pages = 0, rss = 0;
    FILE *f = fopen("/proc/self/statm", "r");
    if (!f) return -1;
    if (fscanf(f, "%ld %ld", &pages, &rss) != 2) rss = 0;
    fclose(f);
    return rss * (double)sysconf(_SC_PAGESIZE) / 1048576.0;
#endif
}

int main(void) {
    const size_t total = 1ul << 30;            // 1 GiB
    const size_t page = (size_t)sysconf(_SC_PAGESIZE);
    const size_t touch_bytes = 256ul << 20;    // このうち 256MiB だけ触る
    const size_t npages = touch_bytes / page;

    printf("page size = %zu bytes\n\n", page);
    printf("%-34s RSS=%8.1f MB  minor faults=%ld\n", "[start]", rss_mb(), minor_faults());

    long f0 = minor_faults();
    uint8_t *p = mmap(NULL, total, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANON, -1, 0);
    if (p == MAP_FAILED) { perror("mmap"); return 1; }
    printf("%-34s RSS=%8.1f MB  (+%ld faults)\n", "[after mmap 1 GiB]", rss_mb(), minor_faults() - f0);

    // 同じ256MiBに何回も触れて、1ページあたりの時間がどう変わるかを見る。
    for (int pass = 1; pass <= 4; pass++) {
        long before = minor_faults();
        double t0 = now_s();
        for (size_t i = 0; i < npages; i++) p[i * page] = (uint8_t)pass;
        double t = now_s() - t0;
        printf("[pass %d: 256 MiB に1ページ1バイト書く] RSS=%8.1f MB  (+%ld faults)  1ページあたり %6.0f ns\n",
               pass, rss_mb(), minor_faults() - before, t / npages * 1e9);
    }

    // 物理ページを OS に返す。Linux は madvise(DONTNEED) で即座に返る。
    // macOS では MADV_DONTNEED は助言にとどまり RSS が減らないので、同じ範囲を
    // 新しい匿名マップで置き換える。
#ifdef __linux__
    madvise(p, touch_bytes, MADV_DONTNEED);
#else
    mmap(p, touch_bytes, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANON | MAP_FIXED, -1, 0);
#endif
    printf("%-34s RSS=%8.1f MB\n", "[物理ページを返した直後]", rss_mb());
    long before = minor_faults();
    for (size_t i = 0; i < npages; i++) p[i * page] = 9;
    printf("%-34s RSS=%8.1f MB  (+%ld faults)  → 再び初回と同じ扱い\n", "[もう一度触る]", rss_mb(), minor_faults() - before);

    munmap(p, total);
    return 0;
}
