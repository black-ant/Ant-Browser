package cdp

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"mime"
	"net/url"
	"regexp"
	"strings"
)

// ResponseData 结构化的响应数据
type ResponseData struct {
	Raw        string                 `json:"raw"`        // 原始内容
	Type       string                 `json:"type"`       // 数据类型：json/xml/html/image/binary/text/form/graphql/protobuf
	Structured interface{}            `json:"structured"` // 结构化数据（JSON对象、XML树等）
	Preview    string                 `json:"preview"`    // 预览文本（截断）
	Size       int                    `json:"size"`       // 字节大小
	Encoding   string                 `json:"encoding"`   // 编码格式
	Error      string                 `json:"error"`      // 解析错误
	Metadata   map[string]interface{} `json:"metadata"`   // 额外元数据
}

// DataParser 数据解析器
type DataParser struct{}

// NewDataParser 创建解析器
func NewDataParser() *DataParser {
	return &DataParser{}
}

// Parse 解析响应体
func (p *DataParser) Parse(body string, contentType string, url string) *ResponseData {
	result := &ResponseData{
		Raw:      body,
		Size:     len(body),
		Metadata: make(map[string]interface{}),
	}

	// 1. 首先根据 Content-Type 判断
	dataType := p.detectTypeFromContentType(contentType)

	// 2. 如果 Content-Type 不明确，尝试内容嗅探
	if dataType == "unknown" || dataType == "text" {
		dataType = p.sniffContentType(body, url)
	}

	result.Type = dataType
	result.Preview = p.generatePreview(body, dataType)

	// 3. 根据类型进行结构化解析
	switch dataType {
	case "json":
		p.parseJSON(body, result)
	case "xml":
		p.parseXML(body, result)
	case "html":
		p.parseHTML(body, result)
	case "form":
		p.parseFormData(body, result)
	case "graphql":
		p.parseGraphQL(body, result)
	case "image":
		p.parseImage(body, contentType, result)
	case "css":
		p.parseCSS(body, result)
	case "javascript":
		p.parseJavaScript(body, result)
	default:
		// text 或 binary 保持原样
		result.Structured = body
	}

	return result
}

// detectTypeFromContentType 从 Content-Type 推断类型
func (p *DataParser) detectTypeFromContentType(contentType string) string {
	if contentType == "" {
		return "unknown"
	}

	// 解析 MIME 类型
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "unknown"
	}

	switch {
	case strings.Contains(mediaType, "json"):
		return "json"
	case strings.Contains(mediaType, "xml"):
		return "xml"
	case strings.Contains(mediaType, "html"):
		return "html"
	case strings.Contains(mediaType, "javascript") || strings.Contains(mediaType, "ecmascript"):
		return "javascript"
	case strings.Contains(mediaType, "css"):
		return "css"
	case strings.Contains(mediaType, "form-urlencoded"):
		return "form"
	case strings.Contains(mediaType, "form-data"):
		return "multipart"
	case strings.HasPrefix(mediaType, "image/"):
		return "image"
	case strings.HasPrefix(mediaType, "video/"):
		return "video"
	case strings.HasPrefix(mediaType, "audio/"):
		return "audio"
	case strings.HasPrefix(mediaType, "application/octet-stream"):
		return "binary"
	case strings.HasPrefix(mediaType, "text/"):
		return "text"
	case strings.Contains(mediaType, "protobuf"):
		return "protobuf"
	case strings.Contains(mediaType, "grpc"):
		return "grpc"
	default:
		return "unknown"
	}
}

// sniffContentType 内容嗅探（当 Content-Type 不可靠时）
func (p *DataParser) sniffContentType(body string, url string) string {
	if len(body) == 0 {
		return "empty"
	}

	trimmed := strings.TrimSpace(body)

	// GraphQL 检测（在 JSON 之前，因为 GraphQL 也是 JSON）
	if p.looksLikeGraphQL(trimmed) {
		return "graphql"
	}

	// JSON 检测
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		var js interface{}
		if err := json.Unmarshal([]byte(trimmed), &js); err == nil {
			return "json"
		}
	}

	// XML 检测
	if strings.HasPrefix(trimmed, "<?xml") || strings.HasPrefix(trimmed, "<") {
		return "xml"
	}

	// HTML 检测
	if strings.Contains(strings.ToLower(trimmed), "<!doctype html") ||
		strings.Contains(strings.ToLower(trimmed), "<html") {
		return "html"
	}

	// URL-encoded form 检测
	if p.looksLikeFormData(trimmed) {
		return "form"
	}

	// Base64 图片检测
	if strings.HasPrefix(trimmed, "data:image/") {
		return "image"
	}

	// 二进制检测（包含不可打印字符）
	if p.isBinary(body) {
		return "binary"
	}

	// 默认文本
	return "text"
}

// parseJSON 解析 JSON
func (p *DataParser) parseJSON(body string, result *ResponseData) {
	var data interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		result.Error = fmt.Sprintf("JSON 解析失败: %v", err)
		result.Structured = body
		return
	}

	result.Structured = data

	// 提取元数据
	if obj, ok := data.(map[string]interface{}); ok {
		result.Metadata["keys"] = len(obj)
		result.Metadata["topLevelKeys"] = p.extractKeys(obj, 10)
	} else if arr, ok := data.([]interface{}); ok {
		result.Metadata["arrayLength"] = len(arr)
	}
}

// parseXML 解析 XML
func (p *DataParser) parseXML(body string, result *ResponseData) {
	var data interface{}
	if err := xml.Unmarshal([]byte(body), &data); err != nil {
		result.Error = fmt.Sprintf("XML 解析失败: %v", err)
		result.Structured = body
		return
	}
	result.Structured = data
}

