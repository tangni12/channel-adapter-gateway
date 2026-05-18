package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"channel-adapter-gateway/internal/config"
	"channel-adapter-gateway/internal/model"
	"channel-adapter-gateway/internal/official"
	"channel-adapter-gateway/internal/transform"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ProxyService struct {
	cache              *MappingCache
	defaultTimeout     int
	requestLogger      *RequestLogger
	maxLogPayloadBytes int
	httpClient         *http.Client
}

type requestLogPayload struct {
	upstreamURL      string
	status           int
	traceID          string
	errorMessage     string
	officialRequest  []byte
	upstreamRequest  []byte
	upstreamResponse []byte
	officialResponse []byte
	usage            []byte
}

const consoleRequestLogMaxBytes = 8192

func NewProxyService(db *gorm.DB, cache *MappingCache, defaultTimeout int, requestLogger *RequestLogger) *ProxyService {
	if defaultTimeout <= 0 {
		defaultTimeout = 180
	}
	if requestLogger == nil {
		requestLogger = NewRequestLogger(db, defaultLoggingConfig())
	}
	return &ProxyService{
		cache:              cache,
		defaultTimeout:     defaultTimeout,
		requestLogger:      requestLogger,
		maxLogPayloadBytes: requestLogger.cfg.MaxPayloadBytes,
		httpClient:         newPooledHTTPClient(),
	}
}

func (s *ProxyService) ListModels(c *gin.Context) {
	data := make([]gin.H, 0)
	for _, modelName := range s.cache.Models() {
		data = append(data, gin.H{
			"id":       modelName,
			"object":   "model",
			"owned_by": "channel-adapter-gateway",
		})
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

func (s *ProxyService) OpenAIImageGeneration(c *gin.Context) {
	start := time.Now()
	requestID := requestID(c)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, openAIError("invalid_request", "read request body failed"))
		return
	}
	publicModel, err := extractJSONModel(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, openAIError("invalid_request", err.Error()))
		return
	}
	logIncomingRequest(c.Request.URL.Path, requestID, publicModel, model.EndpointOpenAIImagesGenerations, body)
	rule, upstreamProvider, err := s.cache.Find(model.TargetProtocolOpenAI, model.EndpointOpenAIImagesGenerations, publicModel)
	if err != nil {
		c.JSON(http.StatusBadRequest, openAIError("mapping_not_found", err.Error()))
		return
	}
	upReq, err := transform.BuildJSON(upstreamProvider, rule, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, openAIError("convert_request_failed", err.Error()))
		return
	}
	s.forward(c, start, requestID, publicModel, rule, upstreamProvider, upReq)
}

func (s *ProxyService) OpenAIImageEdit(c *gin.Context) {
	start := time.Now()
	requestID := requestID(c)
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, openAIError("invalid_request", "read request body failed"))
			return
		}
		publicModel, err := extractJSONModel(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, openAIError("invalid_request", err.Error()))
			return
		}
		logIncomingRequest(c.Request.URL.Path, requestID, publicModel, model.EndpointOpenAIImagesEdits, body)
		rule, upstreamProvider, err := s.cache.Find(model.TargetProtocolOpenAI, model.EndpointOpenAIImagesEdits, publicModel)
		if err != nil {
			c.JSON(http.StatusBadRequest, openAIError("mapping_not_found", err.Error()))
			return
		}
		upReq, err := transform.BuildJSON(upstreamProvider, rule, body)
		if err != nil {
			c.JSON(http.StatusBadRequest, openAIError("convert_request_failed", err.Error()))
			return
		}
		s.forward(c, start, requestID, publicModel, rule, upstreamProvider, upReq)
		return
	}
	publicModel := strings.TrimSpace(c.PostForm("model"))
	if publicModel == "" {
		c.JSON(http.StatusBadRequest, openAIError("invalid_request", "model is required"))
		return
	}
	rule, upstreamProvider, err := s.cache.Find(model.TargetProtocolOpenAI, model.EndpointOpenAIImagesEdits, publicModel)
	if err != nil {
		c.JSON(http.StatusBadRequest, openAIError("mapping_not_found", err.Error()))
		return
	}
	upReq, err := transform.BuildMultipart(c.Request, upstreamProvider, rule)
	if err != nil {
		c.JSON(http.StatusBadRequest, openAIError("convert_request_failed", err.Error()))
		return
	}
	logIncomingRequest(c.Request.URL.Path, requestID, publicModel, model.EndpointOpenAIImagesEdits, upReq.OfficialSnapshot)
	s.forward(c, start, requestID, publicModel, rule, upstreamProvider, upReq)
}

