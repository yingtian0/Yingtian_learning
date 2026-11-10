// Package pki: 演習用の、使い捨ての認証局 (CA) と証明書を、実行時に作る。
// ここで作る鍵と証明書は、演習のためだけのもので、本物の認証局の信頼とは無関係である。
package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"
)

type Entity struct {
	Cert *x509.Certificate
	Key  crypto.Signer
	DER  []byte
}

func genKey(kind string) (crypto.Signer, error) {
	switch kind {
	case "ecdsa":
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "rsa2048":
		return rsa.GenerateKey(rand.Reader, 2048)
	case "rsa3072":
		return rsa.GenerateKey(rand.Reader, 3072)
	case "rsa4096":
		return rsa.GenerateKey(rand.Reader, 4096)
	}
	return nil, fmt.Errorf("不明な鍵の種類: %s", kind)
}

func serial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 100))
	return n
}

func create(tmpl *x509.Certificate, parent *Entity, kind string) (*Entity, error) {
	key, err := genKey(kind)
	if err != nil {
		return nil, err
	}
	var parentCert *x509.Certificate
	var signer crypto.Signer
	if parent == nil { // 自己署名
		parentCert, signer = tmpl, key
	} else {
		parentCert, signer = parent.Cert, parent.Key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parentCert, key.Public(), signer)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &Entity{Cert: cert, Key: key, DER: der}, nil
}

func NewRoot(cn, kind string) (*Entity, error) {
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: cn, Organization: []string{"演習用 (本物ではない)"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	return create(tmpl, nil, kind)
}

func (p *Entity) IssueCA(cn, kind string) (*Entity, error) {
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: cn, Organization: []string{"演習用 (本物ではない)"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(5 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	return create(tmpl, p, kind)
}

func (p *Entity) IssueLeaf(cn string, dns []string, kind string, notBefore, notAfter time.Time) (*Entity, error) {
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: cn},
		DNSNames:              dns,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	if kind != "ecdsa" {
		tmpl.KeyUsage |= x509.KeyUsageKeyEncipherment
	}
	return create(tmpl, p, kind)
}

// TLS の証明書の鎖 (葉 + 中間) を作る。ルートは、通常、送らない。
func Chain(leaf *Entity, inters ...*Entity) tls.Certificate {
	c := tls.Certificate{PrivateKey: leaf.Key, Leaf: leaf.Cert}
	c.Certificate = append(c.Certificate, leaf.DER)
	for _, i := range inters {
		c.Certificate = append(c.Certificate, i.DER)
	}
	return c
}

func Pool(roots ...*Entity) *x509.CertPool {
	p := x509.NewCertPool()
	for _, r := range roots {
		p.AddCert(r.Cert)
	}
	return p
}
