// ex3: ゾンビ、孤児、シグナルの性質
//
//   ./ex3 zombie   子が終了したのに、親が wait していない状態 (ゾンビ) を ps で見る
//   ./ex3 orphan   親が先に死んだとき、子の親が誰になるかを見る
//   ./ex3 signals  シグナルの3つの性質を確かめる
//                    (1) SIGKILL はハンドラを設定できない
//                    (2) 同じシグナルは、保留中に何回送っても1回にまとまる
//                    (3) SIGCHLD ハンドラで子を回収する (waitpid を WNOHANG で繰り返す)
//
// build: cc -O1 -Wall -Wextra -o ex3 ex3_zombie_orphan_signal.c
// run:   ./ex3 zombie ; ./ex3 orphan ; ./ex3 signals
#include <errno.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/wait.h>
#include <unistd.h>

static void show_ps(pid_t pid) {
    char cmd[128];
    snprintf(cmd, sizeof cmd, "ps -o pid,ppid,stat,comm -p %d", (int)pid);
    fflush(stdout);
    if (system(cmd) != 0) printf("(pid %d はもう存在しない)\n", (int)pid);
}

static void demo_zombie(void) {
    pid_t pid = fork();
    if (pid == 0) _exit(7);
    sleep(1);  // 子はすでに終了している。親は、まだ wait していない。
    printf("--- 子が終了し、親が wait する前 (STAT が Z ならゾンビ) ---\n");
    show_ps(pid);
    int status;
    waitpid(pid, &status, 0);
    printf("--- waitpid の後 (終了コード = %d) ---\n", WEXITSTATUS(status));
    show_ps(pid);
}

static void demo_orphan(void) {
    printf("最初のプロセス(親) pid=%d\n", (int)getpid());
    fflush(stdout);
    pid_t child = fork();
    if (child == 0) {
        pid_t grand = fork();
        if (grand == 0) {
            printf("孫: 起動直後         ppid=%d\n", (int)getppid());
            fflush(stdout);
            sleep(2);  // この間に、孫の親(子)が終了する
            printf("孫: 親が終了した後   ppid=%d  (1 = init / launchd に引き取られた)\n", (int)getppid());
            fflush(stdout);  // _exit はバッファを書き出さないので、明示的に出す
            _exit(0);
        }
        printf("子 pid=%d: 孫 pid=%d を作って、すぐ終了する\n", (int)getpid(), (int)grand);
        fflush(stdout);
        _exit(0);
    }
    waitpid(child, NULL, 0);
    sleep(3);  // 孫の出力を待つ
}

static volatile sig_atomic_t usr1_count = 0, term_seen = 0, reaped = 0;

static void on_usr1(int s) { (void)s; usr1_count++; }
static void on_term(int s) { (void)s; term_seen = 1; }
static void on_chld(int s) {
    (void)s;
    int saved = errno;  // ハンドラは errno を壊さないようにする
    while (waitpid(-1, NULL, WNOHANG) > 0) reaped++;
    errno = saved;
}

static void demo_signals(void) {
    // (1) SIGKILL
    struct sigaction sa;
    memset(&sa, 0, sizeof sa);
    sa.sa_handler = on_term;
    if (sigaction(SIGKILL, &sa, NULL) != 0)
        printf("(1) SIGKILL にハンドラを設定: 失敗 (%s) → 捕捉も無視もできない\n", strerror(errno));
    else
        printf("(1) SIGKILL にハンドラを設定できてしまった (想定外)\n");

    // SIGTERM は捕捉できる → 後片付けをしてから終了する、という設計ができる
    sigaction(SIGTERM, &sa, NULL);
    kill(getpid(), SIGTERM);
    printf("    SIGTERM は捕捉できる: ハンドラが呼ばれた = %d\n", (int)term_seen);

    // (2) 保留中のシグナルは1つにまとまる
    memset(&sa, 0, sizeof sa);
    sa.sa_handler = on_usr1;
    sigaction(SIGUSR1, &sa, NULL);
    sigset_t block, old;
    sigemptyset(&block);
    sigaddset(&block, SIGUSR1);
    sigprocmask(SIG_BLOCK, &block, &old);
    for (int i = 0; i < 5; i++) kill(getpid(), SIGUSR1);
    sigprocmask(SIG_SETMASK, &old, NULL);  // ここで保留中のシグナルが配送される
    printf("(2) SIGUSR1 を 5 回送った → ハンドラの呼び出し回数 = %d (通常のシグナルは数えられない)\n",
           (int)usr1_count);

    // (3) SIGCHLD で子を回収する
    memset(&sa, 0, sizeof sa);
    sa.sa_handler = on_chld;
    sa.sa_flags = SA_RESTART | SA_NOCLDSTOP;
    sigaction(SIGCHLD, &sa, NULL);
    for (int i = 0; i < 3; i++) {
        if (fork() == 0) { usleep(100000 * (i + 1)); _exit(0); }
    }
    while (reaped < 3) pause();  // シグナルが来るまで眠る
    printf("(3) SIGCHLD ハンドラで回収した子の数 = %d (ゾンビは残らない)\n", (int)reaped);
}

int main(int argc, char **argv) {
    if (argc < 2) { fprintf(stderr, "使い方: %s zombie|orphan|signals\n", argv[0]); return 1; }
    if (strcmp(argv[1], "zombie") == 0) demo_zombie();
    else if (strcmp(argv[1], "orphan") == 0) demo_orphan();
    else if (strcmp(argv[1], "signals") == 0) demo_signals();
    else { fprintf(stderr, "不明なモード: %s\n", argv[1]); return 1; }
    return 0;
}
