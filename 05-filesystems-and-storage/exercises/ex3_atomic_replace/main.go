// ex3: ファイルの「安全な置き換え」
//
// 設定ファイルなどを、新しい内容で置き換えるとき、途中で止まると、どうなるか。
//
//	naive: ファイルを開いて (中身を空にして)、新しい内容を書く。
//	       → 途中で止まると、古い内容でも新しい内容でもない、中途半端なファイルが残る。
//	safe : 一時ファイルに書き、fsync して、rename で置き換え、ディレクトリも fsync する。
//	       → 途中で止まっても、古い内容か新しい内容の、どちらかが完全な形で残る。
//
// このデモは、子プロセスを書き込みの途中で SIGKILL する。
// **これは「プロセスが落ちた」場合の再現であって、「電源が落ちた」場合ではない。**
// プロセスが死んでも、OS のページキャッシュに入ったデータは残る。
// 電源断で何が失われるかは、このデモでは確かめられない (本文の説明を参照)。
//
//	go run . demo naive
//	go run . demo safe
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const (
	oldSize = 4 << 20  // 古い内容: 4 MiB の 'O'
	newSize = 10 << 20 // 新しい内容: 10 MiB の 'N'
	chunk   = 1 << 20
)

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// 書き込みの途中で、親が kill できるように、1 MiB ごとに少し待つ
func writeSlowly(f *os.File) error {
	buf := bytes.Repeat([]byte{'N'}, chunk)
	for written := 0; written < newSize; written += chunk {
		if _, err := f.Write(buf); err != nil {
			return err
		}
		fmt.Printf("progress %d\n", written+chunk)
		time.Sleep(150 * time.Millisecond)
	}
	return nil
}

func child(mode, path string) {
	switch mode {
	case "naive":
		f, err := os.Create(path) // ← ここで、中身が空になる
		if err != nil {
			panic(err)
		}
		writeSlowly(f)
		f.Close()
	case "safe":
		tmp := path + ".tmp"
		f, err := os.Create(tmp)
		if err != nil {
			panic(err)
		}
		writeSlowly(f)
		f.Sync() // 一時ファイルの中身を、先に永続化する
		f.Close()
		os.Rename(tmp, path)        // 名前の付け替えは、原子的
		syncDir(filepath.Dir(path)) // 名前の変更自体も、永続化する
	}
	fmt.Println("done")
}

func classify(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "読めない: " + err.Error()
	}
	switch {
	case len(data) == oldSize && bytes.Count(data, []byte{'O'}) == oldSize:
		return "古い内容が、そのまま残っている (完全)"
	case len(data) == newSize && bytes.Count(data, []byte{'N'}) == newSize:
		return "新しい内容が、完全に書き込まれた"
	default:
		return fmt.Sprintf("★ 古い内容でも新しい内容でもない (サイズ %d バイト)。壊れている", len(data))
	}
}

func demo(mode string) {
	dir, _ := os.MkdirTemp("", "ex3")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "config.dat")
	os.WriteFile(path, bytes.Repeat([]byte{'O'}, oldSize), 0o644)
	fmt.Printf("[%s] 初期状態: %s\n", mode, classify(path))

	cmd := exec.Command(os.Args[0], "child", mode, path)
	cmd.Stdout = nil
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	time.Sleep(450 * time.Millisecond) // 書き込みの途中 (約 3 MiB 書いたあたり)
	cmd.Process.Signal(syscall.SIGKILL)
	cmd.Wait()
	fmt.Printf("[%s] 書き込みの途中で、プロセスを強制終了した\n", mode)
	fmt.Printf("[%s] 結果: %s\n", mode, classify(path))

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		info, _ := e.Info()
		fmt.Printf("    ディレクトリの中身: %s (%d バイト)\n", e.Name(), info.Size())
	}

	if mode == "safe" { // 最後まで書かせると、新しい内容に置き換わる
		cmd := exec.Command(os.Args[0], "child", mode, path)
		cmd.Run()
		fmt.Printf("[%s] 最後まで実行した場合: %s\n", mode, classify(path))
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("使い方: go run . demo naive|safe")
		return
	}
	switch os.Args[1] {
	case "demo":
		demo(os.Args[2])
	case "child":
		child(os.Args[2], os.Args[3])
	}
}
