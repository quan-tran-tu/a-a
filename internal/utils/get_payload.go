package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func GetStringPayload(payload map[string]any, key string) (string, error) {
	value, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("payload is missing required key: '%s'", key)
	}
	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("payload key '%s' has an invalid type (expected string)", key)
	}
	return strValue, nil
}

func GetIntPayload(payload map[string]any, key string) (int, error) {
	v, ok := payload[key]
	if !ok {
		return 0, fmt.Errorf("payload is missing required key: '%s'", key)
	}
	switch t := v.(type) {
	case float64:
		return int(t), nil
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0, fmt.Errorf("payload key '%s' invalid int: %v", key, err)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("payload key '%s' has unsupported type %T", key, v)
	}
}
