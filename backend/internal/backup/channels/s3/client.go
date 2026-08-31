package s3

import (
	"ant-chrome/backend/internal/backup/channels"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	ControlTimeout    = time.Minute
	TransferTimeout   = 2 * time.Hour
	defaultRegion     = "us-east-1"
	serviceName       = "s3"
	emptyPayloadHash  = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	maxListResultSize = 32 * 1024 * 1024
)

type Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	ForcePathStyle  bool
}

type Client struct {
	config     Config
	endpoint   *url.URL
	httpClient *http.Client
}

type File = channels.File

type listObjectsV2Result struct {
	IsTruncated           bool              `xml:"IsTruncated"`
	NextContinuationToken string            `xml:"NextContinuationToken"`
	Contents              []listObjectEntry `xml:"Contents"`
}

type listObjectEntry struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	Size         int64  `xml:"Size"`
}

func (client *Client) ID() channels.ID {
	return channels.S3
}

func NewClient(configValue Config) (*Client, error) {
	configValue.Endpoint = strings.TrimSpace(configValue.Endpoint)
	configValue.Region = strings.TrimSpace(configValue.Region)
	if configValue.Region == "" {
		configValue.Region = defaultRegion
	}
	configValue.Bucket = strings.TrimSpace(configValue.Bucket)
	prefix, err := cleanPrefix(configValue.Prefix)
	if err != nil {
		return nil, err
	}
	configValue.Prefix = prefix
	configValue.AccessKeyID = strings.TrimSpace(configValue.AccessKeyID)
	configValue.SecretAccessKey = strings.TrimSpace(configValue.SecretAccessKey)
	configValue.SessionToken = strings.TrimSpace(configValue.SessionToken)

	if configValue.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket is empty")
	}
	if err := validateBucket(configValue.Bucket); err != nil {
		return nil, err
	}
	if configValue.AccessKeyID == "" {
		return nil, fmt.Errorf("S3 access key ID is empty")
	}
	if configValue.SecretAccessKey == "" {
		return nil, fmt.Errorf("S3 secret access key is empty")
	}

	endpoint, err := normalizeEndpoint(configValue.Endpoint, configValue.Region)
	if err != nil {
		return nil, err
	}
	if !configValue.ForcePathStyle && !isVirtualHostBucket(configValue.Bucket) {
		return nil, fmt.Errorf("S3 bucket name %q cannot be used with virtual-hosted style; enable path style", configValue.Bucket)
	}

	return &Client{
		config:     configValue,
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: TransferTimeout},
	}, nil
}

func (client *Client) Test(ctx context.Context) error {
	ctx = ensureContext(ctx)
	target, err := client.bucketURL()
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, target.String(), nil)
	if err != nil {
		return fmt.Errorf("create S3 bucket request failed: %w", err)
	}
	client.signRequest(request, time.Now().UTC(), emptyPayloadHash)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request S3 bucket failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return responseError(response, "S3 bucket check")
	}
	return nil
}

