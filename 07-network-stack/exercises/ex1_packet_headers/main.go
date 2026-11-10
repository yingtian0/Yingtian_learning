// ex1: パケットのヘッダを、バイト単位で作って、読む
//
//  1. インターネットチェックサム (RFC 1071) を実装し、IPv4 ヘッダの有名な例 (Wikipedia の例) で検算する。
//  2. イーサネット + IPv4 + UDP のパケットを、1バイトずつ組み立てて、16進で表示する。
//     UDP のチェックサムには「疑似ヘッダ」(IP アドレスなど) が含まれる。
//  3. 作ったパケットを読み戻して、チェックサムを検証する。
//  4. ビットが1つ壊れたら、検出できるか。16ビットの語を2つ入れ替えたら、検出できるか。
//  5. ルーターが TTL を 1 減らすとき、チェックサムを、最初から計算し直さず、差分だけで更新できるか (RFC 1624)。
//
// 注意: UDP チェックサムは、自作の実装で、実際のパケットとの照合はしていない (パケットの取得には管理者権限が要る)。
// 実際のカーネルが作ったチェックサムとの照合は、ex5 (ICMP) で行う。
package main

import (
	"encoding/binary"
	"fmt"
	"net/netip"
)

// RFC 1071: 16ビットの語の、1の補数の和の、補数。
func checksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i:]))
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	for sum>>16 != 0 { // 桁あふれた分を、下位に足し戻す (巡回キャリー)
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

// RFC 1624: 1つの16ビット語が old → new に変わったときの、チェックサムの差分更新。
func updateChecksum(ck, oldWord, newWord uint16) uint16 {
	sum := uint32(^ck) + uint32(^oldWord) + uint32(newWord)
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

type Packet struct {
	DstMAC, SrcMAC [6]byte
	SrcIP, DstIP   netip.Addr
	SrcPort        uint16
	DstPort        uint16
	TTL            uint8
	ID             uint16
	Payload        []byte
}

// 擬似ヘッダ (送信元 IP、宛先 IP、0、プロトコル、UDP の長さ) + UDP ヘッダ + データ のチェックサム
func udpChecksum(src, dst netip.Addr, udp []byte) uint16 {
	pseudo := make([]byte, 0, 12+len(udp))
	s, d := src.As4(), dst.As4()
	pseudo = append(pseudo, s[:]...)
	pseudo = append(pseudo, d[:]...)
	pseudo = append(pseudo, 0, 17) // プロトコル 17 = UDP
	pseudo = binary.BigEndian.AppendUint16(pseudo, uint16(len(udp)))
	pseudo = append(pseudo, udp...)
	ck := checksum(pseudo)
	if ck == 0 { // UDP では、計算結果が 0 のとき、0xffff を送る (0 は「チェックサムなし」の意味)
		ck = 0xffff
	}
	return ck
}

func (p *Packet) Build() []byte {
	udpLen := 8 + len(p.Payload)
	udp := make([]byte, udpLen)
	binary.BigEndian.PutUint16(udp[0:], p.SrcPort)
	binary.BigEndian.PutUint16(udp[2:], p.DstPort)
	binary.BigEndian.PutUint16(udp[4:], uint16(udpLen))
	copy(udp[8:], p.Payload)
	binary.BigEndian.PutUint16(udp[6:], udpChecksum(p.SrcIP, p.DstIP, udp))

	ip := make([]byte, 20)
	ip[0] = 0x45 // バージョン 4、ヘッダ長 5 語 (20 バイト)
	binary.BigEndian.PutUint16(ip[2:], uint16(20+udpLen))
	binary.BigEndian.PutUint16(ip[4:], p.ID)
	binary.BigEndian.PutUint16(ip[6:], 0x4000) // フラグ: DF (断片化禁止)
	ip[8] = p.TTL
	ip[9] = 17
	s, d := p.SrcIP.As4(), p.DstIP.As4()
	copy(ip[12:], s[:])
	copy(ip[16:], d[:])
	binary.BigEndian.PutUint16(ip[10:], checksum(ip))

	eth := make([]byte, 14)
	copy(eth[0:], p.DstMAC[:])
	copy(eth[6:], p.SrcMAC[:])
	binary.BigEndian.PutUint16(eth[12:], 0x0800) // EtherType: IPv4
	return append(append(eth, ip...), udp...)
}

func hexdump(b []byte) {
	for i := 0; i < len(b); i += 16 {
		end := i + 16
		if end > len(b) {
			end = len(b)
		}
		fmt.Printf("  %04x  ", i)
		for j := i; j < end; j++ {
			fmt.Printf("%02x ", b[j])
		}
		fmt.Println()
	}
}

func verify(frame []byte) (ipOK, udpOK bool) {
	ip := frame[14:34]
	udp := frame[34:]
	src := netip.AddrFrom4([4]byte(ip[12:16]))
	dst := netip.AddrFrom4([4]byte(ip[16:20]))
	ipOK = checksum(ip) == 0 // チェックサム欄を含めて合計すると、0 になるはず
	pseudo := make([]byte, 0, 12+len(udp))
	pseudo = append(pseudo, ip[12:20]...)
	pseudo = append(pseudo, 0, 17)
	pseudo = binary.BigEndian.AppendUint16(pseudo, uint16(len(udp)))
	pseudo = append(pseudo, udp...)
	udpOK = checksum(pseudo) == 0
	_, _ = src, dst
	return
}

func main() {
	fmt.Println("[1] IPv4 ヘッダのチェックサム (Wikipedia の例) の検算")
	hdr := []byte{0x45, 0x00, 0x00, 0x73, 0x00, 0x00, 0x40, 0x00, 0x40, 0x11, 0x00, 0x00, 0xc0, 0xa8, 0x00, 0x01, 0xc0, 0xa8, 0x00, 0xc7}
	ck := checksum(hdr)
	fmt.Printf("  チェックサム欄を 0 にして計算 = 0x%04x (期待値 0xb861) → %v\n", ck, ck == 0xb861)
	binary.BigEndian.PutUint16(hdr[10:], ck)
	fmt.Printf("  欄を埋めて、ヘッダ全体を合計し直すと = 0x%04x (正しければ 0) → %v\n", checksum(hdr), checksum(hdr) == 0)

	fmt.Println("\n[2] イーサネット + IPv4 + UDP のパケットを組み立てる")
	p := &Packet{
		DstMAC:  [6]byte{0x02, 0, 0, 0, 0, 0x02}, // 先頭ビットが 02 = ローカル管理のアドレス (実在の機器ではない)
		SrcMAC:  [6]byte{0x02, 0, 0, 0, 0, 0x01},
		SrcIP:   netip.MustParseAddr("192.0.2.1"), // 192.0.2.0/24 は、文書用に予約されたアドレス (RFC 5737)
		DstIP:   netip.MustParseAddr("192.0.2.2"),
		SrcPort: 12345, DstPort: 53, TTL: 64, ID: 0x1c46,
		Payload: []byte("hello, network stack!"),
	}
	frame := p.Build()
	hexdump(frame)
	fmt.Printf("  全体 %d バイト = イーサネット 14 + IP 20 + UDP 8 + データ %d\n", len(frame), len(p.Payload))
	fmt.Printf("  フィールド: 宛先MAC=%x 送信元MAC=%x EtherType=%x / IP: ver+IHL=%02x TTL=%d proto=%d cksum=%x / UDP: %d→%d len=%d cksum=%x\n",
		frame[0:6], frame[6:12], frame[12:14], frame[14], frame[22], frame[23], frame[24:26],
		binary.BigEndian.Uint16(frame[34:]), binary.BigEndian.Uint16(frame[36:]), binary.BigEndian.Uint16(frame[38:]), frame[40:42])

	fmt.Println("\n[3] 読み戻して、チェックサムを検証する")
	ipOK, udpOK := verify(frame)
	fmt.Printf("  IP ヘッダ: %v、UDP: %v\n", ipOK, udpOK)

	fmt.Println("\n[4] 壊れたとき、検出できるか")
	bad := append([]byte(nil), frame...)
	bad[50] ^= 0x01 // データの 1 ビットを反転
	ipOK, udpOK = verify(bad)
	fmt.Printf("  データの 1 ビットを反転: IP ヘッダ %v、UDP %v → 検出できた\n", ipOK, udpOK)

	swap := append([]byte(nil), frame...)
	// データ部の 16 ビット語を 2 つ入れ替える (先頭から 2 語目と 4 語目)
	a, b := 42+2, 42+6
	swap[a], swap[a+1], swap[b], swap[b+1] = swap[b], swap[b+1], swap[a], swap[a+1]
	ipOK, udpOK = verify(swap)
	same := string(swap[42:]) == string(frame[42:])
	fmt.Printf("  16ビット語を 2 つ入れ替え (データは変わった: %v): UDP %v → 検出できなかった (和は、順序を気にしない)\n", !same, udpOK)

	fmt.Println("\n[5] TTL を 1 減らしたとき、チェックサムの更新")
	ip := append([]byte(nil), frame[14:34]...)
	oldWord := binary.BigEndian.Uint16(ip[8:]) // TTL とプロトコルが同じ 16 ビット語に入っている
	oldCk := binary.BigEndian.Uint16(ip[10:])
	ip[8]-- // TTL を 1 減らす
	newWord := binary.BigEndian.Uint16(ip[8:])
	inc := updateChecksum(oldCk, oldWord, newWord)
	binary.BigEndian.PutUint16(ip[10:], 0)
	full := checksum(ip)
	fmt.Printf("  差分で更新したチェックサム = 0x%04x、最初から計算し直した値 = 0x%04x → 一致: %v\n", inc, full, inc == full)
}
