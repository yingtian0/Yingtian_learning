package main

// DNS メッセージの、符号化と復号 (RFC 1035 の主要部分)。
// 対応: A、AAAA、NS、CNAME、MX、TXT、SOA。名前の圧縮 (ポインタ) にも対応する。

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

const (
	TypeA     = 1
	TypeNS    = 2
	TypeCNAME = 5
	TypeSOA   = 6
	TypeMX    = 15
	TypeTXT   = 16
	TypeAAAA  = 28

	RcodeOK       = 0
	RcodeNXDomain = 3
)

var typeNames = map[uint16]string{TypeA: "A", TypeNS: "NS", TypeCNAME: "CNAME", TypeSOA: "SOA", TypeMX: "MX", TypeTXT: "TXT", TypeAAAA: "AAAA"}

func typeName(t uint16) string {
	if n, ok := typeNames[t]; ok {
		return n
	}
	return fmt.Sprint(t)
}

type RR struct {
	Name string // 末尾に "." を付けた、小文字の名前
	Type uint16
	TTL  uint32
	Data string // A/AAAA: アドレス、NS/CNAME: 名前、TXT: 文字列、MX: "優先度 名前"、SOA: "mname rname serial refresh retry expire minimum"
}

type Message struct {
	ID                  uint16
	Response, AA, TC    bool
	RD, RA              bool
	Rcode               int
	QName               string
	QType               uint16
	Answer, Auth, Extra []RR
}

