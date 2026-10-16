package openlist

import (
	"ant-chrome/backend/internal/backup/channels"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
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
	methodMOVE     = `MOVE`
	methodPROPFIND = `PROPFIND`
	propfindBody   = `<?xml version='1.0' encoding='utf-8'?><d:propfind xmlns:d='DAV:'><d:prop><d:displayname/><d:getcontentlength/><d:getlastmodified/><d:resourcetype/></d:prop></d:propfind>`
	authModeBasic  = `basic`
	authModeToken  = `token`

	TransferTimeout            = 2 * time.Hour
	ControlTimeout             = time.Minute
	DefaultUploadRateLimitMBps = 0
	maxUploadRateLimitMBps     = 1024 * 1024
	bytesPerMegabyte           = 1024 * 1024
)

type Config struct {
	BaseURL             string
	RemotePath          string
	AuthMode            string
	Username            string
	Password            string
	Token               string
	UploadRateLimitMBps int
}

type File = channels.File

type Client struct {
	config         Config
	baseURL        *url.URL
	httpClient     *http.Client
	controlTimeout time.Duration
}

func (c *Client) ID() channels.ID {
	return channels.OpenList
}

func NewClient(cfg Config) (*Client, error) {
	baseURL, err := normalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	cfg.AuthMode = normalizeAuthMode(cfg.AuthMode, cfg.Username, cfg.Token)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.Password = strings.TrimSpace(cfg.Password)
	cfg.Token = normalizeToken(cfg.Token)
	if cfg.AuthMode == authModeBasic && cfg.Username == "" {
		return nil, fmt.Errorf(`OpenList WebDAV username is empty`)
	}
	if cfg.AuthMode == authModeBasic && cfg.Password == "" {
		return nil, fmt.Errorf(`OpenList WebDAV password is empty`)
	}
	if cfg.AuthMode == authModeToken && cfg.Token == "" {
		return nil, fmt.Errorf(`OpenList global token is empty`)
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
		config:         cfg,
		baseURL:        baseURL,
		httpClient:     newHTTPClient(),
		controlTimeout: ControlTimeout,
	}, nil
}

func newHTTPClient() *http.Client {
	client := &http.Client{
		Timeout: TransferTimeout,
	}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return nil
		}
		switch via[len(via)-1].Method {
		case http.MethodGet, http.MethodHead:
			return nil
		default:
			return http.ErrUseLastResponse
		}
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return client
	}
	transport = transport.Clone()
	transport.ResponseHeaderTimeout = ControlTimeout
	client.Transport = transport
	return client
}

func (c *Client) controlContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := c.controlTimeout
	if timeout <= 0 {
		timeout = ControlTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

func (c *Client) cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	} else {
		ctx = context.WithoutCancel(ctx)
	}
	return c.controlContext(ctx)
}

func (c *Client) Test(ctx context.Context) error {
	if err := c.probeRemoteDirectory(ctx, `/`, false); err != nil {
		return fmt.Errorf(`WebDAV endpoint check failed: %w`, err)
	}
	if err := c.validateRemoteDirectory(ctx); err != nil {
		return err
	}
	return nil
}

func (c *Client) List(ctx context.Context) ([]File, error) {
	items, err := c.propfindDirectory(ctx, ``, `1`)
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
	outcome, err := c.uploadFile(ctx, localPath, cleanName, `backup`, nil)
	return outcome.File, err
}

func (c *Client) UploadWithProgress(ctx context.Context, localPath, fileName string, progress channels.UploadProgressFunc) (File, error) {
	cleanName, err := cleanFileName(fileName)
	if err != nil {
		return File{}, err
	}
	outcome, err := c.uploadFile(ctx, localPath, cleanName, `backup`, progress)
	return outcome.File, err
}

func (c *Client) UploadWithProgressOutcome(ctx context.Context, localPath, fileName string, progress channels.UploadProgressFunc) (channels.UploadOutcome, error) {
	cleanName, err := cleanFileName(fileName)
	if err != nil {
		return channels.UploadOutcome{}, err
	}
	return c.uploadFile(ctx, localPath, cleanName, `backup`, progress)
}

