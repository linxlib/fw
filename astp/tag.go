package astp

import (
	"reflect"
	"strings"
)

func parseTag(tagValue string) map[string]string {
	tagValue = strings.Trim(tagValue, "`")
	result := make(map[string]string)

	st := reflect.StructTag(tagValue)
	for _, key := range extractTagKeys(tagValue) {
		if val, ok := st.Lookup(key); ok {
			result[key] = val
		}
	}

	return result
}

func extractTagKeys(tagValue string) []string {
	var keys []string
	tagValue = strings.Trim(tagValue, "`")

	for tagValue != "" {
		i := strings.Index(tagValue, ":")
		if i < 0 {
			break
		}
		key := strings.TrimSpace(tagValue[:i])
		if key != "" {
			keys = append(keys, key)
		}
		tagValue = tagValue[i+1:]

		if tagValue == "" {
			break
		}

		if tagValue[0] != '"' {
			continue
		}

		end := strings.Index(tagValue[1:], "\"")
		if end < 0 {
			break
		}
		tagValue = tagValue[end+2:]
		tagValue = strings.TrimSpace(tagValue)
	}

	return keys
}

func GetTag(tags map[string]string, key string) string {
	if tags == nil {
		return ""
	}
	return tags[key]
}

func HasTag(tags map[string]string, key string) bool {
	if tags == nil {
		return false
	}
	_, ok := tags[key]
	return ok
}
