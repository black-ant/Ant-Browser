package cdp

import (
	"encoding/json"
	"time"
)

// HAR HAR格式根结构
type HAR struct {
	Log HARLog `json:"log"`
}

// HARLog HAR日志
type HARLog struct {
	Version string      `json:"version"`
	Creator HARCreator  `json:"creator"`
	Browser HARBrowser  `json:"browser,omitempty"`
	Pages   []HARPage   `json:"pages"`
	Entries []HAREntry  `json:"entries"`
}

// HARCreator 创建者信息
type HARCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// HARBrowser 浏览器信息
type HARBrowser struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// HARPage 页面信息
type HARPage struct {
	StartedDateTime string            `json:"startedDateTime"`
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	PageTimings     HARPageTimings    `json:"pageTimings"`
}

// HARPageTimings 页面时间
type HARPageTimings struct {
	OnContentLoad float64 `json:"onContentLoad,omitempty"`
	OnLoad        float64 `json:"onLoad,omitempty"`
}

// HAREntry HAR条目
type HAREntry struct {
	StartedDateTime string      `json:"startedDateTime"`
	Time            float64     `json:"time"`
	Request         HARRequest  `json:"request"`
	Response        HARResponse `json:"response"`
	Cache           HARCache    `json:"cache"`
	Timings         HARTimings  `json:"timings"`
	ServerIPAddress string      `json:"serverIPAddress,omitempty"`
	Connection      string      `json:"connection,omitempty"`
}

// HARRequest 请求信息
type HARRequest struct {
	Method      string       `json:"method"`
	URL         string       `json:"url"`
	HTTPVersion string       `json:"httpVersion"`
	Headers     []HARHeader  `json:"headers"`
	QueryString []HARParam   `json:"queryString"`
	Cookies     []HARCookie  `json:"cookies"`
	HeadersSize int64        `json:"headersSize"`
	BodySize    int64        `json:"bodySize"`
	PostData    *HARPostData `json:"postData,omitempty"`
}

// HARResponse 响应信息
type HARResponse struct {
	Status      int         `json:"status"`
	StatusText  string      `json:"statusText"`
	HTTPVersion string      `json:"httpVersion"`
	Headers     []HARHeader `json:"headers"`
	Cookies     []HARCookie `json:"cookies"`
	Content     HARContent  `json:"content"`
	RedirectURL string      `json:"redirectURL"`
	HeadersSize int64       `json:"headersSize"`
	BodySize    int64       `json:"bodySize"`
}

// HARHeader HTTP头
type HARHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HARParam 查询参数
type HARParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HARCookie Cookie
type HARCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Path     string `json:"path,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Expires  string `json:"expires,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
}

// HARPostData POST数据
type HARPostData struct {
	MimeType string      `json:"mimeType"`
	Params   []HARParam  `json:"params,omitempty"`
	Text     string      `json:"text,omitempty"`
}

// HARContent 内容
type HARContent struct {
	Size        int64  `json:"size"`
	Compression int64  `json:"compression,omitempty"`
	MimeType    string `json:"mimeType"`
	Text        string `json:"text,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
}

// HARCache 缓存信息
type HARCache struct {
	BeforeRequest *HARCacheEntry `json:"beforeRequest,omitempty"`
	AfterRequest  *HARCacheEntry `json:"afterRequest,omitempty"`
}

// HARCacheEntry 缓存条目
type HARCacheEntry struct {
	Expires    string `json:"expires,omitempty"`
	LastAccess string `json:"lastAccess"`
	ETag       string `json:"eTag,omitempty"`
	HitCount   int    `json:"hitCount"`
}

// HARTimings 时间信息
type HARTimings struct {
	Blocked float64 `json:"blocked,omitempty"`
	DNS     float64 `json:"dns,omitempty"`
	Connect float64 `json:"connect,omitempty"`
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
	SSL     float64 `json:"ssl,omitempty"`
}

// ExportHAR 导出HAR格式
func (s *CDPSession) ExportHAR() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	har := HAR{
		Log: HARLog{
			Version: "1.2",
			Creator: HARCreator{
				Name:    "Ant Browser",
				Version: "1.0.0",
			},
			Browser: HARBrowser{
				Name:    "Chrome",
				Version: "Unknown",
			},
			Pages:   []HARPage{},
			Entries: []HAREntry{},
		},
	}

	// 添加一个默认页面
	if len(s.networkRequests) > 0 {
		firstReq := s.networkRequests[0]
		har.Log.Pages = append(har.Log.Pages, HARPage{
			StartedDateTime: formatTimestamp(firstReq.Timestamp),
			ID:              "page_1",
			Title:           "Captured Traffic",
			PageTimings: HARPageTimings{
				OnContentLoad: -1,
				OnLoad:        -1,
			},
		})
	}

	// 转换所有请求
	for _, req := range s.networkRequests {
		entry := s.convertToHAREntry(&req)
		har.Log.Entries = append(har.Log.Entries, entry)
	}

	return json.MarshalIndent(har, "", "  ")
}

// convertToHAREntry 转换为HAR条目
func (s *CDPSession) convertToHAREntry(req *NetworkRequest) HAREntry {
	entry := HAREntry{
		StartedDateTime: formatTimestamp(req.Timestamp),
		Time:            float64(req.Duration),
		Request:         s.convertToHARRequest(req),
		Response:        s.convertToHARResponse(req),
		Cache:           HARCache{},
		Timings: HARTimings{
			Send:    0,
			Wait:    float64(req.Duration),
			Receive: 0,
		},
	}

	return entry
}

// convertToHARRequest 转换为HAR请求
func (s *CDPSession) convertToHARRequest(req *NetworkRequest) HARRequest {
	headers := make([]HARHeader, 0, len(req.RequestHeaders))
	for name, value := range req.RequestHeaders {
		headers = append(headers, HARHeader{
			Name:  name,
			Value: value,
		})
	}

	harReq := HARRequest{
		Method:      req.Method,
		URL:         req.URL,
		HTTPVersion: "HTTP/1.1",
		Headers:     headers,
		QueryString: []HARParam{},
		Cookies:     []HARCookie{},
		HeadersSize: -1,
		BodySize:    int64(len(req.RequestBody)),
	}

	if req.RequestBody != "" {
		harReq.PostData = &HARPostData{
			MimeType: "application/x-www-form-urlencoded",
			Text:     req.RequestBody,
		}
	}

	return harReq
}

// convertToHARResponse 转换为HAR响应
func (s *CDPSession) convertToHARResponse(req *NetworkRequest) HARResponse {
	headers := make([]HARHeader, 0, len(req.ResponseHeaders))
	for name, value := range req.ResponseHeaders {
		headers = append(headers, HARHeader{
			Name:  name,
			Value: value,
		})
	}

	return HARResponse{
		Status:      req.StatusCode,
		StatusText:  req.StatusText,
		HTTPVersion: "HTTP/1.1",
		Headers:     headers,
		Cookies:     []HARCookie{},
		Content: HARContent{
			Size:     req.Size,
			MimeType: req.MimeType,
			Text:     req.ResponseBody,
		},
		RedirectURL: "",
		HeadersSize: -1,
		BodySize:    req.Size,
	}
}

// formatTimestamp 格式化时间戳
func formatTimestamp(timestamp int64) string {
	t := time.Unix(0, timestamp*int64(time.Millisecond))
	return t.Format(time.RFC3339)
}