func (c *Client) UploadMetadata(ctx context.Context, localPath, fileName string) (File, error) {
	cleanName, err := cleanMetadataFileName(fileName)
	if err != nil {
		return File{}, err
	}
	outcome, err := c.uploadFile(ctx, localPath, cleanName, `backup metadata`, nil)
	return outcome.File, err
}

func (c *Client) UploadMetadataWithProgress(ctx context.Context, localPath, fileName string, progress channels.UploadProgressFunc) (File, error) {
	cleanName, err := cleanMetadataFileName(fileName)
	if err != nil {
		return File{}, err
	}
	outcome, err := c.uploadFile(ctx, localPath, cleanName, `backup metadata`, progress)
	return outcome.File, err
}

func (c *Client) uploadFile(ctx context.Context, localPath, cleanName, artifactName string, progress channels.UploadProgressFunc) (channels.UploadOutcome, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	info, err := os.Stat(localPath)
	if err != nil {
		return channels.UploadOutcome{}, fmt.Errorf(`stat local %s failed: %w`, artifactName, err)
	}
	if info.IsDir() {
		return channels.UploadOutcome{}, fmt.Errorf(`local %s path is a directory`, artifactName)
	}
	controlCtx, controlCancel := c.controlContext(ctx)
	err = c.validateRemoteDirectory(controlCtx)
	controlCancel()
	if err != nil {
		return channels.UploadOutcome{}, err
	}
	file, err := os.Open(localPath)
	if err != nil {
		return channels.UploadOutcome{}, fmt.Errorf(`open local backup failed: %w`, err)
	}
	defer file.Close()

	if err := c.put(ctx, cleanName, file, info.Size(), progress); err != nil {
		committed := isCommittedUploadError(err)
		if isTimeoutError(err) || committed {
			reportUploadVerification(progress, info.Size())
			verifyCtx, verifyCancel := c.cleanupContext(ctx)
			remoteFile, verifyErr := c.stat(verifyCtx, cleanName)
			verifyCancel()
			if verifyErr == nil && remoteFile.Size == info.Size() {
				return channels.UploadOutcome{
					File:    remoteFile,
					Warning: uploadWarning(err, committed),
				}, nil
			}
			if committed {
				if verifyErr != nil {
					return channels.UploadOutcome{}, fmt.Errorf(`upload %s failed after OpenList reported the file was written: %w (remote verification failed: %v; remote file was left in place)`, artifactName, err, verifyErr)
				}
				return channels.UploadOutcome{}, fmt.Errorf(`upload %s failed after OpenList reported the file was written: %w (remote size mismatch: local=%d remote=%d; remote file was left in place)`, artifactName, err, info.Size(), remoteFile.Size)
			}
		}
		cleanupCtx, cleanupCancel := c.cleanupContext(ctx)
		_ = c.delete(cleanupCtx, cleanName)
		cleanupCancel()
		return channels.UploadOutcome{}, fmt.Errorf(`upload %s failed: %w`, artifactName, err)
	}
	reportUploadVerification(progress, info.Size())
	controlCtx, controlCancel = c.controlContext(ctx)
	remoteFile, err := c.stat(controlCtx, cleanName)
	controlCancel()
	if err != nil {
		verifyCtx, verifyCancel := c.cleanupContext(ctx)
		verifiedFile, verifyErr := c.stat(verifyCtx, cleanName)
		verifyCancel()
		if verifyErr == nil && verifiedFile.Size == info.Size() {
			return channels.UploadOutcome{File: verifiedFile}, nil
		}
		cleanupCtx, cleanupCancel := c.cleanupContext(ctx)
		_ = c.delete(cleanupCtx, cleanName)
		cleanupCancel()
		return channels.UploadOutcome{}, fmt.Errorf(`verify remote %s failed: %w`, artifactName, err)
	}
	if remoteFile.Size != info.Size() {
		cleanupCtx, cleanupCancel := c.cleanupContext(ctx)
		_ = c.delete(cleanupCtx, cleanName)
		cleanupCancel()
		return channels.UploadOutcome{}, fmt.Errorf(`remote %s size mismatch: local=%d remote=%d`, artifactName, info.Size(), remoteFile.Size)
	}
	return channels.UploadOutcome{File: remoteFile}, nil
}