func (client *Client) List(ctx context.Context) ([]File, error) {
	ctx = ensureContext(ctx)
	result := make([]File, 0)
	seen := make(map[string]struct{})
	continuationToken := ""

	for {
		target, err := client.bucketURL()
		if err != nil {
			return nil, err
		}
		query := url.Values{}
		query.Set("list-type", "2")
		if prefix := client.objectPrefix(); prefix != "" {
			query.Set("prefix", prefix)
		}
		if continuationToken != "" {
			query.Set("continuation-token", continuationToken)
		}
		target.RawQuery = query.Encode()

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("create S3 list request failed: %w", err)
		}
		client.signRequest(request, time.Now().UTC(), emptyPayloadHash)
		response, err := client.httpClient.Do(request)
		if err != nil {
			return nil, fmt.Errorf("request S3 object list failed: %w", err)
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			err := responseError(response, "S3 object list")
			_ = response.Body.Close()
			return nil, err
		}

		var page listObjectsV2Result
		decodeErr := xml.NewDecoder(io.LimitReader(response.Body, maxListResultSize)).Decode(&page)
		closeErr := response.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("parse S3 object list failed: %w", decodeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close S3 object list response failed: %w", closeErr)
		}

		for _, object := range page.Contents {
			name, ok := client.objectName(object.Key)
			if !ok || strings.EqualFold(name, ".uploading") || !strings.HasSuffix(strings.ToLower(name), ".zip") {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			result = append(result, File{
				Name:       name,
				Size:       object.Size,
				ModifiedAt: strings.TrimSpace(object.LastModified),
			})
		}

		if !page.IsTruncated {
			break
		}
		nextToken := strings.TrimSpace(page.NextContinuationToken)
		if nextToken == "" || nextToken == continuationToken {
			return nil, fmt.Errorf("S3 object list returned an invalid continuation token")
		}
		continuationToken = nextToken
	}

	sort.SliceStable(result, func(left, right int) bool {
		leftTime, leftOK := parseModifiedAt(result[left].ModifiedAt)
		rightTime, rightOK := parseModifiedAt(result[right].ModifiedAt)
		if leftOK && rightOK && !leftTime.Equal(rightTime) {
			return leftTime.After(rightTime)
		}
		if leftOK != rightOK {
			return leftOK
		}
		if result[left].ModifiedAt != result[right].ModifiedAt {
			return result[left].ModifiedAt > result[right].ModifiedAt
		}
		return result[left].Name < result[right].Name
	})
	return result, nil
}

func (client *Client) Upload(ctx context.Context, localPath, fileName string) (File, error) {
	cleanName, err := cleanFileName(fileName)
	if err != nil {
		return File{}, err
	}
	return client.uploadFile(ctx, localPath, cleanName, "backup", "application/zip", nil)
}

func (client *Client) UploadWithProgress(ctx context.Context, localPath, fileName string, progress channels.UploadProgressFunc) (File, error) {
	cleanName, err := cleanFileName(fileName)
	if err != nil {
		return File{}, err
	}
	return client.uploadFile(ctx, localPath, cleanName, "backup", "application/zip", progress)
}

func (client *Client) UploadMetadata(ctx context.Context, localPath, fileName string) (File, error) {
	cleanName, err := cleanMetadataFileName(fileName)
	if err != nil {
		return File{}, err
	}
	return client.uploadFile(ctx, localPath, cleanName, "backup metadata", "application/json", nil)
}

func (client *Client) UploadMetadataWithProgress(ctx context.Context, localPath, fileName string, progress channels.UploadProgressFunc) (File, error) {
	cleanName, err := cleanMetadataFileName(fileName)
	if err != nil {
		return File{}, err
	}
	return client.uploadFile(ctx, localPath, cleanName, "backup metadata", "application/json", progress)
}

func (client *Client) uploadFile(ctx context.Context, localPath, fileName, artifactName, contentType string, progress channels.UploadProgressFunc) (File, error) {
	ctx = ensureContext(ctx)
	info, err := os.Stat(localPath)
	if err != nil {
		return File{}, fmt.Errorf("stat local %s failed: %w", artifactName, err)
	}
	if info.IsDir() {
		return File{}, fmt.Errorf("local %s path is a directory", artifactName)
	}
	file, err := os.Open(localPath)
	if err != nil {
		return File{}, fmt.Errorf("open local %s failed: %w", artifactName, err)
	}
	defer file.Close()

	payloadHash, err := hashFile(file)
	if err != nil {
		return File{}, fmt.Errorf("hash local %s failed: %w", artifactName, err)
	}
	target, err := client.objectURLForName(fileName)
	if err != nil {
		return File{}, err
	}
	body := channels.NewUploadProgressReader(file, info.Size(), progress)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, target.String(), body)
	if err != nil {
		return File{}, fmt.Errorf("create S3 upload request failed: %w", err)
	}
	request.ContentLength = info.Size()
	request.Header.Set("Content-Type", contentType)
	client.signRequest(request, time.Now().UTC(), payloadHash)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return File{}, fmt.Errorf("upload %s failed: %w", artifactName, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		err := responseError(response, "S3 upload")
		_ = response.Body.Close()
		return File{}, fmt.Errorf("upload %s failed: %w", artifactName, err)
	}
	if err := response.Body.Close(); err != nil {
		return File{}, fmt.Errorf("close S3 upload response failed: %w", err)
	}

	remoteFile, err := client.statObjectForName(ctx, fileName)
	if err != nil {
		return File{}, fmt.Errorf("verify remote %s failed: %w", artifactName, err)
	}
	if remoteFile.Size != info.Size() {
		return File{}, fmt.Errorf("remote %s size mismatch: local=%d remote=%d", artifactName, info.Size(), remoteFile.Size)
	}
	return remoteFile, nil
}

