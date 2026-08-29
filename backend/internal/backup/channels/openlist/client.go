package openlist

import (
	"ant-chrome/backend/internal/backup/channels"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	methodMKCOL    = `MKCOL`
	methodMOVE     = `MOVE`
	methodPROPFIND = `PROPFIND`
	propfindBody   = `<?xml version='1.0' encoding='utf-8'?><d:propfind xmlns:d='DAV:'><d:prop><d:displayname/><d:getcontentlength/><d:getlastmodified/><d:resourcetype/></d:prop></d:propfind>`

	TransferTimeout            = 2 * time.Hour
	ControlTimeout             = time.Minute
	DefaultUploadRateLimitMBps = 0
	maxUploadRateLimitMBps     = 1024 * 1024
	bytesPerMegabyte           = 1024 * 1024
)

type Config struct {
	BaseURL             string
	RemotePath          string
	Token               string
	UploadRateLimitMBps int
}

type File = channels.File

type Client struct {
	config     Config
	baseURL    *url.URL
	httpClient *http.Client
}

func (c *Client) ID() channels.ID {
	return channels.OpenList
}

func NewClient(cfg Config) (*Client, error) {
	baseURL, err := normalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	cfg.Token = strings.TrimSpace(cfg.Token)
	if cfg.Token == "" {
		return nil, fmt.Errorf(`OpenList token is empty`)
	}
	if cfg.UploadRateLimitMBps < 0 || cfg.UploadRateLimitMBps > maxUploadRateLimitMBps {
		return nil, fmt.Errorf(`OpenList upload rate limit must be between 0 and %d MB/s`, maxUploadRateLimitMBps)
	}
	remotePath, err := cleanRelativePath(cfg.RemotePath, true)
	if err != nil {
		return nil, fmt.Errorf(`invalid remote path: %w`, err)
	}
	cfg.BaseURL = baseURL.String()
	cfg.RemotePath = remotePath
	return &Client{
		config:     cfg,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: TransferTimeout},
	}, nil
}

func (c *Client) Test(ctx context.Context) error {
	if err := c.ensureRemoteDirectory(ctx); err != nil {
		return err
	}
	_, err := c.propfind(ctx, ``, `0`)
	return err
}

