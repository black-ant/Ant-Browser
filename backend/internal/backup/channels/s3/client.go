package s3

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	ControlTimeout = time.Minute
	defaultRegion  = "us-east-1"
	serviceName    = "s3"
)

type Config struct {
	Endpoint        string
	Region          string
	Bucket          string
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

func NewClient(configValue Config) (*Client, error) {
	configValue.Endpoint = strings.TrimSpace(configValue.Endpoint)
	configValue.Region = strings.TrimSpace(configValue.Region)
	if configValue.Region == "" {
		configValue.Region = defaultRegion
	}
	configValue.Bucket = strings.TrimSpace(configValue.Bucket)
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
		httpClient: &http.Client{Timeout: ControlTimeout},
	}, nil
}

func (client *Client) Test(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	target, err := client.bucketURL()
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, target.String(), nil)
	if err != nil {
		return fmt.Errorf("create S3 bucket request failed: %w", err)
	}
	client.signRequest(request, time.Now().UTC())

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request S3 bucket failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		detail := compactResponseBody(string(body))
		if detail != "" {
			return fmt.Errorf("S3 bucket check returned HTTP %s: %s", response.Status, detail)
		}
		return fmt.Errorf("S3 bucket check returned HTTP %s", response.Status)
	}
	return nil
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

func (client *Client) signRequest(request *http.Request, timestamp time.Time) {
	const emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	amzDate := timestamp.UTC().Format("20060102T150405Z")
	date := timestamp.UTC().Format("20060102")
	request.Header.Set("x-amz-content-sha256", emptyPayloadHash)
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
		"x-amz-content-sha256:" + emptyPayloadHash,
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
		emptyPayloadHash,
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

func compactResponseBody(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
