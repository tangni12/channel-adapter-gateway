package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	TargetProtocolOpenAI = "openai"

	EndpointOpenAIImagesGenerations = "openai.images.generations"
	EndpointOpenAIImagesEdits       = "openai.images.edits"

	BodyModeJSON      = "json"
	BodyModeMultipart = "multipart"
)

type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	Username     string         `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	Role         string         `gorm:"size:32;not null;default:admin" json:"role"`
	Enabled      bool           `gorm:"not null;default:true" json:"enabled"`
	LastLoginAt  *time.Time     `json:"last_login_at,omitempty"`
}

type Provider struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Code           string         `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name           string         `gorm:"size:128;not null" json:"name"`
	Type           string         `gorm:"size:64;not null" json:"type"`
	BaseURL        string         `gorm:"size:512;not null" json:"base_url"`
	APIKey         string         `gorm:"size:1024" json:"api_key,omitempty"`
	Enabled        bool           `gorm:"not null;default:true" json:"enabled"`
	TimeoutSeconds int            `gorm:"not null;default:180" json:"timeout_seconds"`
	ExtraJSON      string         `gorm:"type:jsonb;column:extra_json" json:"extra_json,omitempty"`
}

type MappingRule struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Name           string         `gorm:"size:128;not null" json:"name"`
	PublicModel    string         `gorm:"size:128;not null;index:idx_mapping_lookup,priority:1" json:"public_model"`
	TargetProtocol string         `gorm:"size:64;not null;index:idx_mapping_lookup,priority:2" json:"target_protocol"`
	TargetEndpoint string         `gorm:"size:96;not null;index:idx_mapping_lookup,priority:3" json:"target_endpoint"`
	ProviderCode   string         `gorm:"size:64;not null;index" json:"provider_code"`

	UpstreamModel      string `gorm:"size:128;not null" json:"upstream_model"`
	UpstreamModelField string `gorm:"size:64;not null;default:model" json:"upstream_model_field"`
	UpstreamMethod     string `gorm:"size:16;not null;default:POST" json:"upstream_method"`
	UpstreamPath       string `gorm:"size:512;not null" json:"upstream_path"`
	BodyMode           string `gorm:"size:32;not null;default:json" json:"body_mode"`

	FieldMapJSON     string `gorm:"type:jsonb" json:"field_map_json,omitempty"`
	FileFieldMapJSON string `gorm:"type:jsonb" json:"file_field_map_json,omitempty"`
	DefaultsJSON     string `gorm:"type:jsonb" json:"defaults_json,omitempty"`
	IgnoreFieldsJSON string `gorm:"type:jsonb" json:"ignore_fields_json,omitempty"`
	HeaderMapJSON    string `gorm:"type:jsonb" json:"header_map_json,omitempty"`

	ResponseFieldMapJSON string `gorm:"type:jsonb" json:"response_field_map_json,omitempty"`
	ResponseDefaultsJSON string `gorm:"type:jsonb" json:"response_defaults_json,omitempty"`
	ErrorFieldMapJSON    string `gorm:"type:jsonb" json:"error_field_map_json,omitempty"`
	ErrorDefaultsJSON    string `gorm:"type:jsonb" json:"error_defaults_json,omitempty"`

	NormalizeOpenAIUsage bool   `gorm:"not null;default:true" json:"normalize_openai_usage"`
	Enabled              bool   `gorm:"not null;default:true" json:"enabled"`
	ExtraJSON            string `gorm:"type:jsonb" json:"extra_json,omitempty"`
}

type RequestLog struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	RequestID        string    `gorm:"size:64;index" json:"request_id"`
	Method           string    `gorm:"size:16" json:"method"`
	Path             string    `gorm:"size:256" json:"path"`
	PublicModel      string    `gorm:"size:128;index" json:"public_model"`
	TargetProtocol   string    `gorm:"size:64;index" json:"target_protocol"`
	TargetEndpoint   string    `gorm:"size:96;index" json:"target_endpoint"`
	UpstreamModel    string    `gorm:"size:128" json:"upstream_model"`
	ProviderCode     string    `gorm:"size:64;index" json:"provider_code"`
	UpstreamURL      string    `gorm:"size:1024" json:"upstream_url"`
	StatusCode       int       `gorm:"index" json:"status_code"`
	LatencyMs        int64     `json:"latency_ms"`
	TraceID          string    `gorm:"size:128;index" json:"trace_id"`
	ErrorMessage     string    `gorm:"size:2048" json:"error_message"`
	RequestSnapshot  string    `gorm:"type:jsonb" json:"request_snapshot,omitempty"`
	OfficialRequest  string    `gorm:"type:jsonb" json:"official_request,omitempty"`
	UpstreamRequest  string    `gorm:"type:jsonb" json:"upstream_request,omitempty"`
	UpstreamResponse string    `gorm:"type:jsonb" json:"upstream_response,omitempty"`
	OfficialResponse string    `gorm:"type:jsonb" json:"official_response,omitempty"`
	ResponseUsage    string    `gorm:"type:jsonb" json:"response_usage,omitempty"`
}
