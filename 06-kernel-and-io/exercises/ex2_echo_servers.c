// ex2: 3つの流儀のエコーサーバーを比べる (C10K 問題)
//
//   threads : 1 接続 = 1 スレッド。読み出しでブロックする、素直な書き方
//   poll    : 1 スレッドで、poll() に全接続を渡して、準備できたものを処理する
//   kqueue  : 1 スレッドで、kqueue (macOS) / epoll (Linux) に、接続を登録しておく
//
// 負荷の掛け方: クライアントは、N 本の接続を張るが、実際にやりとりするのは、そのうちの nactive 本だけ。
// 残りは、つないだまま黙っている (現実のサーバーの「大半の接続は暇」という状況)。
// 活発な接続は、1 バイトを送り、エコーが返ったら、すぐ次の 1 バイトを送る。
// 見るもの: 1秒あたりの往復回数 / サーバーのメモリ (RSS) とスレッド数 / 接続確立にかかった時間。
//
// 注意: Linux 用の epoll の部分は、書いてあるが、未確認 (この講義の環境は macOS)。
//
// build: cc -O2 -pthread -Wall -Wextra -o ex2 ex2_echo_servers.c
// run:   ./ex2
#include <arpa/inet.h>
#include <errno.h>
#include <fcntl.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <poll.h>
#include <pthread.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/resource.h>
#include <sys/socket.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>
#ifdef __APPLE__
#include <sys/event.h>
#else
#include <sys/epoll.h>
#endif

#define MAXC 20000

static double now_s(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}

static void raise_fd_limit(void) {
    struct rlimit rl;
    getrlimit(RLIMIT_NOFILE, &rl);
    rlim_t want = 60000;
    if (rl.rlim_max != RLIM_INFINITY && rl.rlim_max < want) want = rl.rlim_max;
    rl.rlim_cur = want;
    setrlimit(RLIMIT_NOFILE, &rl);
}

static void nodelay(int fd) {
    int one = 1;
    setsockopt(fd, IPPROTO_TCP, TCP_NODELAY, &one, sizeof one);
}

// ---------------- サーバー 1: スレッド ----------------
static void *conn_thread(void *arg) {
    int fd = (int)(intptr_t)arg;
    char buf[64];
    ssize_t n;
    while ((n = read(fd, buf, sizeof buf)) > 0)
        if (write(fd, buf, (size_t)n) != n) break;
    close(fd);
    return NULL;
}

static void serve_threads(int lfd) {
    pthread_attr_t a;
    pthread_attr_init(&a);
    pthread_attr_setdetachstate(&a, PTHREAD_CREATE_DETACHED);
    pthread_attr_setstacksize(&a, 64 * 1024);
    int warned = 0;
    for (;;) {
        int c = accept(lfd, NULL, NULL);
        if (c < 0) continue;
        nodelay(c);
        pthread_t t;
        if (pthread_create(&t, &a, conn_thread, (void *)(intptr_t)c) != 0) {
            if (!warned) { warned = 1; dprintf(2, "    ★ スレッドを作れなかった (OS のスレッド数の上限)。以降の接続は切る\n"); }
            close(c);
        }
    }
}

// ---------------- サーバー 2: poll ----------------
static void serve_poll(int lfd) {
    struct pollfd *fds = malloc(sizeof(struct pollfd) * (MAXC + 1));
    int n = 1;
    fds[0].fd = lfd; fds[0].events = POLLIN;
    for (;;) {
        int r = poll(fds, (nfds_t)n, -1);   // 毎回、全部の fd をカーネルに渡し、全部を調べてもらう → O(n)
        if (r < 0) continue;
        for (int i = n - 1; i >= 0 && r > 0; i--) {
            if (!fds[i].revents) continue;
            r--;
            if (i == 0) {
                int c = accept(lfd, NULL, NULL);
                if (c >= 0 && n < MAXC) { nodelay(c); fds[n].fd = c; fds[n].events = POLLIN; fds[n].revents = 0; n++; }
                else if (c >= 0) close(c);
            } else {
                char buf[64];
                ssize_t k = read(fds[i].fd, buf, sizeof buf);
                if (k <= 0) { close(fds[i].fd); fds[i] = fds[--n]; }
                else if (write(fds[i].fd, buf, (size_t)k) != k) { close(fds[i].fd); fds[i] = fds[--n]; }
            }
        }
    }
}

