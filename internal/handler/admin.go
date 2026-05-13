package handler

import (
	"net/http"
	"strconv"
	"strings"

	"channel-adapter-gateway/internal/model"
	"channel-adapter-gateway/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db    *gorm.DB
	admin *service.AdminService
}

func NewAdminHandler(db *gorm.DB, admin *service.AdminService) *AdminHandler {
	return &AdminHandler{db: db, admin: admin}
}

func (h *AdminHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	token, user, err := h.admin.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

func (h *AdminHandler) Me(c *gin.Context) {
	claims, _ := c.Get("claims")
	c.JSON(http.StatusOK, claims)
}

func (h *AdminHandler) ListProviders(c *gin.Context) {
	var rows []model.Provider
	if err := h.db.Order("id desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *AdminHandler) CreateProvider(c *gin.Context) {
	var row model.Provider
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	normalizeProvider(&row)
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	h.refreshCache(c)
	c.JSON(http.StatusOK, row)
}

func (h *AdminHandler) UpdateProvider(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var row model.Provider
	if err := h.db.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "provider not found"})
		return
	}
	var req model.Provider
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	normalizeProvider(&req)
	row.Code = req.Code
	row.Name = req.Name
	row.Type = req.Type
	row.BaseURL = req.BaseURL
	row.APIKey = req.APIKey
	row.Enabled = req.Enabled
	row.TimeoutSeconds = req.TimeoutSeconds
	row.ExtraJSON = req.ExtraJSON
	if err := h.db.Save(&row).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	h.refreshCache(c)
	c.JSON(http.StatusOK, row)
}

func (h *AdminHandler) DeleteProvider(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.db.Delete(&model.Provider{}, id).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	h.refreshCache(c)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	var rows []model.User
	if err := h.db.Order("id desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Enabled  bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "username and password are required"})
		return
	}
	if req.Role == "" {
		req.Role = "admin"
	}
	hash, err := service.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	row := model.User{
		Username:     req.Username,
		PasswordHash: hash,
		Role:         req.Role,
		Enabled:      req.Enabled,
	}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var row model.User
	if err := h.db.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "user not found"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Enabled  bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	row.Username = strings.TrimSpace(req.Username)
	row.Role = defaultString(strings.TrimSpace(req.Role), "admin")
	row.Enabled = req.Enabled
	if strings.TrimSpace(req.Password) != "" {
		hash, err := service.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		row.PasswordHash = hash
	}
	if err := h.db.Save(&row).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.db.Delete(&model.User{}, id).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) ListMappings(c *gin.Context) {
	var rows []model.MappingRule
	if err := h.db.Order("id desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *AdminHandler) CreateMapping(c *gin.Context) {
	var row model.MappingRule
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	normalizeMapping(&row)
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	h.refreshCache(c)
	c.JSON(http.StatusOK, row)
}

func (h *AdminHandler) UpdateMapping(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var row model.MappingRule
	if err := h.db.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "mapping not found"})
		return
	}
	var req model.MappingRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	normalizeMapping(&req)
	row.Name = req.Name
	row.PublicModel = req.PublicModel
	row.TargetProtocol = req.TargetProtocol
	row.TargetEndpoint = req.TargetEndpoint
	row.ProviderCode = req.ProviderCode
	row.UpstreamModel = req.UpstreamModel
	row.UpstreamModelField = req.UpstreamModelField
	row.UpstreamMethod = req.UpstreamMethod
	row.UpstreamPath = req.UpstreamPath
	row.BodyMode = req.BodyMode
	row.FieldMapJSON = req.FieldMapJSON
	row.FileFieldMapJSON = req.FileFieldMapJSON
	row.DefaultsJSON = req.DefaultsJSON
	row.IgnoreFieldsJSON = req.IgnoreFieldsJSON
	row.HeaderMapJSON = req.HeaderMapJSON
	row.ResponseFieldMapJSON = req.ResponseFieldMapJSON
	row.ResponseDefaultsJSON = req.ResponseDefaultsJSON
	row.NormalizeOpenAIUsage = req.NormalizeOpenAIUsage
	row.Enabled = req.Enabled
	row.ExtraJSON = req.ExtraJSON
	if err := h.db.Save(&row).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	h.refreshCache(c)
	c.JSON(http.StatusOK, row)
}

func (h *AdminHandler) DeleteMapping(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.db.Delete(&model.MappingRule{}, id).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	h.refreshCache(c)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AdminHandler) ListLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []model.RequestLog
	if err := h.db.Order("id desc").Limit(limit).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *AdminHandler) refreshCache(c *gin.Context) {
	if err := h.admin.RefreshMappings(); err != nil {
		c.Header("X-Mapping-Refresh-Error", err.Error())
	}
}

func normalizeProvider(row *model.Provider) {
	row.Code = strings.TrimSpace(row.Code)
	row.Name = strings.TrimSpace(row.Name)
	row.Type = strings.TrimSpace(row.Type)
	row.BaseURL = strings.TrimRight(strings.TrimSpace(row.BaseURL), "/")
	row.ExtraJSON = normalizeJSONObject(row.ExtraJSON)
	if row.TimeoutSeconds <= 0 {
		row.TimeoutSeconds = 180
	}
}

func normalizeMapping(row *model.MappingRule) {
	row.Name = strings.TrimSpace(row.Name)
	row.PublicModel = strings.TrimSpace(row.PublicModel)
	row.TargetProtocol = defaultString(strings.TrimSpace(row.TargetProtocol), model.TargetProtocolOpenAI)
	row.TargetEndpoint = strings.TrimSpace(row.TargetEndpoint)
	row.ProviderCode = strings.TrimSpace(row.ProviderCode)
	row.UpstreamModel = strings.TrimSpace(row.UpstreamModel)
	row.UpstreamModelField = defaultString(strings.TrimSpace(row.UpstreamModelField), "model")
	row.UpstreamMethod = defaultString(strings.TrimSpace(row.UpstreamMethod), http.MethodPost)
	row.UpstreamPath = strings.TrimSpace(row.UpstreamPath)
	row.BodyMode = defaultString(strings.TrimSpace(row.BodyMode), model.BodyModeJSON)
	row.FieldMapJSON = normalizeJSONObject(row.FieldMapJSON)
	row.FileFieldMapJSON = normalizeJSONObject(row.FileFieldMapJSON)
	row.DefaultsJSON = normalizeJSONObject(row.DefaultsJSON)
	row.IgnoreFieldsJSON = normalizeJSONArray(row.IgnoreFieldsJSON)
	row.HeaderMapJSON = normalizeJSONObject(row.HeaderMapJSON)
	row.ResponseFieldMapJSON = normalizeJSONObject(row.ResponseFieldMapJSON)
	row.ResponseDefaultsJSON = normalizeJSONObject(row.ResponseDefaultsJSON)
	row.ExtraJSON = normalizeJSONObject(row.ExtraJSON)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func normalizeJSONObject(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "{}"
	}
	return value
}

func normalizeJSONArray(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "[]"
	}
	return value
}