func (s *ProxyService) forward(c *gin.Context, start time.Time, requestID string, publicModel string, rule model.MappingRule, provider model.Provider, upReq *transform.UpstreamRequest) {
	timeout := time.Duration(s.defaultTimeout) * time.Second
	if provider.TimeoutSeconds > 0 {
		timeout = time.Duration(provider.TimeoutSeconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, upReq.Method, upReq.URL, upReq.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, openAIError("build_upstream_request_failed", err.Error()))
		return
	}
	req.Header.Set("Content-Type", upReq.ContentType)
	req.Header.Set("Authorization", s.upstreamAuthorization(c, provider))
	if requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logRequest(c, requestID, start, publicModel, rule, provider, requestLogPayload{
			upstreamURL:     upReq.URL,
			errorMessage:    err.Error(),
			officialRequest: upReq.OfficialSnapshot,
			upstreamRequest: upReq.UpstreamSnapshot,
		})
		c.JSON(http.StatusBadGateway, openAIError("upstream_request_failed", err.Error()))
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logRequest(c, requestID, start, publicModel, rule, provider, requestLogPayload{
			upstreamURL:     upReq.URL,
			status:          resp.StatusCode,
			traceID:         resp.Header.Get("Trace-Id"),
			errorMessage:    err.Error(),
			officialRequest: upReq.OfficialSnapshot,
			upstreamRequest: upReq.UpstreamSnapshot,
		})
		c.JSON(http.StatusBadGateway, openAIError("read_upstream_response_failed", err.Error()))
		return
	}

	outBody := respBody
	var usage []byte
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		mappedBody, mappedUsage, err := transform.MapResponse(respBody, rule.ResponseFieldMapJSON, rule.ResponseDefaultsJSON, responseSpecs(rule.TargetEndpoint))
		if err != nil {
			s.logRequest(c, requestID, start, publicModel, rule, provider, requestLogPayload{
				upstreamURL:      upReq.URL,
				status:           resp.StatusCode,
				traceID:          resp.Header.Get("Trace-Id"),
				errorMessage:     err.Error(),
				officialRequest:  upReq.OfficialSnapshot,
				upstreamRequest:  upReq.UpstreamSnapshot,
				upstreamResponse: respBody,
			})
			c.JSON(http.StatusBadGateway, openAIError("map_response_failed", err.Error()))
			return
		}
		outBody = mappedBody
		usage = mappedUsage
	}
	if rule.TargetProtocol == model.TargetProtocolOpenAI && rule.NormalizeOpenAIUsage {
		outBody, usage, err = transform.NormalizeOpenAIUsageForEndpoint(outBody, rule.TargetEndpoint)
		if err != nil {
			s.logRequest(c, requestID, start, publicModel, rule, provider, requestLogPayload{
				upstreamURL:      upReq.URL,
				status:           resp.StatusCode,
				traceID:          resp.Header.Get("Trace-Id"),
				errorMessage:     err.Error(),
				officialRequest:  upReq.OfficialSnapshot,
				upstreamRequest:  upReq.UpstreamSnapshot,
				upstreamResponse: respBody,
				officialResponse: outBody,
			})
			c.JSON(http.StatusBadGateway, openAIError("normalize_response_failed", err.Error()))
			return
		}
	}

	for _, key := range []string{"Trace-Id", "X-Request-Id"} {
		if value := resp.Header.Get(key); value != "" {
			c.Header(key, value)
		}
	}
	s.logRequest(c, requestID, start, publicModel, rule, provider, requestLogPayload{
		upstreamURL:      upReq.URL,
		status:           resp.StatusCode,
		traceID:          resp.Header.Get("Trace-Id"),
		officialRequest:  upReq.OfficialSnapshot,
		upstreamRequest:  upReq.UpstreamSnapshot,
		upstreamResponse: respBody,
		officialResponse: outBody,
		usage:            usage,
	})
	c.Data(resp.StatusCode, "application/json", outBody)
}

func (s *ProxyService) upstreamAuthorization(c *gin.Context, provider model.Provider) string {
	if strings.TrimSpace(provider.APIKey) != "" {
		return "Bearer " + strings.TrimSpace(provider.APIKey)
	}
	return c.GetHeader("Authorization")
}

