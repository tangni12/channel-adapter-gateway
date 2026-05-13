package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"channel-adapter-gateway/internal/model"
	"channel-adapter-gateway/internal/official"
	"channel-adapter-gateway/internal/transform"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ProxyService struct {
	db             *gorm.DB
	cache          *MappingCache
	defaultTimeout int
}

func NewProxyService(db *gorm.DB, cache *MappingCache, defaultTimeout int) *ProxyService {
	if defaultTimeout <= 0 {
		defaultTimeout = 180
	}
	return &ProxyService{db: db, cache: cache, defaultTimeout: defaultTimeout}
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

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		s.logRequest(c, requestID, start, publicModel, rule, provider, upReq.URL, 0, "", err.Error(), upReq.Snapshot, nil)
		c.JSON(http.StatusBadGateway, openAIError("upstream_request_failed", err.Error()))
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logRequest(c, requestID, start, publicModel, rule, provider, upReq.URL, resp.StatusCode, resp.Header.Get("Trace-Id"), err.Error(), upReq.Snapshot, nil)
		c.JSON(http.StatusBadGateway, openAIError("read_upstream_response_failed", err.Error()))
		return
	}

	outBody := respBody
	var usage []byte
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		mappedBody, mappedUsage, err := transform.MapResponse(respBody, rule.ResponseFieldMapJSON, rule.ResponseDefaultsJSON, responseSpecs(rule.TargetEndpoint))
		if err != nil {
			s.logRequest(c, requestID, start, publicModel, rule, provider, upReq.URL, resp.StatusCode, resp.Header.Get("Trace-Id"), err.Error(), upReq.Snapshot, nil)
			c.JSON(http.StatusBadGateway, openAIError("map_response_failed", err.Error()))
			return
		}
		outBody = mappedBody
		usage = mappedUsage
	}
	if rule.TargetProtocol == model.TargetProtocolOpenAI && rule.NormalizeOpenAIUsage {
		outBody, usage, err = transform.NormalizeOpenAIUsage(outBody)
		if err != nil {
			s.logRequest(c, requestID, start, publicModel, rule, provider, upReq.URL, resp.StatusCode, resp.Header.Get("Trace-Id"), err.Error(), upReq.Snapshot, nil)
			c.JSON(http.StatusBadGateway, openAIError("normalize_response_failed", err.Error()))
			return
		}
	}

	for _, key := range []string{"Trace-Id", "X-Request-Id"} {
		if value := resp.Header.Get(key); value != "" {
			c.Header(key, value)
		}
	}
	s.logRequest(c, requestID, start, publicModel, rule, provider, upReq.URL, resp.StatusCode, resp.Header.Get("Trace-Id"), "", upReq.Snapshot, usage)
	c.Data(resp.StatusCode, "application/json", outBody)
}

func (s *ProxyService) upstreamAuthorization(c *gin.Context, provider model.Provider) string {
	if strings.TrimSpace(provider.APIKey) != "" {
		return "Bearer " + strings.TrimSpace(provider.APIKey)
	}
	return c.GetHeader("Authorization")
}

func (s *ProxyService) logRequest(c *gin.Context, requestID string, start time.Time, publicModel string, rule model.MappingRule, provider model.Provider, upstreamURL string, status int, traceID, errorMessage string, snapshot []byte, usage []byte) {
	logRow := model.RequestLog{
		RequestID:       requestID,
		Method:          c.Request.Method,
		Path:            c.Request.URL.Path,
		PublicModel:     publicModel,
		TargetProtocol:  rule.TargetProtocol,
		TargetEndpoint:  rule.TargetEndpoint,
		UpstreamModel:   rule.UpstreamModel,
		ProviderCode:    provider.Code,
		UpstreamURL:     upstreamURL,
		StatusCode:      status,
		LatencyMs:       time.Since(start).Milliseconds(),
		TraceID:         traceID,
		ErrorMessage:    errorMessage,
		RequestSnapshot: string(snapshot),
		ResponseUsage:   string(usage),
	}
	_ = s.db.Create(&logRow).Error
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
