package official

type ParamSpec struct {
	Name        string   `json:"name"`
	Location    string   `json:"location"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description"`
}

type EndpointSpec struct {
	Key              string      `json:"key"`
	Protocol         string      `json:"protocol"`
	Name             string      `json:"name"`
	Method           string      `json:"method"`
	Path             string      `json:"path"`
	BodyMode         string      `json:"body_mode"`
	UpstreamBodyMode string      `json:"upstream_body_mode"`
	Description      string      `json:"description"`
	Params           []ParamSpec `json:"params"`
	ResponseFields   []ParamSpec `json:"response_fields"`
}

type Catalog struct {
	Endpoints []EndpointSpec `json:"endpoints"`
}

func AllEndpoints() []EndpointSpec {
	endpoints := make([]EndpointSpec, 0)
	// 官方协议的接口形态在代码中内置，三方渠道只需要在数据库中配置字段映射。
	endpoints = append(endpoints, OpenAIEndpoints()...)
	return endpoints
}

func FindEndpoint(key string) (EndpointSpec, bool) {
	for _, endpoint := range AllEndpoints() {
		if endpoint.Key == key {
			return endpoint, true
		}
	}
	return EndpointSpec{}, false
}
