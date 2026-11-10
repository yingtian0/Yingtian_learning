// ex1: ミニシェル (fork / exec / wait / pipe / リダイレクト)
//
// 対応する機能:
//   - コマンドの実行            例) ls -l
//   - パイプ                    例) seq 1 5 | sort -r | head -2
//   - リダイレクト              例) sort < in.txt > out.txt   (>> で追記)
//   - 組み込みコマンド          cd, exit
// 制限: トークンは空白で区切る (例: `a | b` と書く。`a|b` は不可)。
//       クォート、変数展開、ジョブ制御、バックグラウンド実行は扱わない。
//
// シェル自身は Ctrl-C (SIGINT) を無視し、子プロセスだけが既定の動作に戻して受け取る。
//
// build: cc -O1 -Wall -Wextra -o ex1 ex1_mini_shell.c
// run:   ./ex1                           (対話)
//        printf 'ls | wc -l\nexit\n' | ./ex1   (非対話)
#include <errno.h>
#include <fcntl.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/wait.h>
#include <unistd.h>

#define MAXARGS 64
#define MAXCMDS 8

typedef struct {
    char *argv[MAXARGS];
    char *in, *out;
    int append;
} cmd_t;

// 行を空白で区切り、`|` で分けてコマンドの配列にする。コマンド数を返す (エラーは -1)。
static int parse(char *line, cmd_t *cmds) {
    int n = 0, argc = 0;
    memset(&cmds[0], 0, sizeof cmds[0]);
    char *save = NULL;
    for (char *tok = strtok_r(line, " \t\n", &save); tok; tok = strtok_r(NULL, " \t\n", &save)) {
        cmd_t *c = &cmds[n];
        if (strcmp(tok, "|") == 0) {
            if (argc == 0 || ++n >= MAXCMDS) return -1;
            memset(&cmds[n], 0, sizeof cmds[n]);
            argc = 0;
        } else if (strcmp(tok, "<") == 0 || strcmp(tok, ">") == 0 || strcmp(tok, ">>") == 0) {
            char *file = strtok_r(NULL, " \t\n", &save);
            if (!file) return -1;
            if (tok[0] == '<') c->in = file;
            else { c->out = file; c->append = (tok[1] == '>'); }
        } else {
            if (argc >= MAXARGS - 1) return -1;
            c->argv[argc++] = tok;
            c->argv[argc] = NULL;
        }
    }
    if (argc == 0) return n == 0 ? 0 : -1;  // 空行は 0、`a |` のような末尾の `|` はエラー
    return n + 1;
}

static void run_child(cmd_t *c, int in_fd, int out_fd, int (*pipes)[2], int npipes) {
    signal(SIGINT, SIG_DFL);  // シェルが無視している設定を、子は引き継がない
    if (in_fd != STDIN_FILENO) dup2(in_fd, STDIN_FILENO);
    if (out_fd != STDOUT_FILENO) dup2(out_fd, STDOUT_FILENO);
    for (int i = 0; i < npipes; i++) { close(pipes[i][0]); close(pipes[i][1]); }
    if (c->in) {
        int fd = open(c->in, O_RDONLY);
        if (fd < 0) { perror(c->in); _exit(1); }
        dup2(fd, STDIN_FILENO); close(fd);
    }
    if (c->out) {
        int fd = open(c->out, O_WRONLY | O_CREAT | (c->append ? O_APPEND : O_TRUNC), 0644);
        if (fd < 0) { perror(c->out); _exit(1); }
        dup2(fd, STDOUT_FILENO); close(fd);
    }
    execvp(c->argv[0], c->argv);
    fprintf(stderr, "%s: %s\n", c->argv[0], strerror(errno));
    _exit(127);
}

static void run_pipeline(cmd_t *cmds, int n) {
    int pipes[MAXCMDS][2];
    pid_t pids[MAXCMDS];
    for (int i = 0; i < n - 1; i++)
        if (pipe(pipes[i]) != 0) { perror("pipe"); return; }

    for (int i = 0; i < n; i++) {
        int in_fd = (i == 0) ? STDIN_FILENO : pipes[i - 1][0];
        int out_fd = (i == n - 1) ? STDOUT_FILENO : pipes[i][1];
        pids[i] = fork();
        if (pids[i] < 0) { perror("fork"); break; }
        if (pids[i] == 0) run_child(&cmds[i], in_fd, out_fd, pipes, n - 1);
    }
    // 親は、パイプの両端をすべて閉じる。書き込み端が残っていると、
    // 読み手が EOF を受け取れず、パイプラインが終わらない。
    for (int i = 0; i < n - 1; i++) { close(pipes[i][0]); close(pipes[i][1]); }

    int status = 0;
    for (int i = 0; i < n; i++) {
        if (pids[i] > 0) waitpid(pids[i], &status, 0);  // 最後のコマンドの状態を残す
    }
    if (WIFEXITED(status) && WEXITSTATUS(status) != 0)
        fprintf(stderr, "[exit %d]\n", WEXITSTATUS(status));
    else if (WIFSIGNALED(status))
        fprintf(stderr, "[signal %d]\n", WTERMSIG(status));
}

int main(void) {
    signal(SIGINT, SIG_IGN);
    char line[1024];
    int interactive = isatty(STDIN_FILENO);
    for (;;) {
        if (interactive) { fputs("mini$ ", stdout); fflush(stdout); }
        if (!fgets(line, sizeof line, stdin)) break;
        cmd_t cmds[MAXCMDS];
        int n = parse(line, cmds);
        if (n < 0) { fputs("構文エラー\n", stderr); continue; }
        if (n == 0) continue;
        // 組み込みコマンドは、シェル自身のプロセスで実行する (子で cd しても親には効かない)。
        if (n == 1 && strcmp(cmds[0].argv[0], "exit") == 0) break;
        if (n == 1 && strcmp(cmds[0].argv[0], "cd") == 0) {
            const char *dir = cmds[0].argv[1] ? cmds[0].argv[1] : getenv("HOME");
            if (!dir || chdir(dir) != 0) perror("cd");
            continue;
        }
        run_pipeline(cmds, n);
    }
    return 0;
}