// ---------------- サーバー 3: kqueue / epoll ----------------
#ifdef __APPLE__
static void serve_event(int lfd) {
    int kq = kqueue();
    struct kevent ev;
    EV_SET(&ev, lfd, EVFILT_READ, EV_ADD, 0, 0, NULL);
    kevent(kq, &ev, 1, NULL, 0, NULL);
    struct kevent evs[256];
    for (;;) {
        int n = kevent(kq, NULL, 0, evs, 256, NULL);   // 準備ができたものだけが返る → O(準備ができた数)
        for (int i = 0; i < n; i++) {
            int fd = (int)evs[i].ident;
            if (fd == lfd) {
                for (intptr_t k = 0; k < evs[i].data; k++) {
                    int c = accept(lfd, NULL, NULL);
                    if (c < 0) break;
                    nodelay(c);
                    EV_SET(&ev, c, EVFILT_READ, EV_ADD, 0, 0, NULL);
                    kevent(kq, &ev, 1, NULL, 0, NULL);
                }
            } else {
                char buf[64];
                ssize_t k = read(fd, buf, sizeof buf);
                if (k <= 0) close(fd);                   // close すると、kqueue から自動で外れる
                else if (write(fd, buf, (size_t)k) != k) close(fd);
            }
        }
    }
}
#else
static void serve_event(int lfd) {   // Linux (未確認)
    fcntl(lfd, F_SETFL, fcntl(lfd, F_GETFL) | O_NONBLOCK);
    int ep = epoll_create1(0);
    struct epoll_event ev = { .events = EPOLLIN, .data.fd = lfd };
    epoll_ctl(ep, EPOLL_CTL_ADD, lfd, &ev);
    struct epoll_event evs[256];
    for (;;) {
        int n = epoll_wait(ep, evs, 256, -1);
        for (int i = 0; i < n; i++) {
            int fd = evs[i].data.fd;
            if (fd == lfd) {
                for (;;) {
                    int c = accept(lfd, NULL, NULL);
                    if (c < 0) break;
                    nodelay(c);
                    struct epoll_event e = { .events = EPOLLIN, .data.fd = c };
                    epoll_ctl(ep, EPOLL_CTL_ADD, c, &e);
                }
            } else {
                char buf[64];
                ssize_t k = read(fd, buf, sizeof buf);
                if (k <= 0) close(fd);
                else if (write(fd, buf, (size_t)k) != k) close(fd);
            }
        }
    }
}
#endif

// ---------------- クライアント (親プロセス) ----------------
static int make_listener(int *port) {
    int l = socket(AF_INET, SOCK_STREAM, 0);
    int one = 1;
    setsockopt(l, SOL_SOCKET, SO_REUSEADDR, &one, sizeof one);
    struct sockaddr_in a = { .sin_family = AF_INET, .sin_addr.s_addr = htonl(INADDR_LOOPBACK), .sin_port = 0 };
    if (bind(l, (struct sockaddr *)&a, sizeof a) != 0 || listen(l, 4096) != 0) { perror("listen"); exit(1); }
    socklen_t len = sizeof a;
    getsockname(l, (struct sockaddr *)&a, &len);
    *port = ntohs(a.sin_port);
    return l;
}

static int connect_to(int port) {
    for (int tries = 0; tries < 50; tries++) {
        int s = socket(AF_INET, SOCK_STREAM, 0);
        struct sockaddr_in a = { .sin_family = AF_INET, .sin_addr.s_addr = htonl(INADDR_LOOPBACK), .sin_port = htons((uint16_t)port) };
        if (connect(s, (struct sockaddr *)&a, sizeof a) == 0) {
            nodelay(s);
            struct linger lg = { 1, 0 };   // close で RST を送り、TIME_WAIT を残さない (ポートを使い果たさないため)
            setsockopt(s, SOL_SOCKET, SO_LINGER, &lg, sizeof lg);
            return s;
        }
        close(s);
        usleep(20000);
    }
    return -1;
}

