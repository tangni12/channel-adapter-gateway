package transform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"

	"channel-adapter-gateway/internal/model"
)

type UpstreamRequest struct {
	Method      string
	URL         string
	ContentType string
	Body        io.Reader
	Snapshot    []byte
}

func BuildJSON(provider model.Provider, rule model.MappingRule, body []byte) (*UpstreamRequest, error) {
	var source map[string]any
	if err := json.Unmarshal(body, &source); err != nil {
		return nil, fmt.Errorf("invalid json body: %w", err)
	}

	target := make(map[string]any)
	fieldMap := parseStringMap(rule.FieldMapJSON)
	ignore := parseStringSet(rule.IgnoreFieldsJSON)

	for key, value := range source {
		if ignore[key] {
			continue
		}
		targetKey := fieldMap[key]
		if targetKey == "" {
			targetKey = key
		}
		if targetKey == "-" {
			continue
		}
		setPath(target, targetKey, value)
	}
	applyDefaults(target, rule.DefaultsJSON)
	if rule.UpstreamModel != "" && rule.UpstreamModelField != "" {
		setPath(target, rule.UpstreamModelField, rule.UpstreamModel)
	}

	cleaned, err := json.Marshal(target)
	if err != nil {
		return nil, err
	}
	return &UpstreamRequest{
		Method:      defaultString(rule.UpstreamMethod, http.MethodPost),
		URL:         joinURL(provider.BaseURL, rule.UpstreamPath),
		ContentType: "application/json",
		Body:        bytes.NewReader(cleaned),
		Snapshot:    cleaned,
	}, nil
}

func BuildMultipart(req *http.Request, provider model.Provider, rule model.MappingRule) (*UpstreamRequest, error) {
	if err := req.ParseMultipartForm(128 << 20); err != nil {
		return nil, fmt.Errorf("invalid multipart body: %w", err)
	}

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	fieldMap := parseStringMap(rule.FieldMapJSON)
	fileFieldMap := parseStringMap(rule.FileFieldMapJSON)
	ignore := parseStringSet(rule.IgnoreFieldsJSON)

	if rule.UpstreamModel != "" && rule.UpstreamModelField != "" {
		if err := writer.WriteField(rule.UpstreamModelField, rule.UpstreamModel); err != nil {
			return nil, err
		}
	}

	for key, values := range req.MultipartForm.Value {
		if ignore[key] || key == rule.UpstreamModelField {
			continue
		}
		targetKey := fieldMap[key]
		if targetKey == "" {
			targetKey = key
		}
		if targetKey == "-" {
			continue
		}
		for _, value := range values {
			if err := writer.WriteField(targetKey, value); err != nil {
				return nil, err
			}
		}
	}

	fileCount := 0
	for key, files := range req.MultipartForm.File {
		if ignore[key] {
			continue
		}
		targetKey := fileFieldMap[key]
		if targetKey == "" {
			targetKey = key
		}
		if targetKey == "-" {
			continue
		}
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				return nil, err
			}
			if err := writeFormFile(writer, targetKey, fileHeader.Filename, file); err != nil {
				_ = file.Close()
				return nil, err
			}
			_ = file.Close()
			fileCount++
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}
	snapshot, _ := json.Marshal(map[string]any{
		"body_mode":   model.BodyModeMultipart,
		"model":       rule.UpstreamModel,
		"file_count":  fileCount,
		"field_map":   fieldMap,
		"target_path": rule.UpstreamPath,
	})

	return &UpstreamRequest{
		Method:      defaultString(rule.UpstreamMethod, http.MethodPost),
		URL:         joinURL(provider.BaseURL, rule.UpstreamPath),
		ContentType: writer.FormDataContentType(),
		Body:        bytes.NewReader(buffer.Bytes()),
		Snapshot:    snapshot,
	}, nil
}

func NormalizeOpenAIUsage(body []byte) ([]byte, []byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, nil, nil
	}
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		return body, nil, nil
	}
	_, hasInputTokens := usage["input_tokens"]
	_, hasOutputTokens := usage["output_tokens"]
	if hasInputTokens || hasOutputTokens {
		// 图片接口的官方 usage 使用 input_tokens/output_tokens。
		// NewAPI 会把这两个字段归一化到 prompt_tokens/completion_tokens；
		// 如果网关提前也返回 prompt_tokens/completion_tokens，会导致 NewAPI 再叠加一次。
		delete(usage, "prompt_tokens")
		delete(usage, "completion_tokens")
		if _, exists := usage["total_tokens"]; !exists {
			usage["total_tokens"] = toFloat(usage["input_tokens"]) + toFloat(usage["output_tokens"])
		}
		normalized, err := json.Marshal(payload)
		if err != nil {
			return body, nil, err
		}
		usageBytes, _ := json.Marshal(usage)
		return normalized, usageBytes, nil
	}
	if _, exists := usage["prompt_tokens"]; !exists {
		if value, ok := usage["input_tokens"]; ok {
			usage["prompt_tokens"] = value
		}
	}
	if _, exists := usage["completion_tokens"]; !exists {
		if value, ok := usage["output_tokens"]; ok {
			usage["completion_tokens"] = value
		}
	}
	if _, exists := usage["total_tokens"]; !exists {
		usage["total_tokens"] = toFloat(usage["prompt_tokens"]) + toFloat(usage["completion_tokens"])
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return body, nil, err
	}
	usageBytes, _ := json.Marshal(usage)
	return normalized, usageBytes, nil
}

func parseStringMap(raw string) map[string]string {
	result := make(map[string]string)
	if strings.TrimSpace(raw) == "" {
		return result
	}
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func parseStringSet(raw string) map[string]bool {
	result := make(map[string]bool)
	if strings.TrimSpace(raw) == "" {
		return result
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err == nil {
		for _, item := range list {
			result[item] = true
		}
	}
	return result
}

func applyDefaults(target map[string]any, raw string) {
	if strings.TrimSpace(raw) == "" {
		return
	}
	var defaults map[string]any
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		return
	}
	for key, value := range defaults {
		setPath(target, key, value)
	}
}

func setPath(target map[string]any, pathText string, value any) {
	parts := strings.Split(pathText, ".")
	if len(parts) == 1 {
		target[pathText] = value
		return
	}
	current := target
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func joinURL(baseURL, subPath string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path.Clean(subPath), "/")
}

func writeFormFile(writer *multipart.Writer, field, filename string, reader io.Reader) error {
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, reader)
	return err
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	default:
		return 0
	}
}