func fqdn(s string) string {
	s = strings.ToLower(s)
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

// ---- 符号化 ----

type packer struct {
	buf   []byte
	names map[string]int // 圧縮: 名前の (サフィックスの) 位置
}

func (p *packer) name(n string) {
	n = fqdn(n)
	for n != "." && n != "" {
		if off, ok := p.names[n]; ok {
			p.buf = binary.BigEndian.AppendUint16(p.buf, 0xC000|uint16(off))
			return
		}
		if len(p.buf) < 0x3fff {
			p.names[n] = len(p.buf)
		}
		label, rest, _ := strings.Cut(n, ".")
		p.buf = append(p.buf, byte(len(label)))
		p.buf = append(p.buf, label...)
		n = rest
	}
	p.buf = append(p.buf, 0)
}

func (p *packer) rr(r RR) {
	p.name(r.Name)
	p.buf = binary.BigEndian.AppendUint16(p.buf, r.Type)
	p.buf = binary.BigEndian.AppendUint16(p.buf, 1) // クラス IN
	p.buf = binary.BigEndian.AppendUint32(p.buf, r.TTL)
	lenPos := len(p.buf)
	p.buf = append(p.buf, 0, 0)
	start := len(p.buf)
	switch r.Type {
	case TypeA, TypeAAAA:
		a := netip.MustParseAddr(r.Data)
		p.buf = append(p.buf, a.AsSlice()...)
	case TypeNS, TypeCNAME:
		p.name(r.Data)
	case TypeMX:
		var pref int
		var host string
		fmt.Sscanf(r.Data, "%d %s", &pref, &host)
		p.buf = binary.BigEndian.AppendUint16(p.buf, uint16(pref))
		p.name(host)
	case TypeTXT:
		s := r.Data
		for len(s) > 255 {
			p.buf = append(p.buf, 255)
			p.buf = append(p.buf, s[:255]...)
			s = s[255:]
		}
		p.buf = append(p.buf, byte(len(s)))
		p.buf = append(p.buf, s...)
	case TypeSOA:
		f := strings.Fields(r.Data)
		p.name(f[0])
		p.name(f[1])
		for _, x := range f[2:] {
			var v uint32
			fmt.Sscanf(x, "%d", &v)
			p.buf = binary.BigEndian.AppendUint32(p.buf, v)
		}
	}
	binary.BigEndian.PutUint16(p.buf[lenPos:], uint16(len(p.buf)-start))
}

func (m *Message) Pack() []byte {
	p := &packer{names: map[string]int{}}
	var flags uint16
	if m.Response {
		flags |= 1 << 15
	}
	if m.AA {
		flags |= 1 << 10
	}
	if m.TC {
		flags |= 1 << 9
	}
	if m.RD {
		flags |= 1 << 8
	}
	if m.RA {
		flags |= 1 << 7
	}
	flags |= uint16(m.Rcode & 0xf)
	p.buf = binary.BigEndian.AppendUint16(p.buf, m.ID)
	p.buf = binary.BigEndian.AppendUint16(p.buf, flags)
	p.buf = binary.BigEndian.AppendUint16(p.buf, 1)
	p.buf = binary.BigEndian.AppendUint16(p.buf, uint16(len(m.Answer)))
	p.buf = binary.BigEndian.AppendUint16(p.buf, uint16(len(m.Auth)))
	p.buf = binary.BigEndian.AppendUint16(p.buf, uint16(len(m.Extra)))
	p.name(m.QName)
	p.buf = binary.BigEndian.AppendUint16(p.buf, m.QType)
	p.buf = binary.BigEndian.AppendUint16(p.buf, 1)
	for _, sec := range [][]RR{m.Answer, m.Auth, m.Extra} {
		for _, r := range sec {
			p.rr(r)
		}
	}
	return p.buf
}

// ---- 復号 ----

func readName(msg []byte, off int) (string, int, error) {
	var parts []string
	next := -1
	for hops := 0; hops < 32; hops++ {
		if off >= len(msg) {
			return "", 0, errors.New("名前が途中で切れている")
		}
		l := int(msg[off])
		switch {
		case l == 0:
			if next < 0 {
				next = off + 1
			}
			if len(parts) == 0 {
				return ".", next, nil
			}
			return strings.Join(parts, ".") + ".", next, nil
		case l&0xC0 == 0xC0: // 圧縮ポインタ
			if off+1 >= len(msg) {
				return "", 0, errors.New("ポインタが途中で切れている")
			}
			if next < 0 {
				next = off + 2
			}
			off = int(binary.BigEndian.Uint16(msg[off:]) & 0x3fff)
		default:
			if off+1+l > len(msg) {
				return "", 0, errors.New("ラベルが途中で切れている")
			}
			parts = append(parts, string(msg[off+1:off+1+l]))
			off += 1 + l
		}
	}
	return "", 0, errors.New("ポインタの輪")
}

func Unpack(msg []byte) (*Message, error) {
	if len(msg) < 12 {
		return nil, errors.New("短すぎる")
	}
	m := &Message{ID: binary.BigEndian.Uint16(msg)}
	f := binary.BigEndian.Uint16(msg[2:])
	m.Response, m.AA, m.TC = f&(1<<15) != 0, f&(1<<10) != 0, f&(1<<9) != 0
	m.RD, m.RA = f&(1<<8) != 0, f&(1<<7) != 0
	m.Rcode = int(f & 0xf)
	qd, an, ns, ar := int(binary.BigEndian.Uint16(msg[4:])), int(binary.BigEndian.Uint16(msg[6:])), int(binary.BigEndian.Uint16(msg[8:])), int(binary.BigEndian.Uint16(msg[10:]))
	off := 12
	for i := 0; i < qd; i++ {
		n, o, err := readName(msg, off)
		if err != nil || o+4 > len(msg) {
			return nil, errors.New("質問が壊れている")
		}
		m.QName, m.QType = n, binary.BigEndian.Uint16(msg[o:])
		off = o + 4
	}
	read := func(count int) ([]RR, error) {
		var out []RR
		for i := 0; i < count; i++ {
			n, o, err := readName(msg, off)
			if err != nil || o+10 > len(msg) {
				return nil, errors.New("RR が壊れている")
			}
			r := RR{Name: n, Type: binary.BigEndian.Uint16(msg[o:]), TTL: binary.BigEndian.Uint32(msg[o+4:])}
			rdlen := int(binary.BigEndian.Uint16(msg[o+8:]))
			rd := o + 10
			if rd+rdlen > len(msg) {
				return nil, errors.New("RDATA が壊れている")
			}
			switch r.Type {
			case TypeA, TypeAAAA:
				a, _ := netip.AddrFromSlice(msg[rd : rd+rdlen])
				r.Data = a.String()
			case TypeNS, TypeCNAME:
				r.Data, _, _ = readName(msg, rd)
			case TypeMX:
				host, _, _ := readName(msg, rd+2)
				r.Data = fmt.Sprintf("%d %s", binary.BigEndian.Uint16(msg[rd:]), host)
			case TypeTXT:
				var sb strings.Builder
				for p := rd; p < rd+rdlen; {
					l := int(msg[p])
					sb.Write(msg[p+1 : p+1+l])
					p += 1 + l
				}
				r.Data = sb.String()
			case TypeSOA:
				mn, p1, _ := readName(msg, rd)
				rn, p2, _ := readName(msg, p1)
				r.Data = fmt.Sprintf("%s %s %d %d %d %d %d", mn, rn, binary.BigEndian.Uint32(msg[p2:]), binary.BigEndian.Uint32(msg[p2+4:]),
					binary.BigEndian.Uint32(msg[p2+8:]), binary.BigEndian.Uint32(msg[p2+12:]), binary.BigEndian.Uint32(msg[p2+16:]))
			}
			out = append(out, r)
			off = rd + rdlen
		}
		return out, nil
	}
	var err error
	if m.Answer, err = read(an); err != nil {
		return nil, err
	}
	if m.Auth, err = read(ns); err != nil {
		return nil, err
	}
	if m.Extra, err = read(ar); err != nil {
		return nil, err
	}
	return m, nil
}