static void run_text(const char *cmd, char *out, size_t n) {
    out[0] = 0;
    FILE *p = popen(cmd, "r");
    if (!p) return;
    if (!fgets(out, (int)n, p)) out[0] = 0;
    pclose(p);
    out[strcspn(out, "\n")] = 0;
}

static void scenario(const char *name, void (*serve)(int), int nconn, int nactive, double seconds) {
    int port;
    int lfd = make_listener(&port);
    pid_t pid = fork();
    if (pid == 0) { signal(SIGPIPE, SIG_IGN); serve(lfd); _exit(0); }
    close(lfd);

    static int fds[MAXC];
    double t0 = now_s();
    int ok = 0;
    for (int i = 0; i < nconn; i++) {
        fds[i] = connect_to(port);
        if (fds[i] < 0) break;
        ok++;
    }
    double t_conn = now_s() - t0;
    if (ok < nconn) printf("  (接続を %d 本までしか張れなかった)\n", ok);
    int na = nactive < ok ? nactive : ok;

    struct pollfd p[256];
    for (int i = 0; i < na; i++) { p[i].fd = fds[i]; p[i].events = POLLIN; }
    char b = 'x';
    for (int i = 0; i < na; i++) if (write(fds[i], &b, 1) != 1) break;
    long count = 0;
    double ts = now_s(), end = ts + seconds;
    while (now_s() < end) {
        int r = poll(p, (nfds_t)na, 100);
        if (r <= 0) continue;
        for (int i = 0; i < na; i++) {
            if (!(p[i].revents & POLLIN)) continue;
            if (read(p[i].fd, &b, 1) != 1) continue;
            count++;
            if (write(p[i].fd, &b, 1) != 1) break;
        }
    }
    double el = now_s() - ts;

    char cmd[128], rss[64], thr[64];
    snprintf(cmd, sizeof cmd, "ps -o rss= -p %d", (int)pid);
    run_text(cmd, rss, sizeof rss);
#ifdef __APPLE__
    snprintf(cmd, sizeof cmd, "ps -M -p %d | tail -n +2 | wc -l | tr -d ' '", (int)pid);
#else
    snprintf(cmd, sizeof cmd, "ps -o nlwp= -p %d | tr -d ' '", (int)pid);
#endif
    run_text(cmd, thr, sizeof thr);

    double rps = count / el;
    printf("  %-8s %5d本 (活発 %3d)  接続確立 %7.0f ms  %9.0f 往復/秒  平均 %7.1f µs  サーバー RSS %6.1f MB  スレッド %s\n", name, ok, na,
           t_conn * 1e3, rps, rps > 0 ? na / rps * 1e6 : 0.0, atof(rss) / 1024.0, thr);

    for (int i = 0; i < ok; i++) close(fds[i]);
    kill(pid, SIGKILL);
    waitpid(pid, NULL, 0);
}

int main(void) {
    raise_fd_limit();
    signal(SIGPIPE, SIG_IGN);
    const struct { const char *name; void (*fn)(int); } kinds[] = {
        {"threads", serve_threads}, {"poll", serve_poll},
#ifdef __APPLE__
        {"kqueue", serve_event},
#else
        {"epoll", serve_event},
#endif
    };
    const int cases[][2] = {{100, 100}, {100, 10}, {1000, 10}, {3000, 10}, {5000, 10}};
    for (size_t c = 0; c < sizeof cases / sizeof cases[0]; c++) {
        printf("\n[接続 %d 本のうち、活発なのは %d 本]\n", cases[c][0], cases[c][1]);
        for (size_t k = 0; k < sizeof kinds / sizeof kinds[0]; k++)
            scenario(kinds[k].name, kinds[k].fn, cases[c][0], cases[c][1], 1.5);
    }
    return 0;
}
