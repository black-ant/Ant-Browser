package openlist

import (
	"io"
	"net/http"
	"net/http/httptest"
	urlpkg "net/url"
	pathpkg "path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type memoryWebDAV struct {
	mu                           sync.Mutex
	dirs                         map[string]bool
	files                        map[string][]byte
	mkcolCalls                   int
	rejectMoves                  bool
	rejectPROPFIND               bool
	rejectHEAD                   bool
	rejectDirectoryTrailingSlash bool
	allowOptions                 bool
	hangFileStat                 bool
	hangPutResponse              bool
	putResponseStatus            int
	putResponseBody              string
	putStoredData                []byte
}

func newMemoryWebDAV() *memoryWebDAV {
	return &memoryWebDAV{
		dirs:  map[string]bool{``: true},
		files: make(map[string][]byte),
	}
}

func (store *memoryWebDAV) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(store.handle))
}

func (store *memoryWebDAV) hasFile(name string) bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	_, ok := store.files[name]
	return ok
}

func (store *memoryWebDAV) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(`Authorization`) != `Bearer secret` {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	resource := strings.Trim(strings.TrimPrefix(r.URL.Path, `/dav`), `/`)
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.rejectDirectoryTrailingSlash && resource != `` && strings.HasSuffix(r.URL.Path, `/`) && (r.Method == methodPROPFIND || r.Method == http.MethodHead || r.Method == http.MethodOptions) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method {
	case `MKCOL`:
		store.mkcolCalls++
		w.WriteHeader(http.StatusMethodNotAllowed)
	case http.MethodPut:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if store.putStoredData != nil {
			data = append([]byte(nil), store.putStoredData...)
		}
		store.files[resource] = data
		if store.putResponseStatus != 0 {
			w.WriteHeader(store.putResponseStatus)
			_, _ = w.Write([]byte(store.putResponseBody))
			return
		}
		if store.hangPutResponse {
			<-r.Context().Done()
			return
		}
		w.WriteHeader(http.StatusCreated)
	case methodMOVE:
		if store.rejectMoves {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		destinationURL, _ := urlpkg.Parse(r.Header.Get(`Destination`))
		destination := strings.Trim(strings.TrimPrefix(destinationURL.Path, `/dav`), `/`)
		store.files[destination] = append([]byte(nil), store.files[resource]...)
		delete(store.files, resource)
		w.WriteHeader(http.StatusCreated)
	case http.MethodDelete:
		delete(store.files, resource)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodGet:
		data, exists := store.files[resource]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	case http.MethodHead:
		if store.rejectHEAD {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if resource != `` && !store.dirs[resource] {
			if _, exists := store.files[resource]; !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}
		if store.dirs[resource] {
			w.Header().Set(`Content-Type`, `httpd/unix-directory`)
		}
		w.WriteHeader(http.StatusOK)
	case methodPROPFIND:
		if store.rejectPROPFIND {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if store.hangFileStat && strings.HasSuffix(resource, `.zip`) {
			time.Sleep(200 * time.Millisecond)
			return
		}
		if resource != `` && !store.dirs[resource] && store.files[resource] == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(207)
		_, _ = w.Write([]byte(store.propfindXML(resource, r.Header.Get(`Depth`))))
	case http.MethodOptions:
		if store.allowOptions {
			w.Header().Set(`DAV`, `1`)
			w.Header().Set(`Allow`, `OPTIONS, PROPFIND`)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (store *memoryWebDAV) propfindXML(resource, depth string) string {
	result := `<?xml version='1.0' encoding='utf-8'?><d:multistatus xmlns:d='DAV:'>`
	result += store.propfindEntry(resource, store.dirs[resource])
	if depth == `1` {
		for dir := range store.dirs {
			if dir != resource && pathpkg.Dir(dir) == resource {
				result += store.propfindEntry(dir, true)
			}
		}
		for file := range store.files {
			if pathpkg.Dir(file) == resource {
				result += store.propfindEntry(file, false)
			}
		}
	}
	return result + `</d:multistatus>`
}

func (store *memoryWebDAV) propfindEntry(resource string, directory bool) string {
	href := `/dav/` + resource
	if directory {
		href += `/`
	}
	name := pathpkg.Base(strings.TrimSuffix(resource, `/`))
	if resource == `` {
		name = `dav`
	}
	collection := ``
	length := 0
	if directory {
		collection = `<d:collection/>`
	} else {
		length = len(store.files[resource])
	}
	return `<d:response><d:href>` + href + `</d:href><d:propstat><d:prop><d:displayname>` + name + `</d:displayname><d:getcontentlength>` + strconv.Itoa(length) + `</d:getcontentlength><d:getlastmodified>2026-08-25T00:00:00Z</d:getlastmodified><d:resourcetype>` + collection + `</d:resourcetype></d:prop></d:propstat></d:response>`
}
