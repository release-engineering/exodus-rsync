package gw

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/conf"
)

func TestNewClientCertError(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := conf.NewMockConfig(ctrl)

	cfg.EXPECT().GwCert().Return("cert-does-not-exist")
	cfg.EXPECT().GwKey().Return("key-does-not-exist")

	_, err := Package.NewClient(context.Background(), cfg)

	// Should have given us this error
	if !strings.Contains(fmt.Sprint(err), "can't load cert/key") {
		t.Error("did not get expected error, err =", err)
	}
}

func TestNewClientSessionError(t *testing.T) {
	cfg := testConfig(t)

	oldProvider := ext.awsSessionProvider
	defer func() { ext.awsSessionProvider = oldProvider }()

	ext.awsSessionProvider = func(_ session.Options) (*session.Session, error) {
		return nil, fmt.Errorf("simulated error")
	}

	_, err := Package.NewClient(context.Background(), cfg)

	// Should have given us this error
	if err.Error() != "create AWS session: simulated error" {
		t.Error("did not get expected error, err =", err)
	}
}

func TestNewClientOk(t *testing.T) {
	cfg := testConfig(t)

	client, err := Package.NewClient(context.Background(), cfg)

	// Should have succeeded
	if client == nil || err != nil {
		t.Errorf("unexpectedly failed to make client, client = %v, err = %v", client, err)
	}
}