func (client *Client) Download(ctx context.Context, fileName, localPath string) error {
	ctx = ensureContext(ctx)
	cleanName, err := cleanFileName(fileName)
	if err != nil {
		return err
	}
	target, err := client.objectURL(cleanName)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return fmt.Errorf("create S3 download request failed: %w", err)
	}
	client.signRequest(request, time.Now().UTC(), emptyPayloadHash)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("download S3 backup failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return responseError(response, "S3 download")
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create local backup directory failed: %w", err)
	}
	temporaryPath := localPath + ".tmp"
	file, err := os.Create(temporaryPath)
	if err != nil {
		return fmt.Errorf("create downloaded backup failed: %w", err)
	}
	written, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("download S3 backup failed: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("close downloaded backup failed: %w", closeErr)
	}
	if response.ContentLength >= 0 && written != response.ContentLength {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("downloaded S3 backup size mismatch: expected=%d actual=%d", response.ContentLength, written)
	}
	if err := os.Rename(temporaryPath, localPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("replace downloaded backup failed: %w", err)
	}
	return nil
}

func (client *Client) statObject(ctx context.Context, fileName string) (File, error) {
	ctx = ensureContext(ctx)
	target, err := client.objectURL(fileName)
	if err != nil {
		return File{}, err
	}
	return client.statObjectAtTarget(ctx, target, fileName)
}

func (client *Client) statObjectForName(ctx context.Context, fileName string) (File, error) {
	ctx = ensureContext(ctx)
	target, err := client.objectURLForName(fileName)
	if err != nil {
		return File{}, err
	}
	return client.statObjectAtTarget(ctx, target, fileName)
}

func (client *Client) statObjectAtTarget(ctx context.Context, target *url.URL, fileName string) (File, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, target.String(), nil)
	if err != nil {
		return File{}, fmt.Errorf("create S3 object check request failed: %w", err)
	}
	client.signRequest(request, time.Now().UTC(), emptyPayloadHash)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return File{}, fmt.Errorf("request S3 object check failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return File{}, responseError(response, "S3 object check")
	}
	if response.ContentLength < 0 {
		contentLength, parseErr := strconv.ParseInt(strings.TrimSpace(response.Header.Get("Content-Length")), 10, 64)
		if parseErr != nil || contentLength < 0 {
			return File{}, fmt.Errorf("S3 object check did not return a valid content length")
		}
		response.ContentLength = contentLength
	}
	modifiedAt := strings.TrimSpace(response.Header.Get("Last-Modified"))
	if modifiedAt == "" {
		modifiedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return File{
		Name:       fileName,
		Size:       response.ContentLength,
		ModifiedAt: modifiedAt,
	}, nil
}

func (client *Client) objectURL(fileName string) (*url.URL, error) {
	cleanName, err := cleanFileName(fileName)
	if err != nil {
		return nil, err
	}
	return client.objectURLForName(cleanName)
}

func (client *Client) objectURLForName(fileName string) (*url.URL, error) {
	cleanName, err := cleanObjectName(fileName)
	if err != nil {
		return nil, err
	}
	return client.objectURLForKey(client.objectPrefix() + cleanName)
}