func (c *Client) List(ctx context.Context) ([]File, error) {
	items, err := c.propfind(ctx, ``, `1`)
	if err != nil {
		return nil, err
	}
	result := make([]File, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if item.Directory || name == `` || strings.EqualFold(name, `.uploading`) || !strings.HasSuffix(strings.ToLower(name), `.zip`) {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		leftTime, leftOK := parseModifiedAt(result[i].ModifiedAt)
		rightTime, rightOK := parseModifiedAt(result[j].ModifiedAt)
		if leftOK && rightOK && !leftTime.Equal(rightTime) {
			return leftTime.After(rightTime)
		}
		if leftOK != rightOK {
			return leftOK
		}
		if result[i].ModifiedAt != result[j].ModifiedAt {
			return result[i].ModifiedAt > result[j].ModifiedAt
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (c *Client) Upload(ctx context.Context, localPath, fileName string) (File, error) {
	cleanName, err := cleanFileName(fileName)
	if err != nil {
		return File{}, err
	}
	return c.uploadFile(ctx, localPath, cleanName, `backup`, nil)
}

func (c *Client) UploadWithProgress(ctx context.Context, localPath, fileName string, progress channels.UploadProgressFunc) (File, error) {
	cleanName, err := cleanFileName(fileName)
	if err != nil {
		return File{}, err
	}
	return c.uploadFile(ctx, localPath, cleanName, `backup`, progress)
}

func (c *Client) UploadMetadata(ctx context.Context, localPath, fileName string) (File, error) {
	cleanName, err := cleanMetadataFileName(fileName)
	if err != nil {
		return File{}, err
	}
	return c.uploadFile(ctx, localPath, cleanName, `backup metadata`, nil)
}

func (c *Client) UploadMetadataWithProgress(ctx context.Context, localPath, fileName string, progress channels.UploadProgressFunc) (File, error) {
	cleanName, err := cleanMetadataFileName(fileName)
	if err != nil {
		return File{}, err
	}
	return c.uploadFile(ctx, localPath, cleanName, `backup metadata`, progress)
}

func (c *Client) uploadFile(ctx context.Context, localPath, cleanName, artifactName string, progress channels.UploadProgressFunc) (File, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return File{}, fmt.Errorf(`stat local %s failed: %w`, artifactName, err)
	}
	if info.IsDir() {
		return File{}, fmt.Errorf(`local %s path is a directory`, artifactName)
	}
	if err := c.ensureRemoteDirectory(ctx); err != nil {
		return File{}, err
	}
	file, err := os.Open(localPath)
	if err != nil {
		return File{}, fmt.Errorf(`open local backup failed: %w`, err)
	}
	defer file.Close()

	temporaryName := cleanName + `.uploading`
	if err := c.put(ctx, temporaryName, file, info.Size(), progress); err != nil {
		_ = c.delete(ctx, temporaryName)
		return File{}, fmt.Errorf(`upload %s failed: %w`, artifactName, err)
	}
	if err := c.move(ctx, temporaryName, cleanName); err != nil {
		_ = c.delete(ctx, temporaryName)
		return File{}, fmt.Errorf(`finalize remote %s failed: %w`, artifactName, err)
	}
	remoteFile, err := c.stat(ctx, cleanName)
	if err != nil {
		return File{}, fmt.Errorf(`verify remote %s failed: %w`, artifactName, err)
	}
	if remoteFile.Size != info.Size() {
		_ = c.delete(ctx, cleanName)
		return File{}, fmt.Errorf(`remote %s size mismatch: local=%d remote=%d`, artifactName, info.Size(), remoteFile.Size)
	}
	return remoteFile, nil
}

func (c *Client) Download(ctx context.Context, fileName, localPath string) error {
	cleanName, err := cleanFileName(fileName)
	if err != nil {
		return err
	}
	response, err := c.request(ctx, http.MethodGet, cleanName, nil, -1, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !isSuccess(response.StatusCode) {
		return responseError(response)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf(`create local backup directory failed: %w`, err)
	}
	temporaryPath := localPath + `.tmp`
	file, err := os.Create(temporaryPath)
	if err != nil {
		return fmt.Errorf(`create downloaded backup failed: %w`, err)
	}
	written, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf(`download backup failed: %w`, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf(`close downloaded backup failed: %w`, closeErr)
	}
	if response.ContentLength >= 0 && written != response.ContentLength {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf(`downloaded backup size mismatch: expected=%d actual=%d`, response.ContentLength, written)
	}
	if err := os.Rename(temporaryPath, localPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf(`replace downloaded backup failed: %w`, err)
	}
	return nil
}

func (c *Client) ensureRemoteDirectory(ctx context.Context) error {
	segments, err := cleanPathSegments(c.config.RemotePath, true)
	if err != nil {
		return err
	}
	current := ``
	for _, segment := range segments {
		if current == `` {
			current = segment
		} else {
			current = pathpkg.Join(current, segment)
		}
		response, requestErr := c.requestAtPath(ctx, methodMKCOL, current, nil, -1, nil, false)
		if requestErr != nil {
			return requestErr
		}
		if isSuccess(response.StatusCode) {
			_ = response.Body.Close()
			continue
		}
		statusCode := response.StatusCode
		_ = response.Body.Close()
		if statusCode != http.StatusMethodNotAllowed && statusCode != http.StatusConflict {
			return fmt.Errorf(`create remote directory failed: HTTP %d`, statusCode)
		}
		if _, statErr := c.propfindAtPath(ctx, current, `0`, false); statErr != nil {
			return fmt.Errorf(`create remote directory failed: %w`, statErr)
		}
	}
	return nil
}

func (c *Client) put(ctx context.Context, remotePath string, body io.Reader, size int64, progress channels.UploadProgressFunc) error {
	if c.config.UploadRateLimitMBps > 0 {
		bytesPerSecond := int64(c.config.UploadRateLimitMBps) * bytesPerMegabyte
		body = channels.NewRateLimitedReader(ctx, body, bytesPerSecond)
	}
	body = channels.NewUploadProgressReader(body, size, progress)
	response, err := c.request(ctx, http.MethodPut, remotePath, body, size, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !isSuccess(response.StatusCode) {
		return responseError(response)
	}
	return nil
}

func (c *Client) move(ctx context.Context, sourcePath, targetPath string) error {
	targetURL, err := c.resourceURL(targetPath)
	if err != nil {
		return err
	}
	response, err := c.request(ctx, methodMOVE, sourcePath, nil, -1, map[string]string{
		`Destination`: targetURL.String(),
		`Overwrite`:   `T`,
	})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !isSuccess(response.StatusCode) {
		return responseError(response)
	}
	return nil
}

func (c *Client) delete(ctx context.Context, remotePath string) error {
	response, err := c.request(ctx, http.MethodDelete, remotePath, nil, -1, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if !isSuccess(response.StatusCode) {
		return responseError(response)
	}
	return nil
}

func (c *Client) stat(ctx context.Context, remotePath string) (File, error) {
	items, err := c.propfind(ctx, remotePath, `0`)
	if err != nil {
		return File{}, err
	}
	if len(items) == 0 {
		return File{}, fmt.Errorf(`remote file not found`)
	}
	return items[0], nil
}

func (c *Client) propfind(ctx context.Context, remotePath, depth string) ([]File, error) {
	return c.propfindAtPath(ctx, remotePath, depth, true)
}

func (c *Client) propfindAtPath(ctx context.Context, remotePath, depth string, includeRoot bool) ([]File, error) {
	body := bytes.NewReader([]byte(propfindBody))
	response, err := c.requestAtPath(ctx, methodPROPFIND, remotePath, body, int64(len(propfindBody)), map[string]string{
		`Depth`:        depth,
		`Content-Type`: `application/xml; charset=utf-8`,
	}, includeRoot)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if !isSuccess(response.StatusCode) {
		return nil, responseError(response)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 16*1024*1024))
	if err != nil {
		return nil, fmt.Errorf(`read remote directory failed: %w`, err)
	}
	return parsePropfind(data)
}

func (c *Client) request(ctx context.Context, method, remotePath string, body io.Reader, contentLength int64, headers map[string]string) (*http.Response, error) {
	return c.requestAtPath(ctx, method, remotePath, body, contentLength, headers, true)
}

func (c *Client) requestAtPath(ctx context.Context, method, remotePath string, body io.Reader, contentLength int64, headers map[string]string, includeRoot bool) (*http.Response, error) {
	target, err := c.resourceURLAtPath(remotePath, includeRoot)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, fmt.Errorf(`create remote request failed: %w`, err)
	}
	if contentLength >= 0 {
		request.ContentLength = contentLength
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if token := strings.TrimSpace(c.config.Token); token != `` {
		request.Header.Set(`Authorization`, `Bearer `+token)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf(`remote request failed: %w`, err)
	}
	return response, nil
}

func (c *Client) resourceURL(remotePath string) (*url.URL, error) {
	return c.resourceURLAtPath(remotePath, true)
}

func (c *Client) resourceURLAtPath(remotePath string, includeRoot bool) (*url.URL, error) {
	cleanPath, err := cleanRelativePath(remotePath, true)
	if err != nil {
		return nil, err
	}
	if includeRoot && c.config.RemotePath != `` {
		if cleanPath == `` {
			cleanPath = c.config.RemotePath
		} else {
			cleanPath = pathpkg.Join(c.config.RemotePath, cleanPath)
		}
	}
	result := *c.baseURL
	if cleanPath == `` {
		result.Path = strings.TrimRight(result.Path, `/`)
	} else if result.Path == `` || result.Path == `/` {
		result.Path = `/` + cleanPath
	} else {
		result.Path = pathpkg.Join(result.Path, cleanPath)
	}
	result.RawPath = ``
	return &result, nil
}

func normalizeBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf(`invalid OpenList URL: %w`, err)
	}
	if !strings.EqualFold(parsed.Scheme, `http`) && !strings.EqualFold(parsed.Scheme, `https`) {
		return nil, fmt.Errorf(`OpenList URL must use http or https`)
	}
	if parsed.Host == `` {
		return nil, fmt.Errorf(`OpenList URL host is empty`)
	}
	if parsed.RawQuery != `` || parsed.Fragment != `` {
		return nil, fmt.Errorf(`OpenList URL must not contain query or fragment`)
	}
	parsed.Path = strings.TrimRight(parsed.Path, `/`)
	parsed.RawPath = ``
	return parsed, nil
}

func cleanRelativePath(value string, allowEmpty bool) (string, error) {
	segments, err := cleanPathSegments(value, allowEmpty)
	if err != nil {
		return ``, err
	}
	return strings.Join(segments, `/`), nil
}

func cleanPathSegments(value string, allowEmpty bool) ([]string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), `\`, `/`)
	normalized = strings.Trim(normalized, `/`)
	if normalized == `` {
		if allowEmpty {
			return nil, nil
		}
		return nil, fmt.Errorf(`remote path is empty`)
	}
	parts := strings.Split(normalized, `/`)
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == `` || part == `.` {
			continue
		}
		if part == `..` {
			return nil, fmt.Errorf(`remote path traversal is not allowed`)
		}
		segments = append(segments, part)
	}
	if len(segments) == 0 && !allowEmpty {
		return nil, fmt.Errorf(`remote path is empty`)
	}
	return segments, nil
}

func cleanFileName(value string) (string, error) {
	name := strings.TrimSpace(strings.ReplaceAll(value, `\`, `/`))
	if name == `` || name == `.` || name == `..` || strings.Contains(name, `/`) {
		return ``, fmt.Errorf(`invalid remote backup file name`)
	}
	if !strings.HasSuffix(strings.ToLower(name), `.zip`) {
		return ``, fmt.Errorf(`remote backup file must use .zip suffix`)
	}
	return name, nil
}

func cleanMetadataFileName(value string) (string, error) {
	name := strings.TrimSpace(strings.ReplaceAll(value, `\`, `/`))
	if name == `` || name == `.` || name == `..` || strings.Contains(name, `/`) {
		return ``, fmt.Errorf(`invalid remote backup metadata file name`)
	}
	if !strings.HasSuffix(strings.ToLower(name), `.json`) {
		return ``, fmt.Errorf(`remote backup metadata file must use .json suffix`)
	}
	return name, nil
}

func parseModifiedAt(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == `` {
		return time.Time{}, false
	}
	if parsed, err := http.ParseTime(value); err == nil {
		return parsed, true
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func parsePropfind(data []byte) ([]File, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	items := make([]File, 0)
	var current *File
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf(`parse remote directory failed: %w`, err)
		}
		switch item := token.(type) {
		case xml.StartElement:
			switch item.Name.Local {
			case `response`:
				current = &File{}
			case `href`, `displayname`, `getcontentlength`, `getlastmodified`:
				if current == nil {
					continue
				}
				text, readErr := readElementText(decoder)
				if readErr != nil {
					return nil, readErr
				}
				switch item.Name.Local {
				case `href`:
					if current.Name == `` {
						current.Name = hrefBaseName(text)
					}
				case `displayname`:
					if strings.TrimSpace(text) != `` {
						current.Name = strings.TrimSpace(text)
					}
				case `getcontentlength`:
					current.Size, _ = strconv.ParseInt(strings.TrimSpace(text), 10, 64)
				case `getlastmodified`:
					current.ModifiedAt = strings.TrimSpace(text)
				}
			case `collection`:
				if current != nil {
					current.Directory = true
				}
				continue
			}
		case xml.EndElement:
			if item.Name.Local == `response` && current != nil {
				items = append(items, *current)
				current = nil
			}
		}
	}
	return items, nil
}

func readElementText(decoder *xml.Decoder) (string, error) {
	var builder strings.Builder
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return ``, fmt.Errorf(`read remote directory value failed: %w`, err)
		}
		switch item := token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			builder.Write([]byte(item))
		}
	}
	return builder.String(), nil
}

func hrefBaseName(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return ``
	}
	pathValue, err := url.PathUnescape(parsed.Path)
	if err != nil {
		pathValue = parsed.Path
	}
	return pathpkg.Base(strings.TrimRight(pathValue, `/`))
}

func responseError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	message := strings.TrimSpace(string(body))
	if response.StatusCode == http.StatusRequestEntityTooLarge {
		return fmt.Errorf(`remote request failed: HTTP 413 Request Entity Too Large：远端反向代理拒绝了过大的请求体（通常是 OpenResty/Nginx 的 client_max_body_size），请将其调大到超过备份文件大小；客户端限速和超时无法绕过此限制`)
	}
	if message == `` {
		message = http.StatusText(response.StatusCode)
	}
	return fmt.Errorf(`remote request failed: HTTP %d: %s`, response.StatusCode, message)
}

func isSuccess(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}

func pathpkgDir(value string) string {
	index := strings.LastIndexAny(value, `/\\`)
	if index < 0 {
		return `.`
	}
	if index == 0 {
		return value[:1]
	}
	return value[:index]
}
