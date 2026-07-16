package gw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestS3ClientPresentsClientCertificate(t *testing.T) {
	t.Parallel()

	certs := testTLSCertsFor(t)
	var clientCertPresented atomic.Bool

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			clientCertPresented.Store(true)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	srv.TLS = certs.serverTLSConfig()
	srv.StartTLS()
	defer srv.Close()

	httpClient := srv.Client()
	httpClient.Transport = &http.Transport{
		TLSClientConfig: certs.clientTLSConfig(),
	}

	client := newTestS3ClientAnonymous(t, srv.URL, httpClient)
	_, err := client.HeadObject(context.Background(), headObjectInput("env", "key"))
	if err == nil {
		t.Fatal("expected error for missing object")
	}
	if !clientCertPresented.Load() {
		t.Fatal("server did not receive a client certificate during mTLS handshake")
	}
}
