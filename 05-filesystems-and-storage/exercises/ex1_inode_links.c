// ex1: inode、リンク、ファイルディスクリプタ、スパースファイル
//
// ファイルの「名前」「中身(inode)」「開いている状態(fd)」が、別のものであることを確かめる。
//   1. ハードリンク     名前が2つ、inode は1つ (リンク数 2)
//   2. シンボリックリンク 別の inode。中身は「パス名の文字列」
//   3. unlink          名前を消すだけ。リンクが残っていれば、中身は消えない
//   4. 開いたまま unlink  fd がある間は、中身を読める。閉じたときに初めて領域が解放される
//   5. スパースファイル   サイズ 1 GiB でも、実際に使うブロックはごくわずか
//   6. オフセットの共有   open を2回 → 別々の位置。dup → 位置を共有
//
// build: cc -O1 -Wall -Wextra -o ex1 ex1_inode_links.c
// run:   ./ex1
#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

static char dir[256];

static void path(char *out, size_t n, const char *name) { snprintf(out, n, "%s/%s", dir, name); }

static void show(const char *label, const struct stat *st) {
    printf("  %-26s inode=%-10llu リンク数=%-2d サイズ=%-12lld 使用ブロック=%lld バイト\n", label,
           (unsigned long long)st->st_ino, (int)st->st_nlink, (long long)st->st_size,
           (long long)st->st_blocks * 512);
}

int main(void) {
    snprintf(dir, sizeof dir, "%s/ex1XXXXXX", getenv("TMPDIR") ? getenv("TMPDIR") : "/tmp");
    if (!mkdtemp(dir)) { perror("mkdtemp"); return 1; }
    char a[300], b[300], s[300], big[300], buf[64];
    path(a, sizeof a, "a.txt"); path(b, sizeof b, "b.txt"); path(s, sizeof s, "s.txt"); path(big, sizeof big, "big");
    struct stat st;

    printf("作業ディレクトリ: %s\n\n", dir);

    // 1. ハードリンク
    printf("[1] ハードリンク\n");
    int fd = open(a, O_CREAT | O_WRONLY, 0644);
    if (write(fd, "hello\n", 6) != 6) return 1;
    close(fd);
    stat(a, &st); show("a.txt を作った", &st);
    if (link(a, b) != 0) { perror("link"); return 1; }
    stat(a, &st); show("a.txt (link 後)", &st);
    stat(b, &st); show("b.txt (a.txt のハードリンク)", &st);

    // 2. シンボリックリンク
    printf("\n[2] シンボリックリンク\n");
    if (symlink(a, s) != 0) { perror("symlink"); return 1; }
    lstat(s, &st); show("s.txt 自身 (lstat)", &st);
    stat(s, &st);  show("s.txt の指す先 (stat)", &st);

    // 3. unlink
    printf("\n[3] a.txt を unlink する\n");
    unlink(a);
    stat(b, &st); show("b.txt (リンク数が減った)", &st);
    fd = open(b, O_RDONLY);
    ssize_t n = read(fd, buf, sizeof buf - 1);
    buf[n > 0 ? n : 0] = 0;
    printf("  b.txt の中身はまだ読める: %s", buf);
    close(fd);
    fd = open(s, O_RDONLY);
    printf("  s.txt (a.txt を指していた) を開くと: %s\n", fd < 0 ? strerror(errno) : "開けた");
    if (fd >= 0) close(fd);

    // 4. 開いたまま unlink
    printf("\n[4] 開いたまま unlink\n");
    fd = open(b, O_RDONLY);
    unlink(b);
    fstat(fd, &st); show("fd から見た状態", &st);
    n = pread(fd, buf, sizeof buf - 1, 0);
    buf[n > 0 ? n : 0] = 0;
    printf("  名前は無いが、fd からは読める: %s", buf);
    close(fd);
    printf("  close した時点で、領域が解放される\n");

    // 5. スパースファイル
    printf("\n[5] スパースファイル (1 GiB)\n");
    fd = open(big, O_CREAT | O_RDWR, 0644);
    if (lseek(fd, (off_t)1 << 30, SEEK_SET) < 0 || write(fd, "x", 1) != 1) { perror("sparse"); return 1; }
    fsync(fd);
    fstat(fd, &st); show("末尾に 1 バイトだけ書いた", &st);
    printf("  → サイズは 1 GiB だが、使っているのは %lld バイトだけ (対応するファイルシステムの場合)\n",
           (long long)st.st_blocks * 512);
    close(fd);
    unlink(big);

    // 6. オフセットの共有
    printf("\n[6] オフセットは「開いたファイル」ごとの状態\n");
    fd = open(a, O_CREAT | O_RDWR | O_TRUNC, 0644);
    if (write(fd, "0123456789", 10) != 10) return 1;
    int fd1 = open(a, O_RDONLY), fd2 = open(a, O_RDONLY), fd3 = dup(fd1);
    char c;
    if (read(fd1, &c, 1) != 1 || read(fd1, &c, 1) != 1) return 1;   // fd1 を 2 バイト進める
    printf("  fd1 で2バイト読んだ後の位置: fd1=%lld  別に open した fd2=%lld  dup した fd3=%lld\n",
           (long long)lseek(fd1, 0, SEEK_CUR), (long long)lseek(fd2, 0, SEEK_CUR), (long long)lseek(fd3, 0, SEEK_CUR));
    printf("  → open を2回 = 別々の「開いたファイル」(位置も別)。dup = 同じ「開いたファイル」を指す (位置を共有)\n");
    close(fd); close(fd1); close(fd2); close(fd3);

    unlink(a);
    rmdir(dir);
    return 0;
}
