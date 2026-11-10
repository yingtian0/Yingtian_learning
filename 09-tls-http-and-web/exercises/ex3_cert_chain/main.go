// ex3: 証明書の鎖と、検証が失敗する場合
//
// 使い捨ての CA (ルート → 中間 → 葉) を作り、証明書の中身を見て、
// 検証が「成功する場合」と「失敗する場合」(7 種類) の、エラーメッセージを確認する。
// 最後に、実際の TLS 接続で、ホスト名が合わないとき、どうなるか (と、検証を切ると何が起きるか) を見る。
//
// 注意: すべて、演習のために作った使い捨ての鍵と証明書。実在の認証局とは無関係。
// 実行: go run ./ex3_cert_chain
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"time"

	"ex9web/internal/pki"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func describe(name string, c *x509.Certificate) {
	var ku []string
	for _, p := range []struct {
		bit  x509.KeyUsage
		name string
	}{{x509.KeyUsageDigitalSignature, "DigitalSignature"}, {x509.KeyUsageKeyEncipherment, "KeyEncipherment"}, {x509.KeyUsageCertSign, "CertSign"}, {x509.KeyUsageCRLSign, "CRLSign"}} {
		if c.KeyUsage&p.bit != 0 {
			ku = append(ku, p.name)
		}
	}
	var eku []string
	for _, e := range c.ExtKeyUsage {
		switch e {
		case x509.ExtKeyUsageServerAuth:
			eku = append(eku, "ServerAuth")
		case x509.ExtKeyUsageClientAuth:
			eku = append(eku, "ClientAuth")
		}
	}
	fmt.Printf("  [%s]\n", name)
	fmt.Printf("    主体 (Subject)      : %s\n", c.Subject.CommonName)
	fmt.Printf("    発行者 (Issuer)     : %s\n", c.Issuer.CommonName)
	fmt.Printf("    SAN (DNS 名)        : %v\n", c.DNSNames)
	fmt.Printf("    有効期間            : %s 〜 %s\n", c.NotBefore.Format("2006-01-02 15:04"), c.NotAfter.Format("2006-01-02 15:04"))
	fmt.Printf("    CA か               : %v    鍵の用途: %v    拡張鍵用途: %v\n", c.IsCA, ku, eku)
	fmt.Printf("    署名のアルゴリズム  : %v    公開鍵: %v    大きさ(DER): %d B\n", c.SignatureAlgorithm, c.PublicKeyAlgorithm, len(c.Raw))
}

func verify(name string, leaf *x509.Certificate, roots, inters *x509.CertPool, host string, at time.Time, usages []x509.ExtKeyUsage) {
	opts := x509.VerifyOptions{Roots: roots, Intermediates: inters, DNSName: host, CurrentTime: at, KeyUsages: usages}
	chains, err := leaf.Verify(opts)
	if err != nil {
		fmt.Printf("  %-34s → 失敗: %v\n", name, err)
		return
	}
	var names []string
	for _, c := range chains[0] {
		names = append(names, c.Subject.CommonName)
	}
	fmt.Printf("  %-34s → 成功 (鎖: %s)\n", name, strings.Join(names, " → "))
}

func main() {
	root, err := pki.NewRoot("演習用ルート CA", "ecdsa")
	must(err)
	inter, err := root.IssueCA("演習用 中間 CA", "ecdsa")
	must(err)
	now := time.Now()
	leaf, err := inter.IssueLeaf("example.test", []string{"example.test", "www.example.test"}, "ecdsa", now.Add(-time.Hour), now.Add(90*24*time.Hour))
	must(err)

	fmt.Println("1. 証明書の中身")
	describe("葉 (サーバーの証明書)", leaf.Cert)
	describe("中間 CA", inter.Cert)
	describe("ルート CA (自己署名)", root.Cert)

	fmt.Println("\n2. 検証 (x509 の Verify)")
	rootPool, interPool := pki.Pool(root), pki.Pool(inter)
	verify("正常 (中間をサーバーが送った)", leaf.Cert, rootPool, interPool, "example.test", now, nil)
	verify("SAN にある別名 (www)", leaf.Cert, rootPool, interPool, "www.example.test", now, nil)
	verify("中間が足りない (サーバーが送り忘れ)", leaf.Cert, rootPool, nil, "example.test", now, nil)
	verify("ホスト名が違う", leaf.Cert, rootPool, interPool, "evil.test", now, nil)
	verify("検証の時刻が、有効期間を過ぎた (+100 日)", leaf.Cert, rootPool, interPool, "example.test", now.Add(100*24*time.Hour), nil)
	otherRoot, _ := pki.NewRoot("別のルート CA (信頼していない)", "ecdsa")
	verify("信頼していないルート", leaf.Cert, pki.Pool(otherRoot), interPool, "example.test", now, nil)
	expired, _ := inter.IssueLeaf("example.test", []string{"example.test"}, "ecdsa", now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	verify("期限切れの証明書", expired.Cert, rootPool, interPool, "example.test", now, nil)
	future, _ := inter.IssueLeaf("example.test", []string{"example.test"}, "ecdsa", now.Add(24*time.Hour), now.Add(48*time.Hour))
	verify("まだ有効でない証明書", future.Cert, rootPool, interPool, "example.test", now, nil)
	verify("クライアント認証の用途で検証", leaf.Cert, rootPool, interPool, "example.test", now, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

	fmt.Println("\n3. 実際の TLS 接続で、ホスト名が合わないとき")
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	cfg := &tls.Config{Certificates: []tls.Certificate{pki.Chain(leaf, inter)}}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				s := tls.Server(c, cfg)
				if s.Handshake() == nil {
					s.Write([]byte("秘密のデータ"))
				}
			}(c)
		}
	}()
	dial := func(serverName string, skip bool) {
		c, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{RootCAs: rootPool, ServerName: serverName, InsecureSkipVerify: skip})
		label := fmt.Sprintf("ServerName=%q、検証を切る=%v", serverName, skip)
		if err != nil {
			fmt.Printf("  %-44s → 接続失敗: %v\n", label, err)
			return
		}
		defer c.Close()
		buf := make([]byte, 64)
		n, _ := c.Read(buf)
		fmt.Printf("  %-44s → 接続できた。受信: %q\n", label, string(buf[:n]))
	}
	dial("example.test", false)
	dial("other.test", false)
	dial("other.test", true)
	fmt.Println("  (InsecureSkipVerify = true は、相手が本物かを確認しない。暗号化はされるが、中間者にも、同じように接続できてしまう。)")
}