func reportUploadVerification(progress channels.UploadProgressFunc, totalBytes int64) {
	if progress == nil {
		return
	}
	progress(channels.UploadProgress{
		BytesTransferred: totalBytes,
		TotalBytes:       totalBytes,
		Stage:            channels.UploadProgressStageVerifying,
	})
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}

func isCommittedUploadError(err error) bool {
	var responseErr *remoteHTTPError
	return errors.As(err, &responseErr) && responseErr.uploadCommitted
}

func uploadWarning(err error, committed bool) string {
	if !committed {
		return ``
	}
	var responseErr *remoteHTTPError
	if errors.As(err, &responseErr) && strings.TrimSpace(responseErr.committedMessage) != `` {
		return fmt.Sprintf(`%s；远端文件大小已校验`, strings.TrimSpace(responseErr.committedMessage))
	}
	return `OpenList 已报告文件写入虚拟盘，但目标同步失败；远端文件大小已校验`
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

func (c *Client) DownloadMetadata(ctx context.Context, fileName, localPath string) error {
	cleanName, err := cleanMetadataFileName(fileName)
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
		return fmt.Errorf(`create local backup metadata directory failed: %w`, err)
	}
	temporaryPath := localPath + `.tmp`
	file, err := os.Create(temporaryPath)
	if err != nil {
		return fmt.Errorf(`create downloaded backup metadata failed: %w`, err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, channels.MaxBackupMetadataBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf(`download backup metadata failed: %w`, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf(`close downloaded backup metadata failed: %w`, closeErr)
	}
	if written > channels.MaxBackupMetadataBytes {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf(`downloaded backup metadata exceeds %d bytes`, channels.MaxBackupMetadataBytes)
	}
	if response.ContentLength >= 0 && written != response.ContentLength {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf(`downloaded backup metadata size mismatch: expected=%d actual=%d`, response.ContentLength, written)
	}
	if err := os.Rename(temporaryPath, localPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf(`replace downloaded backup metadata failed: %w`, err)
	}
	return nil
}

func (c *Client) validateRemoteDirectory(ctx context.Context) error {
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
		exists, probeErr := c.remoteDirectoryExists(ctx, current, false)
		if probeErr != nil {
			return fmt.Errorf(`check remote directory %q failed: %w`, current, probeErr)
		}
		if exists {
			continue
		}
		return fmt.Errorf(`remote directory %q does not exist`, current)
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

func (c *Client) propfindDirectory(ctx context.Context, remotePath, depth string) ([]File, error) {
	return c.propfindDirectoryAtPath(ctx, remotePath, depth, true)
}

func (c *Client) propfindDirectoryAtPath(ctx context.Context, remotePath, depth string, includeRoot bool) ([]File, error) {
	directoryPath := directoryRemotePath(remotePath)
	items, err := c.propfindAtPath(ctx, directoryPath, depth, includeRoot)
	if err == nil || (!isRemoteHTTPStatus(err, http.StatusNotFound) && !isRemoteHTTPStatus(err, http.StatusMethodNotAllowed)) {
		return items, err
	}
	return c.propfindAtPath(ctx, remotePath, depth, includeRoot)
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

func (c *Client) probeRemoteDirectory(ctx context.Context, remotePath string, includeRoot bool) error {
	if _, err := c.propfindDirectoryAtPath(ctx, remotePath, `0`, includeRoot); err == nil {
		return nil
	} else if !isRemoteHTTPStatus(err, http.StatusMethodNotAllowed) {
		return err
	} else {
		exists, headErr := c.headDirectoryAtPath(ctx, remotePath, includeRoot)
		if headErr == nil && exists {
			return nil
		}
		if isRemoteHTTPStatus(headErr, http.StatusMethodNotAllowed) {
			if exists, optionsErr := c.optionsDirectoryAtPath(ctx, remotePath, includeRoot); optionsErr == nil && exists {
				return nil
			} else if optionsErr != nil && !isRemoteHTTPStatus(optionsErr, http.StatusNotFound) && !isRemoteHTTPStatus(optionsErr, http.StatusMethodNotAllowed) {
				return fmt.Errorf(`%w (PROPFIND fallback: %v)`, optionsErr, err)
			}
		} else if headErr != nil && !isRemoteHTTPStatus(headErr, http.StatusNotFound) {
			return headErr
		}
		return err
	}
}

func (c *Client) remoteDirectoryExists(ctx context.Context, remotePath string, includeRoot bool) (bool, error) {
	if _, err := c.propfindDirectoryAtPath(ctx, remotePath, `0`, includeRoot); err == nil {
		return true, nil
	} else if isRemoteHTTPStatus(err, http.StatusNotFound) {
		return false, nil
	} else if !isRemoteHTTPStatus(err, http.StatusMethodNotAllowed) {
		return false, err
	} else {
		exists, headErr := c.headDirectoryAtPath(ctx, remotePath, includeRoot)
		if headErr == nil {
			return exists, nil
		}
		if isRemoteHTTPStatus(headErr, http.StatusMethodNotAllowed) {
			if exists, optionsErr := c.optionsDirectoryAtPath(ctx, remotePath, includeRoot); optionsErr == nil {
				return exists, nil
			} else if isRemoteHTTPStatus(optionsErr, http.StatusNotFound) {
				return false, nil
			} else if !isRemoteHTTPStatus(optionsErr, http.StatusMethodNotAllowed) {
				return false, fmt.Errorf(`%w (PROPFIND fallback: %v)`, optionsErr, err)
			}
			return false, headErr
		}
		if isRemoteHTTPStatus(headErr, http.StatusNotFound) {
			return false, nil
		}
		return false, headErr
	}
}

func (c *Client) headDirectoryAtPath(ctx context.Context, remotePath string, includeRoot bool) (bool, error) {
	directoryPath := directoryRemotePath(remotePath)
	exists, err := c.headDirectoryAtPathRaw(ctx, directoryPath, includeRoot)
	if err == nil || (!isRemoteHTTPStatus(err, http.StatusNotFound) && !isRemoteHTTPStatus(err, http.StatusMethodNotAllowed)) {
		return exists, err
	}
	return c.headDirectoryAtPathRaw(ctx, remotePath, includeRoot)
}

func (c *Client) headDirectoryAtPathRaw(ctx context.Context, remotePath string, includeRoot bool) (bool, error) {
	response, err := c.requestAtPath(ctx, http.MethodHead, remotePath, nil, -1, nil, includeRoot)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, responseError(response)
	}
	if !isSuccess(response.StatusCode) {
		return false, responseError(response)
	}
	contentType := strings.ToLower(strings.TrimSpace(response.Header.Get(`Content-Type`)))
	if contentType != `httpd/unix-directory` && !strings.HasPrefix(contentType, `httpd/unix-directory;`) {
		return false, fmt.Errorf(`remote path is not a directory`)
	}
	return true, nil
}

func (c *Client) optionsDirectoryAtPath(ctx context.Context, remotePath string, includeRoot bool) (bool, error) {
	directoryPath := directoryRemotePath(remotePath)
	exists, err := c.optionsDirectoryAtPathRaw(ctx, directoryPath, includeRoot)
	if err == nil || (!isRemoteHTTPStatus(err, http.StatusNotFound) && !isRemoteHTTPStatus(err, http.StatusMethodNotAllowed)) {
		return exists, err
	}
	return c.optionsDirectoryAtPathRaw(ctx, remotePath, includeRoot)
}

func (c *Client) optionsDirectoryAtPathRaw(ctx context.Context, remotePath string, includeRoot bool) (bool, error) {
	response, err := c.requestAtPath(ctx, http.MethodOptions, remotePath, nil, -1, nil, includeRoot)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, responseError(response)
	}
	if !isSuccess(response.StatusCode) {
		return false, responseError(response)
	}
	dav := strings.TrimSpace(response.Header.Get(`DAV`))
	allow := strings.ToUpper(response.Header.Get(`Allow`))
	if !strings.Contains(allow, methodPROPFIND) {
		if dav == `` {
			return false, fmt.Errorf(`remote path does not advertise WebDAV directory listing`)
		}
		return false, fmt.Errorf(`remote path does not advertise WebDAV directory listing in Allow header`)
	}
	return true, nil
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
	switch c.config.AuthMode {
	case authModeBasic:
		request.SetBasicAuth(c.config.Username, c.config.Password)
	case authModeToken:
		if token := strings.TrimSpace(c.config.Token); token != `` {
			request.Header.Set(`Authorization`, `Bearer `+token)
		}
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
	if strings.HasSuffix(strings.TrimSpace(remotePath), `/`) && !strings.HasSuffix(result.Path, `/`) {
		result.Path += `/`
	}
	return &result, nil
}

func directoryRemotePath(value string) string {
	value = strings.TrimSpace(value)
	if value == `` || value == `/` {
		return `/`
	}
	if strings.HasSuffix(value, `/`) {
		return value
	}
	return value + `/`
}

func normalizeAuthMode(value, username, token string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case authModeBasic:
		return authModeBasic
	case authModeToken:
		return authModeToken
	}
	if strings.TrimSpace(username) != `` {
		return authModeBasic
	}
	if strings.TrimSpace(token) != `` {
		return authModeToken
	}
	return authModeBasic
}

func normalizeToken(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len(`Bearer `) && strings.EqualFold(value[:len(`Bearer `)], `Bearer `) {
		return strings.TrimSpace(value[len(`Bearer `):])
	}
	return value
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
	if strings.Trim(parsed.Path, `/`) == `` {
		return nil, fmt.Errorf(`OpenList URL cannot be the site root; use the full WebDAV endpoint`)
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

type remoteHTTPError struct {
	statusCode       int
	message          string
	uploadCommitted  bool
	committedMessage string
}

func (e *remoteHTTPError) Error() string {
	return e.message
}

func isRemoteHTTPStatus(err error, statusCode int) bool {
	var responseErr *remoteHTTPError
	return errors.As(err, &responseErr) && responseErr.statusCode == statusCode
}

func responseError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	message := strings.TrimSpace(string(body))
	uploadCommitted, committedMessage := committedUploadResponse(response.StatusCode, body)
	if response.StatusCode == http.StatusRequestEntityTooLarge {
		return fmt.Errorf(`remote request failed: HTTP 413 Request Entity Too Large：远端反向代理拒绝了过大的请求体（通常是 OpenResty/Nginx 的 client_max_body_size），请将其调大到超过备份文件大小；客户端限速和超时无法绕过此限制`)
	}
	if response.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf(`OpenList WebDAV 认证失败：请使用 WebDAV 用户名/密码，或 OpenList 服务端全局 Token；/api/auth/login 返回的登录 Token 不能直接用于 WebDAV`)
	}
	if isRedirectStatus(response.StatusCode) {
		if location := strings.TrimSpace(response.Header.Get(`Location`)); location != `` {
			message = fmt.Sprintf(`WebDAV endpoint redirected to %s; configure the final HTTPS WebDAV URL`, location)
		}
	}
	if message == `` {
		message = http.StatusText(response.StatusCode)
	}
	return &remoteHTTPError{
		statusCode:       response.StatusCode,
		message:          fmt.Sprintf(`remote request failed: HTTP %d: %s`, response.StatusCode, message),
		uploadCommitted:  uploadCommitted,
		committedMessage: committedMessage,
	}
}

func committedUploadResponse(statusCode int, body []byte) (bool, string) {
	if statusCode < http.StatusBadRequest {
		return false, ``
	}
	var payload struct {
		Message    string            `json:"message"`
		RemotePath string            `json:"remotePath"`
		Targets    []json.RawMessage `json:"targets"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(body), &payload); err != nil {
		return false, ``
	}
	if strings.TrimSpace(payload.RemotePath) == `` || len(payload.Targets) == 0 {
		return false, ``
	}
	message := strings.ToLower(strings.TrimSpace(payload.Message))
	written := strings.Contains(message, `写入`) || strings.Contains(message, `written`) || strings.Contains(message, `uploaded`)
	synchronized := strings.Contains(message, `同步`) || strings.Contains(message, `sync`)
	if !written || !synchronized {
		return false, ``
	}
	return true, strings.TrimSpace(payload.Message)
}

func isRedirectStatus(statusCode int) bool {
	return statusCode == http.StatusMovedPermanently || statusCode == http.StatusFound || statusCode == http.StatusSeeOther || statusCode == http.StatusTemporaryRedirect || statusCode == http.StatusPermanentRedirect
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
