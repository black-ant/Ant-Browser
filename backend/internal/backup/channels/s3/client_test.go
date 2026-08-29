package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClientRequiresCredentials(t *testing.T) {
	if _, err := NewClient(Config{Bucket: "backup-bucket"}); err == nil || !strings.Contains(err.Error(), "access key ID") {
		t.Fatalf("NewClient error = %v, want access key validation", err)
	}
}

func TestClientTestUsesPathStyleAndSignature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodHead {
			t.Errorf("request method = %s, want HEAD", request.Method)
		}
		if request.URL.Path != "/backup-bucket" {
			t.Errorf("request path = %s, want /backup-bucket", request.URL.Path)
		}
		if request.Header.Get("x-amz-content-sha256") == "" {
			t.Error("missing x-amz-content-sha256 header")
		}
		if !strings.HasPrefix(request.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Error("missing AWS Signature V4 authorization")
		}
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "backup-bucket",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Test(context.Background()); err != nil {
		t.Fatalf("Test: %v", err)
	}
}

func TestBucketURLUsesVirtualHostStyle(t *testing.T) {
	client, err := NewClient(Config{
		Endpoint:        "https://s3.example.com",
		Region:          "us-east-1",
		Bucket:          "backup-bucket",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	target, err := client.bucketURL()
	if err != nil {
		t.Fatalf("bucketURL: %v", err)
	}
	if target.Host != "backup-bucket.s3.example.com" || target.Path != "/" {
		t.Fatalf("bucket URL = %s, want virtual host style", target.String())
	}
}
