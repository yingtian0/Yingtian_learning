// 【Linux 専用・この Mac では未実行】名前空間 (namespace) を、自分で作る
// clone(CLONE_NEWPID | CLONE_NEWUTS | CLONE_NEWNS | CLONE_NEWUSER ...) で、子を新しい名前空間に入れる。
// 子の中では、getpid() が 1 になり、hostname を変えても、親 (ホスト) に影響しない。
// ビルドと実行 (Linux): gcc -o ns_demo ns_demo.c && ./ns_demo   (非特権の user namespace が有効なこと)
#define _GNU_SOURCE
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/wait.h>
#include <unistd.h>

static char stack[1024 * 1024];

static int child(void *arg) {
    (void)arg;
    printf("子: getpid() = %d (新しい PID 名前空間では 1)\n", getpid());
    sethostname("in-container", 12);
    char buf[64];
    gethostname(buf, sizeof buf);
    printf("子: hostname = %s\n", buf);
    return 0;
}

int main(void) {
    char before[64];
    gethostname(before, sizeof before);
    int flags = CLONE_NEWUSER | CLONE_NEWPID | CLONE_NEWUTS | CLONE_NEWNS | SIGCHLD;
    pid_t p = clone(child, stack + sizeof stack, flags, NULL);
    if (p < 0) { perror("clone"); return 1; }
    printf("親: 子の PID (ホストから見た) = %d\n", p);
    waitpid(p, NULL, 0);
    char after[64];
    gethostname(after, sizeof after);
    printf("親: hostname = %s (変わっていない: %s)\n", after, strcmp(before, after) == 0 ? "はい" : "いいえ");
    return 0;
}
