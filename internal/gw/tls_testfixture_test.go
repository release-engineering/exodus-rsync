package gw

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"
)

// Ephemeral PKI for unit tests only. Generated once per test run via sync.Once;
// not related to exodus-gw certificates in test/data/.

type testTLSCerts struct {
	Server tls.Certificate
	Client tls.Certificate
	CAPool *x509.CertPool
}

var (
	testTLS     *testTLSCerts
	testTLSErr  error
	testTLSOnce sync.Once
)

func testTLSCertsFor(t *testing.T) *testTLSCerts {
	t.Helper()

	testTLSOnce.Do(func() {
		testTLS, testTLSErr = generateTestTLSCerts()
	})
	if testTLSErr != nil {
		t.Fatal(testTLSErr)
	}
	return testTLS
}

func (c *testTLSCerts) serverTLSConfig() *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{c.Server},
		ClientCAs:    c.CAPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}
}

func (c *testTLSCerts) clientTLSConfig() *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{c.Client},
		RootCAs:      c.CAPool,
		MinVersion:   tls.VersionTLS12,
	}
}

func generateTestTLSCerts() (*testTLSCerts, error) {
	ca, caKey, err := generateCA()
	if err != nil {
		return nil, fmt.Errorf("generate CA: %w", err)
	}

	serverCert, serverKey, err := generateCert(ca, caKey, "server", nil, true)
	if err != nil {
		return nil, fmt.Errorf("generate server cert: %w", err)
	}

	clientCert, clientKey, err := generateCert(ca, caKey, "client", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, false)
	if err != nil {
		return nil, fmt.Errorf("generate client cert: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})) {
		return nil, fmt.Errorf("append CA cert to pool")
	}

	return &testTLSCerts{
		Server: tls.Certificate{
			Certificate: [][]byte{serverCert.Raw},
			PrivateKey:  serverKey,
		},
		Client: tls.Certificate{
			Certificate: [][]byte{clientCert.Raw},
			PrivateKey:  clientKey,
		},
		CAPool: pool,
	}, nil
}

func generateCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "exodus-rsync unit test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

func generateCert(
	ca *x509.Certificate,
	caKey *rsa.PrivateKey,
	commonName string,
	extKeyUsage []x509.ExtKeyUsage,
	serverAuth bool,
) (*x509.Certificate, *rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  extKeyUsage,
	}

	if serverAuth {
		tmpl.DNSNames = []string{"localhost"}
		tmpl.ExtKeyUsage = append(tmpl.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
		tmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}