// parseHTML 解析 HTML（提取关键信息）
func (p *DataParser) parseHTML(body string, result *ResponseData) {
	metadata := make(map[string]interface{})

	// 提取 title
	titleRe := regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
	if matches := titleRe.FindStringSubmatch(body); len(matches) > 1 {
		metadata["title"] = strings.TrimSpace(matches[1])
	}

	// 统计标签数量
	tagRe := regexp.MustCompile(`<(\w+)`)
	tags := tagRe.FindAllStringSubmatch(body, -1)
	tagCount := make(map[string]int)
	for _, match := range tags {
		if len(match) > 1 {
			tagCount[match[1]]++
		}
	}
	metadata["tagCounts"] = tagCount
	metadata["totalTags"] = len(tags)

	result.Metadata = metadata
	result.Structured = body // HTML 保持原样，前端用 iframe 或高亮显示
}

// parseFormData 解析表单数据
func (p *DataParser) parseFormData(body string, result *ResponseData) {
	values, err := url.ParseQuery(body)
	if err != nil {
		result.Error = fmt.Sprintf("表单解析失败: %v", err)
		result.Structured = body
		return
	}

	// 转换为 map
	formMap := make(map[string]interface{})
	for k, v := range values {
		if len(v) == 1 {
			formMap[k] = v[0]
		} else {
			formMap[k] = v
		}
	}

	result.Structured = formMap
	result.Metadata["fieldCount"] = len(formMap)
}

// parseGraphQL 解析 GraphQL 响应
func (p *DataParser) parseGraphQL(body string, result *ResponseData) {
	var gqlResponse map[string]interface{}
	if err := json.Unmarshal([]byte(body), &gqlResponse); err != nil {
		result.Error = fmt.Sprintf("GraphQL 解析失败: %v", err)
		result.Structured = body
		return
	}

	result.Structured = gqlResponse

	// 提取 GraphQL 特有元数据
	if data, ok := gqlResponse["data"]; ok {
		result.Metadata["hasData"] = true
		result.Metadata["dataKeys"] = p.extractKeys(data, 10)
	}
	if errors, ok := gqlResponse["errors"]; ok {
		result.Metadata["hasErrors"] = true
		if errArr, ok := errors.([]interface{}); ok {
			result.Metadata["errorCount"] = len(errArr)
		}
	}
}

// parseImage 解析图片信息
func (p *DataParser) parseImage(body string, contentType string, result *ResponseData) {
	// 如果是 base64，先解码
	var imageData string
	if strings.HasPrefix(body, "data:image/") {
		parts := strings.SplitN(body, ",", 2)
		if len(parts) == 2 {
			imageData = parts[1]
		}
	} else {
		imageData = base64.StdEncoding.EncodeToString([]byte(body))
	}

	result.Structured = map[string]interface{}{
		"base64":      imageData,
		"contentType": contentType,
		"sizeBytes":   len(body),
	}

	result.Metadata["isImage"] = true
	result.Metadata["format"] = strings.TrimPrefix(contentType, "image/")
}

// parseCSS 解析 CSS
func (p *DataParser) parseCSS(body string, result *ResponseData) {
	// 统计规则数量
	ruleRe := regexp.MustCompile(`\{[^}]*\}`)
	rules := ruleRe.FindAllString(body, -1)

	result.Metadata["ruleCount"] = len(rules)
	result.Structured = body
}

// parseJavaScript 解析 JavaScript
func (p *DataParser) parseJavaScript(body string, result *ResponseData) {
	// 检测是否压缩
	isMinified := !strings.Contains(body, "\n") && len(body) > 1000

	result.Metadata["isMinified"] = isMinified
	result.Metadata["lineCount"] = strings.Count(body, "\n") + 1
	result.Structured = body
}

// 辅助函数

func (p *DataParser) looksLikeGraphQL(body string) bool {
	// GraphQL 响应通常有 "data" 和/或 "errors" 字段
	if !strings.HasPrefix(body, "{") {
		return false
	}
	return strings.Contains(body, `"data"`) || strings.Contains(body, `"errors"`)
}

func (p *DataParser) looksLikeFormData(body string) bool {
	// 简单检测：包含 key=value&key=value 模式
	if strings.Contains(body, "=") && (strings.Contains(body, "&") || !strings.Contains(body, " ")) {
		_, err := url.ParseQuery(body)
		return err == nil
	}
	return false
}

func (p *DataParser) isBinary(body string) bool {
	// 检查前 512 字节是否包含不可打印字符
	checkLen := 512
	if len(body) < checkLen {
		checkLen = len(body)
	}

	nonPrintable := 0
	for i := 0; i < checkLen; i++ {
		b := body[i]
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			nonPrintable++
		}
	}

	// 超过 30% 不可打印字符，认为是二进制
	return float64(nonPrintable)/float64(checkLen) > 0.3
}

func (p *DataParser) generatePreview(body string, dataType string) string {
	const maxLen = 200
	if len(body) <= maxLen {
		return body
	}

	preview := body[:maxLen]
	if dataType == "json" || dataType == "xml" {
		// 尝试在合适的位置截断
		if lastComma := strings.LastIndex(preview, ","); lastComma > maxLen/2 {
			preview = preview[:lastComma]
		}
	}

	return preview + "..."
}

func (p *DataParser) extractKeys(data interface{}, limit int) []string {
	if obj, ok := data.(map[string]interface{}); ok {
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
			if len(keys) >= limit {
				break
			}
		}
		return keys
	}
	return nil
}
