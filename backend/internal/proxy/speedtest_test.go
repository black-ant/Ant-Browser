package proxy

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoSpeedTestRequestMeasuresSecondWarmRequest(t *testing.T) {
	var headRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodHead {
			t.Errorf("method = %s, want HEAD", request.Method)
			return
		}
		if headRequests.Add(1) == 1 {
			// 第一次请求模拟冷连接（TCP/代理握手）耗时，其延迟不应被报告。
			time.Sleep(300 * time.Millisecond)
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
	if headRequests.Load() != 2 {
		t.Fatalf("request count = %d, want 2 (warm-up + measured)", headRequests.Load())
	}
	if latency < 0 || latency >= 200 {
		t.Fatalf("latency = %d, want the second (warm) request below 200ms", latency)
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

func TestDoSpeedTestRequestFallsBackToGETWhenHEADNotImplemented(t *testing.T) {
	var headRequests atomic.Int32
	var getRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodHead:
			headRequests.Add(1)
			writer.WriteHeader(http.StatusNotImplemented)
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
