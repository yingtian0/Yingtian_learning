// ex1: プロセスのアドレス空間を観察する
//
// コード、定数、初期化済みデータ、未初期化データ、ヒープ、mmap 領域、スタックが
// それぞれ仮想アドレスのどのあたりに置かれるかを表示する。
// 2回続けて実行し、アドレスが毎回変わること (ASLR) も確認する。
//
// build: cc -O0 -o ex1 ex1_address_space.c
// run:   ./ex1 ; ./ex1
#include <stdio.h>
#include <stdlib.h>
#include <sys/mman.h>
#include <unistd.h>

#ifndef MAP_ANON
#define MAP_ANON MAP_ANONYMOUS
#endif

int g_init = 42;                 // .data
int g_bss;                       // .bss
const char g_rodata[] = "const"; // .rodata

static void some_function(void) {}

int main(void) {
    int local = 0;
    void *small = malloc(100);          // 小さい確保 → 通常はヒープ (brk 系/アリーナ)
    void *large = malloc(64u << 20);    // 大きい確保 → 通常は mmap で確保される
    void *mapped = mmap(NULL, 1u << 20, PROT_READ | PROT_WRITE,
                        MAP_PRIVATE | MAP_ANON, -1, 0);

    printf("page size            : %ld bytes\n", sysconf(_SC_PAGESIZE));
    printf("%-22s %p\n", "code (main)",        (void *)main);
    printf("%-22s %p\n", "code (static func)", (void *)some_function);
    printf("%-22s %p\n", "libc (printf)",      (void *)printf);
    printf("%-22s %p\n", "rodata",             (const void *)g_rodata);
    printf("%-22s %p\n", "data (initialized)", (void *)&g_init);
    printf("%-22s %p\n", "bss (uninitialized)",(void *)&g_bss);
    printf("%-22s %p\n", "heap (malloc 100B)", small);
    printf("%-22s %p\n", "malloc 64MB",        large);
    printf("%-22s %p\n", "mmap 1MB anonymous", mapped);
    printf("%-22s %p\n", "stack (local var)",  (void *)&local);

#ifdef __linux__
    printf("\n--- /proc/self/maps ---\n");
    char cmd[64];
    snprintf(cmd, sizeof cmd, "cat /proc/%d/maps", getpid());
    if (system(cmd) != 0) return 1;
#else
    printf("\n(macOS) 全体の対応表を見るには、別の端末で: vmmap <pid>\n");
    printf("pid=%d  → 5 秒待ちます。この間に vmmap %d を実行してみてください。\n",
           getpid(), getpid());
    fflush(stdout);
    sleep(5);
#endif
    return 0;
}
