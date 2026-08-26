package launchcode

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLaunchTasksTrackInFlightRequest(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := server.trackLaunchTask(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(entered)
		<-release
		writer.WriteHeader(http.StatusAccepted)
	}))

	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/launch", nil)
	request.RemoteAddr = "192.0.2.10:4321"
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()

	<-entered
	items := server.listLaunchTasks(10)
	if len(items) != 1 {
		t.Fatalf("active task count = %d, want 1", len(items))
	}
	if items[0].Status != "running" || items[0].Path != "/api/launch" || items[0].ClientIP != "192.0.2.10" {
		t.Fatalf("active task = %+v", items[0])
	}

	close(release)
	<-done
	if items := server.listLaunchTasks(10); len(items) != 0 {
		t.Fatalf("active task count after completion = %d, want 0", len(items))
	}
}

func TestHandleLaunchTasksReturnsActiveTasks(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/launch/tasks?limit=1", nil)
	response := httptest.NewRecorder()

	server.handleLaunchTasks(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.String() == "" {
		t.Fatal("response body is empty")
	}
}
