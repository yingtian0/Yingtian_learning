// ex3: 実際の TCP の内部の状態を覗く (macOS の TCP_CONNECTION_INFO)
//
//   A. 送信側のソケットの、輻輳ウィンドウ (cwnd) や、MSS、ウィンドウの拡大率などを表示する。
//      ループバックで、1 つの接続を、大量のデータで送り、cwnd が、どう育つかを見る。
//      (「相手のウィンドウ」= 相手が広告した受信ウィンドウ = 送信側から見た送信ウィンドウ。
//       「自分のウィンドウ」= このソケット自身の受信ウィンドウ。)
//   B. フロー制御: 受信側が、しばらく read しないと、受信ウィンドウが 0 になり、送信側が止まる。
//      受信側が止まっている間と、再開した後の、送信側の状態を見る。
//
// 注意: macOS 専用 (TCP_CONNECTION_INFO)。Linux では、同じ目的で TCP_INFO を使う (構造体が違う)。
// ループバックは、損失も遅延もないので、cwnd は、損失で縮むところまでは見えない。
// build: cc -O2 -Wall -Wextra -o ex3 ex3_tcp_info.c
// run:   ./ex3
#include <arpa/inet.h>
#include <errno.h>
#include <fcntl.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

#ifndef TCP_CONNECTION_INFO
#error "この演習は macOS 専用です (TCP_CONNECTION_INFO が必要)"
#endif

static double now_s(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}

static int get_info(int fd, struct tcp_connection_info *ti) {
    socklen_t len = sizeof *ti;
    memset(ti, 0, sizeof *ti);
    return getsockopt(fd, IPPROTO_TCP, TCP_CONNECTION_INFO, ti, &len);
}

static void receiver(int lfd, double delay_s) {
    int c = accept(lfd, NULL, NULL);
    if (delay_s > 0) usleep((useconds_t)(delay_s * 1e6));   // この間、read しない
    static char buf[1 << 20];
    while (read(c, buf, sizeof buf) > 0) {}
    close(c);
}

static int listener(int *port) {
    int l = socket(AF_INET, SOCK_STREAM, 0);
    struct sockaddr_in a = { .sin_family = AF_INET, .sin_addr.s_addr = htonl(INADDR_LOOPBACK), .sin_port = 0 };
    if (bind(l, (struct sockaddr *)&a, sizeof a) != 0 || listen(l, 4) != 0) { perror("listen"); exit(1); }
    socklen_t len = sizeof a;
    getsockname(l, (struct sockaddr *)&a, &len);
    *port = ntohs(a.sin_port);
    return l;
}

static int dial(int port) {
    int s = socket(AF_INET, SOCK_STREAM, 0);
    struct sockaddr_in a = { .sin_family = AF_INET, .sin_addr.s_addr = htonl(INADDR_LOOPBACK), .sin_port = htons((uint16_t)port) };
    if (connect(s, (struct sockaddr *)&a, sizeof a) != 0) { perror("connect"); exit(1); }
    return s;
}

static void print_header(void) {
    printf("  %9s %9s %10s %10s %10s %10s %10s %8s\n", "経過(ms)", "送った量", "cwnd", "ssthresh", "相手のウィンドウ", "自分のウィンドウ", "送信バッファ", "RTT(ms)");
}
static void print_row(double t, double sent, const struct tcp_connection_info *ti) {
    printf("  %9.1f %7.1f MB %8u B %8u B %12u B %12u B %10u B %8u\n", t * 1e3, sent / 1e6, ti->tcpi_snd_cwnd,
           ti->tcpi_snd_ssthresh, ti->tcpi_snd_wnd, ti->tcpi_rcv_wnd, ti->tcpi_snd_sbbytes, ti->tcpi_srtt);
}

int main(void) {
    signal(SIGPIPE, SIG_IGN);
    // ---------------- A ----------------
    int port;
    int l = listener(&port);
    pid_t pid = fork();
    if (pid == 0) { receiver(l, 0); _exit(0); }
    int s = dial(port);

    struct tcp_connection_info ti;
    get_info(s, &ti);
    printf("A. 接続した直後の状態 (ループバック)\n");
    printf("  MSS = %u バイト、cwnd = %u バイト (= %.1f セグメント)、ssthresh = %u バイト\n", ti.tcpi_maxseg, ti.tcpi_snd_cwnd,
           (double)ti.tcpi_snd_cwnd / ti.tcpi_maxseg, ti.tcpi_snd_ssthresh);
    printf("  ウィンドウ拡大率: 送信 %u / 受信 %u (ウィンドウは 2^拡大率 倍して読む)\n", ti.tcpi_snd_wscale, ti.tcpi_rcv_wscale);
    printf("  オプション: タイムスタンプ=%s SACK=%s ウィンドウ拡大=%s ECN=%s\n", (ti.tcpi_options & TCPCI_OPT_TIMESTAMPS) ? "有" : "無",
           (ti.tcpi_options & TCPCI_OPT_SACK) ? "有" : "無", (ti.tcpi_options & TCPCI_OPT_WSCALE) ? "有" : "無", (ti.tcpi_options & TCPCI_OPT_ECN) ? "有" : "無");

    printf("\n  64 MiB を送りながら、状態を観察する\n");
    print_header();
    static char buf[1 << 20];
    memset(buf, 'x', sizeof buf);
    double t0 = now_s(), total = 0, next_print = 0;
    while (total < 64.0 * (1 << 20)) {
        ssize_t w = write(s, buf, sizeof buf);
        if (w <= 0) break;
        total += (double)w;
        double t = now_s() - t0;
        if (total >= next_print) {
            get_info(s, &ti);
            print_row(t, total, &ti);
            next_print += 8.0 * (1 << 20);
        }
    }
    double el = now_s() - t0;
    get_info(s, &ti);
    printf("  合計 %.0f MB を %.1f ms で送信 (%.0f MB/s)。再送したバイト数 = %llu\n", total / 1e6, el * 1e3, total / 1e6 / el,
           (unsigned long long)ti.tcpi_txretransmitbytes);
    close(s);
    waitpid(pid, NULL, 0);
    close(l);

    // ---------------- B ----------------
    printf("\nB. フロー制御: 受信側が、1 秒間、read しない\n");
    l = listener(&port);
    pid = fork();
    if (pid == 0) { receiver(l, 1.0); _exit(0); }
    s = dial(port);
    fcntl(s, F_SETFL, fcntl(s, F_GETFL) | O_NONBLOCK);   // 詰まったら、ブロックせずに、状態を見たい
    print_header();
    t0 = now_s();
    total = 0;
    int stalled_printed = 0;
    double last_print = -1;
    while (now_s() - t0 < 1.8) {
        ssize_t w = write(s, buf, sizeof buf);
        double t = now_s() - t0;
        if (w > 0) total += (double)w;
        else if (errno != EAGAIN) break;
        if (w <= 0) {   // 送信バッファが一杯
            usleep(20000);
        }
        if (t - last_print > 0.25 || (w <= 0 && !stalled_printed)) {
            get_info(s, &ti);
            print_row(t, total, &ti);
            last_print = t;
            if (w <= 0) stalled_printed = 1;
        }
    }
    get_info(s, &ti);
    print_row(now_s() - t0, total, &ti);
    printf("  (受信側は、約 1 秒後に read を始める。それまで、送信側の write は、ほとんど進まない)\n");
    close(s);
    waitpid(pid, NULL, 0);
    return 0;
}
