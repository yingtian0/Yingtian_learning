// ex5: ping を、ゼロから作る (ICMP エコー)
//
// ICMP (Internet Control Message Protocol) は、IP の「制御用」のメッセージで、
// エコー要求 (type 8) を送ると、エコー応答 (type 0) が返る。これが ping である。
// ここでは、管理者権限なしで使える ICMP ソケット (SOCK_DGRAM + IPPROTO_ICMP) を使い、
// ICMP メッセージを自分で組み立てる。宛先は、自分自身 (127.0.0.1) だけ。
//
// 見るもの:
//   - カーネルが返した、実際のパケットの、チェックサム (ex1 の実装が、本物のパケットで通るか)
//   - TTL、ループバックでの往復時間
//   - このマシンのネットワークインターフェースと MTU (アドレスは表示しない)
//
// 注意: macOS で動作を確認した。Linux では、net.ipv4.ping_group_range の設定が要る。
// 実行: go run .
package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"syscall"
	"time"
)

func checksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i:]))
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

func echoRequest(id, seq uint16, payload []byte) []byte {
	m := make([]byte, 8+len(payload))
	m[0] = 8 // type: エコー要求
	m[1] = 0 // code
	binary.BigEndian.PutUint16(m[4:], id)
	binary.BigEndian.PutUint16(m[6:], seq)
	copy(m[8:], payload)
	binary.BigEndian.PutUint16(m[2:], checksum(m))
	return m
}

func main() {
	fmt.Println("このマシンのネットワークインターフェース (名前、MTU、状態。アドレスは表示しない)")
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		fmt.Printf("  %-8s MTU %5d  %v\n", i.Name, i.MTU, i.Flags)
	}

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_ICMP)
	if err != nil {
		fmt.Println("\nICMP ソケットを開けなかった:", err)
		return
	}
	defer syscall.Close(fd)
	tv := syscall.Timeval{Sec: 1}
	syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
	dst := &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}

	fmt.Println("\n127.0.0.1 へ ping (56 バイトのデータ)")
	payload := make([]byte, 56)
	for i := range payload {
		payload[i] = byte(i)
	}
	var rtts []float64
	buf := make([]byte, 2048)
	for seq := 1; seq <= 200; seq++ {
		msg := echoRequest(0x1234, uint16(seq), payload)
		t0 := time.Now()
		if err := syscall.Sendto(fd, msg, 0, dst); err != nil {
			fmt.Println("送信に失敗:", err)
			return
		}
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		rtt := time.Since(t0)
		if err != nil {
			fmt.Printf("seq=%d 応答なし: %v\n", seq, err)
			continue
		}
		pkt := buf[:n]
		ip, icmp := []byte(nil), pkt
		if pkt[0]>>4 == 4 { // macOS は、IP ヘッダ付きで返す
			ihl := int(pkt[0]&0x0f) * 4
			ip, icmp = pkt[:ihl], pkt[ihl:]
		}
		rtts = append(rtts, float64(rtt.Nanoseconds())/1e3)
		if seq <= 3 {
			fmt.Printf("  seq=%d %d バイトを受信  ", seq, n)
			if ip != nil {
				fmt.Printf("IP: TTL=%d プロトコル=%d ヘッダチェックサム欄=0x%04x(合計して検算: %v)  ", ip[8], ip[9],
					binary.BigEndian.Uint16(ip[10:]), checksum(ip) == 0)
			}
			fmt.Printf("ICMP: type=%d code=%d seq=%d チェックサム検算=%v  往復 %.1f µs\n", icmp[0], icmp[1],
				binary.BigEndian.Uint16(icmp[6:]), checksum(icmp) == 0, float64(rtt.Nanoseconds())/1e3)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(rtts) > 0 {
		sort.Float64s(rtts)
		var sum float64
		for _, r := range rtts {
			sum += r
		}
		fmt.Printf("\n%d 回の往復時間: 最小 %.1f µs / 中央値 %.1f µs / 平均 %.1f µs / p99 %.1f µs / 最大 %.1f µs\n",
			len(rtts), rtts[0], rtts[len(rtts)/2], sum/float64(len(rtts)), rtts[len(rtts)*99/100], rtts[len(rtts)-1])
	}
}