func (client *Client) objectURLForKey(key string) (*url.URL, error) {
	result, err := client.bucketURL()
	if err != nil {
		return nil, err
	}
	result.Path = joinURLPath(result.Path, key)
	result.RawPath = ""
	result.RawQuery = ""
	result.Fragment = ""
	return result, nil
}

func (client *Client) objectPrefix() string {
	if client.config.Prefix == "" {
		return ""
	}
	return client.config.Prefix + "/"
}

func (client *Client) objectName(key string) (string, bool) {
	prefix := client.objectPrefix()
	if !strings.HasPrefix(key, prefix) {
		return "", false
	}
	name := strings.TrimPrefix(key, prefix)
	if name == "" || strings.Contains(name, "/") || strings.HasSuffix(name, "/") {
		return "", false
	}
	cleanName, err := cleanFileName(name)
	if err != nil {
		return "", false
	}
	return cleanName, true
}

func (client *Client) bucketURL() (*url.URL, error) {
	result := *client.endpoint
	result.RawQuery = ""
	result.Fragment = ""
	if client.config.ForcePathStyle {
		result.Path = joinURLPath(result.Path, client.config.Bucket)
	} else {
		host := result.Hostname()
		if host == "" {
			return nil, fmt.Errorf("S3 endpoint host is empty")
		}
		result.Host = client.config.Bucket + "." + host
		if port := result.Port(); port != "" {
			result.Host += ":" + port
		}
		if result.Path == "" {
			result.Path = "/"
		}
	}
	result.RawPath = ""
	if result.Path == "" {
		result.Path = "/"
	}
	return &result, nil
}

func normalizeEndpoint(rawEndpoint, region string) (*url.URL, error) {
	if rawEndpoint == "" {
		host := "s3.amazonaws.com"
		switch {
		case strings.HasPrefix(region, "cn-"):
			host = "s3." + region + ".amazonaws.com.cn"
		case region != defaultRegion:
			host = "s3." + region + ".amazonaws.com"
		}
		rawEndpoint = "https://" + host
	}

	parsed, err := url.Parse(rawEndpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid S3 endpoint: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("S3 endpoint must use http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("S3 endpoint host is empty")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("S3 endpoint must not contain user information")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("S3 endpoint must not contain query or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return parsed, nil
}

func validateBucket(bucket string) error {
	for _, character := range bucket {
		if character == '/' || character == '\\' || unicode.IsSpace(character) || unicode.IsControl(character) {
			return fmt.Errorf("S3 bucket name contains an invalid character")
		}
	}
	return nil
}

func isVirtualHostBucket(bucket string) bool {
	if len(bucket) < 3 || len(bucket) > 63 || strings.Contains(bucket, "..") {
		return false
	}
	for index, character := range bucket {
		if character >= 'A' && character <= 'Z' {
			return false
		}
		if !(character >= 'a' && character <= 'z') && !(character >= '0' && character <= '9') && character != '-' && character != '.' {
			return false
		}
		if (index == 0 || index == len(bucket)-1) && (character == '-' || character == '.') {
			return false
		}
	}
	return true
}

func joinURLPath(basePath, pathSegment string) string {
	basePath = strings.TrimRight(basePath, "/")
	return basePath + "/" + pathSegment
}

func (client *Client) signRequest(request *http.Request, timestamp time.Time, payloadHash string) {
	if strings.TrimSpace(payloadHash) == "" {
		payloadHash = emptyPayloadHash
	}
	amzDate := timestamp.UTC().Format("20060102T150405Z")
	date := timestamp.UTC().Format("20060102")
	request.Header.Set("x-amz-content-sha256", payloadHash)
	request.Header.Set("x-amz-date", amzDate)
	if client.config.SessionToken != "" {
		request.Header.Set("x-amz-security-token", client.config.SessionToken)
	}

	host := request.URL.Host
	if request.Host != "" {
		host = request.Host
	}
	canonicalHeaders := []string{
		"host:" + strings.ToLower(host),
		"x-amz-content-sha256:" + payloadHash,
		"x-amz-date:" + amzDate,
	}
	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	if client.config.SessionToken != "" {
		canonicalHeaders = append(canonicalHeaders, "x-amz-security-token:"+client.config.SessionToken)
		signedHeaders = append(signedHeaders, "x-amz-security-token")
	}
	canonicalHeaderText := strings.Join(canonicalHeaders, "\n") + "\n"
	signedHeaderText := strings.Join(signedHeaders, ";")
	canonicalRequest := strings.Join([]string{
		request.Method,
		canonicalURI(request.URL),
		canonicalQuery(request.URL),
		canonicalHeaderText,
		signedHeaderText,
		payloadHash,
	}, "\n")

	scope := date + "/" + client.config.Region + "/" + serviceName + "/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hexHash(canonicalRequest),
	}, "\n")
	signingKey := deriveSigningKey(client.config.SecretAccessKey, date, client.config.Region, serviceName)
	signature := hexHMAC(signingKey, stringToSign)
	request.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		client.config.AccessKeyID,
		scope,
		signedHeaderText,
		signature,
	))
}

