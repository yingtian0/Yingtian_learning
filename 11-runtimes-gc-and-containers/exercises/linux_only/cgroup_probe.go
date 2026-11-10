//go:build linux

// 【Linux 専用・この Mac では未実行】cgroup v2 の、CPU 上限とメモリ上限を、読む
// 自分のプロセスが属する cgroup の cpu.max、memory.max、cpu.stat (スロットリングの回数) を表示する。
// コンテナの中で実行すると、docker run --cpus=2 -m 256m の値が見える。
// 実行 (Linux): go run cgroup_probe.go
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

func read(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "(読めない: " + err.Error() + ")"
	}
	return strings.TrimSpace(string(b))
}

func main() {
	fmt.Println("runtime.NumCPU() =", runtime.NumCPU(), " GOMAXPROCS =", runtime.GOMAXPROCS(0), "(Go のバージョンによっては、cgroup の上限を反映しない)")
	self := read("/proc/self/cgroup") // cgroup v2 では "0::/path"
	fmt.Println("/proc/self/cgroup:", self)
	base := "/sys/fs/cgroup"
	for _, f := range []string{"cpu.max", "memory.max", "memory.current", "pids.max"} {
		fmt.Printf("%-16s %s\n", f, read(base+"/"+f))
	}
	fmt.Println("cpu.stat:")
	fmt.Println(read(base + "/cpu.stat")) // nr_throttled、throttled_usec が、スロットリングの証拠
}
