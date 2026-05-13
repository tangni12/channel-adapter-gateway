package transform

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type ResponseFieldSpec struct {
	Name     string
	Type     string
	Required bool
}

func HasResponseMapping(fieldMapJSON, defaultsJSON string) bool {
	return strings.TrimSpace(fieldMapJSON) != "" && strings.TrimSpace(fieldMapJSON) != "{}" ||
		strings.TrimSpace(defaultsJSON) != "" && strings.TrimSpace(defaultsJSON) != "{}"
}

func MapResponse(body []byte, fieldMapJSON, defaultsJSON string, specs []ResponseFieldSpec) ([]byte, []byte, error) {
	if !HasResponseMapping(fieldMapJSON, defaultsJSON) {
		return body, nil, nil
	}

	var source any
	if err := json.Unmarshal(body, &source); err != nil {
		return nil, nil, fmt.Errorf("response mapping requires json body: %w", err)
	}

	fieldMap := parseStringMap(fieldMapJSON)
	defaults := parseAnyMap(defaultsJSON)
	specMap := responseSpecMap(specs)
	target := make(map[string]any)

	for officialPath, upstreamPath := range fieldMap {
		officialPath = strings.TrimSpace(officialPath)
		upstreamPath = strings.TrimSpace(upstreamPath)
		if officialPath == "" || upstreamPath == "" || upstreamPath == "-" {
			continue
		}
		spec := specMap[officialPath]
		value, ok := getPathValue(source, upstreamPath)
		if !ok && spec.Required {
			return nil, nil, fmt.Errorf("response field mapping failed: upstream field %q for official field %q was not found", upstreamPath, officialPath)
		}
		if !ok {
			continue
		}
		converted, err := convertPathValue(value, spec.Type, officialPath, upstreamPath)
		if err != nil {
			return nil, nil, err
		}
		if err := setPathValue(target, officialPath, converted); err != nil {
			return nil, nil, fmt.Errorf("response field mapping failed: set official field %q: %w", officialPath, err)
		}
	}

	for officialPath, value := range defaults {
		officialPath = strings.TrimSpace(officialPath)
		if officialPath == "" {
			continue
		}
		converted, err := convertPathValue(value, specMap[officialPath].Type, officialPath, "<default>")
		if err != nil {
			return nil, nil, err
		}
		if err := setPathValue(target, officialPath, converted); err != nil {
			return nil, nil, fmt.Errorf("response default mapping failed: set official field %q: %w", officialPath, err)
		}
	}

	mapped, err := json.Marshal(target)
	if err != nil {
		return nil, nil, err
	}
	usage, _ := json.Marshal(target["usage"])
	return mapped, usage, nil
}

type pathToken struct {
	name  string
	array bool
}

func parsePath(pathText string) []pathToken {
	raw := strings.Split(pathText, ".")
	tokens := make([]pathToken, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		token := pathToken{name: part}
		if strings.HasSuffix(part, "[]") {
			token.name = strings.TrimSuffix(part, "[]")
			token.array = true
		}
		tokens = append(tokens, token)
	}
	return tokens
}

func getPathValue(source any, pathText string) (any, bool) {
	return getByTokens(source, parsePath(pathText))
}

func getByTokens(current any, tokens []pathToken) (any, bool) {
	if len(tokens) == 0 {
		return current, true
	}
	token := tokens[0]
	switch value := current.(type) {
	case map[string]any:
		next, ok := value[token.name]
		if !ok {
			return nil, false
		}
		if token.array {
			items, ok := toAnySlice(next)
			if !ok {
				return nil, false
			}
			result := make([]any, 0, len(items))
			for _, item := range items {
				itemValue, ok := getByTokens(item, tokens[1:])
				if ok {
					result = append(result, itemValue)
				}
			}
			return result, len(result) > 0
		}
		return getByTokens(next, tokens[1:])
	case []any:
		result := make([]any, 0, len(value))
		for _, item := range value {
			itemValue, ok := getByTokens(item, tokens)
			if ok {
				result = append(result, itemValue)
			}
		}
		return result, len(result) > 0
	default:
		return nil, false
	}
}

