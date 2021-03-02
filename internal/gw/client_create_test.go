package gw

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/release-engineering/exodus-rsync/internal/conf"
)

func TestNewClientCertError(t *testing.T) {
	cfg := conf.Config{
		GwCert: "cert-does-not-exist",
		GwKey:  "key-does-not-exist",
	}
	env := conf.Environment{Config: &cfg}

	_, err := Package.NewClient(env)

	// Should have given us this error
	if !strings.Contains(fmt.Sprint(err), "can't load cert/key") {
		t.Error("did not get expected error, err =", err)
	}
}

func TestNewClientSessionError(t *testing.T) {
	cfg := conf.Config{
		GwCert: "../../test/data/service.pem",
		GwKey:  "../../test/data/service-key.pem",
	}
	env := conf.Environment{Config: &cfg}

	oldProvider := ext.awsSessionProvider
	defer func() { ext.awsSessionProvider = oldProvider }()

	ext.awsSessionProvider = func(_ session.Options) (*session.Session, error) {
		return nil, fmt.Errorf("simulated error")
	}

	_, err := Package.NewClient(env)

	// Should have given us this error
	if err.Error() != "create AWS session: simulated error" {
		t.Error("did not get expected error, err =", err)
	}
}

func TestNewClientOk(t *testing.T) {
	cfg := conf.Config{
		GwCert: "../../test/data/service.pem",
		GwKey:  "../../test/data/service-key.pem",
	}
	env := conf.Environment{Config: &cfg}

	client, err := Package.NewClient(env)

	// Should have succeeded
	if client == nil || err != nil {
		t.Errorf("unexpectedly failed to make client, client = %v, err = %v", client, err)
	}
}