func canonicalURI(target *url.URL) string {
	path := target.EscapedPath()
	if path == "" {
		return "/"
	}
	return path
}

func canonicalQuery(target *url.URL) string {
	values := target.Query()
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0)
	for _, key := range keys {
		items := append([]string{}, values[key]...)
		sort.Strings(items)
		if len(items) == 0 {
			items = []string{""}
		}
		for _, item := range items {
			parts = append(parts, awsURLEncode(key)+"="+awsURLEncode(item))
		}
	}
	return strings.Join(parts, "&")
}

func awsURLEncode(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(url.QueryEscape(value), "+", "%20"), "%7E", "~")
}

func deriveSigningKey(secretAccessKey, date, region, service string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+secretAccessKey), date)
	regionKey := hmacSHA256(dateKey, region)
	serviceKey := hmacSHA256(regionKey, service)
	return hmacSHA256(serviceKey, "aws4_request")
}

func hmacSHA256(key []byte, value string) []byte {
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write([]byte(value))
	return digest.Sum(nil)
}

func hexHMAC(key []byte, value string) string {
	return hex.EncodeToString(hmacSHA256(key, value))
}

func hexHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func hashFile(file *os.File) (string, error) {
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func cleanPrefix(value string) (string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), `\`, `/`)
	normalized = strings.Trim(normalized, `/`)
	if normalized == "" {
		return "", nil
	}
	parts := strings.Split(normalized, `/`)
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", fmt.Errorf("S3 object prefix traversal is not allowed")
		}
		for _, character := range part {
			if unicode.IsControl(character) {
				return "", fmt.Errorf("S3 object prefix contains a control character")
			}
		}
		segments = append(segments, part)
	}
	return strings.Join(segments, `/`), nil
}

func cleanFileName(value string) (string, error) {
	name, err := cleanObjectName(value)
	if err != nil {
		return "", fmt.Errorf("invalid remote backup file name")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		return "", fmt.Errorf("remote backup file must use .zip suffix")
	}
	return name, nil
}

func cleanMetadataFileName(value string) (string, error) {
	name, err := cleanObjectName(value)
	if err != nil {
		return "", fmt.Errorf("invalid remote backup metadata file name")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".json") {
		return "", fmt.Errorf("remote backup metadata file must use .json suffix")
	}
	return name, nil
}

func cleanObjectName(value string) (string, error) {
	name := strings.TrimSpace(strings.ReplaceAll(value, `\`, `/`))
	if name == "" || name == "." || name == ".." || strings.Contains(name, `/`) {
		return "", fmt.Errorf("invalid remote object name")
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return "", fmt.Errorf("remote object name contains a control character")
		}
	}
	return name, nil
}

func parseModifiedAt(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
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

func responseError(response *http.Response, operation string) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	detail := compactResponseBody(string(body))
	if detail != "" {
		return fmt.Errorf("%s returned HTTP %s: %s", operation, response.Status, detail)
	}
	return fmt.Errorf("%s returned HTTP %s", operation, response.Status)
}

func compactResponseBody(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