func setPathValue(target map[string]any, pathText string, value any) error {
	tokens := parsePath(pathText)
	if len(tokens) == 0 {
		return fmt.Errorf("empty path")
	}
	return setByTokens(target, tokens, value)
}

func setByTokens(current map[string]any, tokens []pathToken, value any) error {
	token := tokens[0]
	if len(tokens) == 1 {
		if token.array {
			items, ok := toAnySlice(value)
			if !ok {
				items = []any{value}
			}
			current[token.name] = items
			return nil
		}
		current[token.name] = value
		return nil
	}
	if token.array {
		values, ok := toAnySlice(value)
		if !ok {
			values = []any{value}
		}
		items, _ := current[token.name].([]any)
		for len(items) < len(values) {
			items = append(items, map[string]any{})
		}
		for i, itemValue := range values {
			child, ok := items[i].(map[string]any)
			if !ok {
				child = make(map[string]any)
				items[i] = child
			}
			if err := setByTokens(child, tokens[1:], itemValue); err != nil {
				return err
			}
		}
		current[token.name] = items
		return nil
	}
	child, ok := current[token.name].(map[string]any)
	if !ok {
		child = make(map[string]any)
		current[token.name] = child
	}
	return setByTokens(child, tokens[1:], value)
}

func convertPathValue(value any, targetType, officialPath, upstreamPath string) (any, error) {
	if targetType == "" {
		return value, nil
	}
	if items, ok := value.([]any); ok {
		converted := make([]any, 0, len(items))
		for index, item := range items {
			next, err := convertSingleValue(item, targetType)
			if err != nil {
				return nil, fmt.Errorf("response field mapping failed: official field %q expects %s, upstream field %q item %d cannot be converted: %w", officialPath, targetType, upstreamPath, index, err)
			}
			converted = append(converted, next)
		}
		return converted, nil
	}
	converted, err := convertSingleValue(value, targetType)
	if err != nil {
		return nil, fmt.Errorf("response field mapping failed: official field %q expects %s, upstream field %q cannot be converted: %w", officialPath, targetType, upstreamPath, err)
	}
	return converted, nil
}

func convertSingleValue(value any, targetType string) (any, error) {
	switch strings.ToLower(targetType) {
	case "", "any":
		return value, nil
	case "string":
		return toStringValue(value)
	case "integer", "int":
		return toIntValue(value)
	case "number", "float":
		return toNumberValue(value)
	case "boolean", "bool":
		return toBoolValue(value)
	case "object":
		if _, ok := value.(map[string]any); ok {
			return value, nil
		}
		return nil, fmt.Errorf("value is %T, not object", value)
	case "array":
		if _, ok := toAnySlice(value); ok {
			return value, nil
		}
		return nil, fmt.Errorf("value is %T, not array", value)
	default:
		return value, nil
	}
}

func toStringValue(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case nil:
		return "", nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
}

func toIntValue(value any) (int64, error) {
	switch v := value.(type) {
	case float64:
		return int64(v), nil
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case json.Number:
		return v.Int64()
	case string:
		return strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	default:
		return 0, fmt.Errorf("value is %T", value)
	}
}

func toNumberValue(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	case string:
		return strconv.ParseFloat(strings.TrimSpace(v), 64)
	default:
		return 0, fmt.Errorf("value is %T", value)
	}
}

func toBoolValue(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(strings.TrimSpace(v))
	default:
		return false, fmt.Errorf("value is %T", value)
	}
}

func toAnySlice(value any) ([]any, bool) {
	items, ok := value.([]any)
	return items, ok
}

func parseAnyMap(raw string) map[string]any {
	result := make(map[string]any)
	if strings.TrimSpace(raw) == "" {
		return result
	}
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func responseSpecMap(specs []ResponseFieldSpec) map[string]ResponseFieldSpec {
	result := make(map[string]ResponseFieldSpec, len(specs))
	for _, spec := range specs {
		result[spec.Name] = spec
	}
	return result
}