func (s *ProxyService) logRequest(c *gin.Context, requestID string, start time.Time, publicModel string, rule model.MappingRule, provider model.Provider, payload requestLogPayload) {
	logRow := model.RequestLog{
		RequestID:      requestID,
		Method:         c.Request.Method,
		Path:           c.Request.URL.Path,
		PublicModel:    publicModel,
		TargetProtocol: rule.TargetProtocol,
		TargetEndpoint: rule.TargetEndpoint,
		UpstreamModel:  rule.UpstreamModel,
		ProviderCode:   provider.Code,
		UpstreamURL:    payload.upstreamURL,
		StatusCode:     payload.status,
		LatencyMs:      time.Since(start).Milliseconds(),
		TraceID:        payload.traceID,
		ErrorMessage:   payload.errorMessage,
		RequestSnapshot: jsonbString(
			transform.SanitizeLogPayload(rule.TargetEndpoint, transform.LogTargetUpstreamRequest, payload.upstreamRequest),
			s.maxLogPayloadBytes,
		),
		OfficialRequest: jsonbString(
			transform.SanitizeLogPayload(rule.TargetEndpoint, transform.LogTargetOfficialRequest, payload.officialRequest),
			s.maxLogPayloadBytes,
		),
		UpstreamRequest: jsonbString(
			transform.SanitizeLogPayload(rule.TargetEndpoint, transform.LogTargetUpstreamRequest, payload.upstreamRequest),
			s.maxLogPayloadBytes,
		),
		UpstreamResponse: jsonbString(
			transform.SanitizeLogPayload(rule.TargetEndpoint, transform.LogTargetUpstreamResponse, payload.upstreamResponse),
			s.maxLogPayloadBytes,
		),
		OfficialResponse: jsonbString(
			transform.SanitizeLogPayload(rule.TargetEndpoint, transform.LogTargetOfficialResponse, payload.officialResponse),
			s.maxLogPayloadBytes,
		),
		ResponseUsage: jsonbString(
			transform.SanitizeLogPayload(rule.TargetEndpoint, transform.LogTargetResponseUsage, payload.usage),
			s.maxLogPayloadBytes,
		),
	}
	s.requestLogger.Enqueue(logRow)
}

func defaultLoggingConfig() config.LoggingConfig {
	return config.LoggingConfig{
		AsyncRequestLog:       boolPtr(true),
		QueueSize:             2000,
		WorkerCount:           2,
		BatchSize:             50,
		FlushIntervalMs:       1000,
		MaxRetries:            3,
		RetryIntervalMs:       300,
		EnqueueTimeoutMs:      10,
		MaxPayloadBytes:       10 * 1024 * 1024,
		LogDroppedWhenFull:    true,
		SyncOnShutdownSeconds: 5,
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func newPooledHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          200,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func jsonbString(body []byte, maxBytes int) string {
	if len(body) == 0 {
		return "null"
	}
	if maxBytes > 0 && len(body) > maxBytes {
		fallback, _ := json.Marshal(map[string]any{
			"truncated": true,
			"size":      len(body),
			"raw":       string(body[:maxBytes]),
		})
		return string(fallback)
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err == nil {
		normalized, marshalErr := json.Marshal(payload)
		if marshalErr == nil {
			return string(normalized)
		}
	}
	fallback, _ := json.Marshal(map[string]any{"raw": string(body)})
	return string(fallback)
}

func logIncomingRequest(path, requestID, publicModel, endpoint string, body []byte) {
	sanitized := transform.SanitizeLogPayload(endpoint, transform.LogTargetOfficialRequest, body)
	payload := jsonbString(sanitized, consoleRequestLogMaxBytes)
	if len(payload) > consoleRequestLogMaxBytes {
		payload = payload[:consoleRequestLogMaxBytes] + "...(truncated)"
	}
	log.Printf("incoming request params path=%s request_id=%s model=%s body=%s", path, requestID, publicModel, payload)
}

func extractJSONModel(body []byte) (string, error) {
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("invalid json body")
	}
	if strings.TrimSpace(payload.Model) == "" {
		return "", fmt.Errorf("model is required")
	}
	return payload.Model, nil
}

func requestID(c *gin.Context) string {
	for _, key := range []string{"X-Request-Id", "X-Newapi-Request-Id", "Trace-Id"} {
		if value := c.GetHeader(key); value != "" {
			return value
		}
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func openAIError(code, message string) gin.H {
	return gin.H{
		"error": gin.H{
			"message": message,
			"type":    "channel_adapter_gateway_error",
			"code":    code,
		},
	}
}

func responseSpecs(endpointKey string) []transform.ResponseFieldSpec {
	endpoint, ok := official.FindEndpoint(endpointKey)
	if !ok {
		return nil
	}
	specs := make([]transform.ResponseFieldSpec, 0, len(endpoint.ResponseFields))
	for _, field := range endpoint.ResponseFields {
		specs = append(specs, transform.ResponseFieldSpec{Name: field.Name, Type: field.Type, Required: field.Required})
	}
	return specs
}
