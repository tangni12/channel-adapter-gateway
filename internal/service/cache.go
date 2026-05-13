package service

import (
	"fmt"
	"sync"

	"channel-adapter-gateway/internal/model"

	"gorm.io/gorm"
)

type MappingCache struct {
	db        *gorm.DB
	mu        sync.RWMutex
	providers map[string]model.Provider
	rules     map[string]model.MappingRule
}

func NewMappingCache(db *gorm.DB) *MappingCache {
	return &MappingCache{
		db:        db,
		providers: make(map[string]model.Provider),
		rules:     make(map[string]model.MappingRule),
	}
}

func (c *MappingCache) Refresh() error {
	var providers []model.Provider
	if err := c.db.Where("enabled = ?", true).Find(&providers).Error; err != nil {
		return err
	}
	var rules []model.MappingRule
	if err := c.db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return err
	}

	nextProviders := make(map[string]model.Provider, len(providers))
	for _, provider := range providers {
		nextProviders[provider.Code] = provider
	}
	nextRules := make(map[string]model.MappingRule, len(rules))
	for _, rule := range rules {
		nextRules[cacheKey(rule.TargetProtocol, rule.TargetEndpoint, rule.PublicModel)] = rule
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers = nextProviders
	c.rules = nextRules
	return nil
}

func (c *MappingCache) Find(targetProtocol, targetEndpoint, publicModel string) (model.MappingRule, model.Provider, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rule, ok := c.rules[cacheKey(targetProtocol, targetEndpoint, publicModel)]
	if !ok {
		return rule, model.Provider{}, fmt.Errorf("mapping not found: protocol=%s endpoint=%s model=%s", targetProtocol, targetEndpoint, publicModel)
	}
	provider, ok := c.providers[rule.ProviderCode]
	if !ok {
		return rule, provider, fmt.Errorf("provider not found or disabled: %s", rule.ProviderCode)
	}
	return rule, provider, nil
}

func (c *MappingCache) Models() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	seen := make(map[string]bool)
	models := make([]string, 0)
	for _, rule := range c.rules {
		if seen[rule.PublicModel] {
			continue
		}
		seen[rule.PublicModel] = true
		models = append(models, rule.PublicModel)
	}
	return models
}

func cacheKey(protocol, endpoint, modelName string) string {
	return protocol + "|" + endpoint + "|" + modelName
}
