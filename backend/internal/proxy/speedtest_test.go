package proxy

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDoSpeedTestRequestUsesSuccessfulHEADOnce(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Method != http.MethodHead {
			t.Fatalf("method = %s, want HEAD", request.Method)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	latency, statusCode, err := doSpeedTestRequest(server.Client(), server.URL)
	if err != nil {
		t.Fatalf("doSpeedTestRequest returned error: %v", err)
	}
	if statusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", statusCode, http.StatusNoContent)
	}
	if latency < 0 {
		t.Fatalf("latency = %d, want non-negative", latency)
	}
	if requests.Load() != 1 {
		t.Fatalf("request count = %d, want 1", requests.Load())
	}
}

func TestDoSpeedTestRequestFallsBackToGETWhenHEADUnsupported(t *testing.T) {
	var headRequests atomic.Int32
	var getRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodHead:
			headRequests.Add(1)
			writer.WriteHeader(http.StatusMethodNotAllowed)
		case http.MethodGet:
			getRequests.Add(1)
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method = %s", request.Method)
		}
	}))
	defer server.Close()

	_, statusCode, err := doSpeedTestRequest(server.Client(), server.URL)
	if err != nil {
		t.Fatalf("doSpeedTestRequest returned error: %v", err)
	}
	if statusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", statusCode, http.StatusNoContent)
	}
	if headRequests.Load() != 1 || getRequests.Load() != 1 {
		t.Fatalf("request counts = HEAD %d, GET %d; want HEAD 1, GET 1", headRequests.Load(), getRequests.Load())
	}
}
