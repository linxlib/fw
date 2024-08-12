package attribute

import (
	"github.com/linxlib/astp"
	"strings"
)

var cmdStructCaches = make(map[*astp.Element][]*Attribute)

// GetStructAttrs 获取结构体的注释
func GetStructAttrs(s *astp.Element) []*Attribute {
	if cmdCache, ok := cmdStructCaches[s]; ok {
		return cmdCache
	}

	cmdCache := ParseDoc(s.Docs, s.Name)
	if cmdCache != nil {
		cmdStructCaches[s] = cmdCache
	}
	return cmdCache
}

// HasAttribute 是否有特定注释
func HasAttribute(s *astp.Element, attrName string) bool {
	if s == nil {
		return false
	}
	attrName = strings.ToUpper(attrName)
	if cmdCache, ok := cmdStructCaches[s]; ok {
		for _, cmd := range cmdCache {
			if cmd.Name == attrName {
				return true
			}
		}
	} else {
		cmdCache = GetStructAttrs(s)
		if cmdCache == nil || len(cmdCache) <= 0 {
			return false
		}
		return GetStructAttrByName(s, attrName) != nil
	}
	return false
}

// GetStructAttrByName 按attr名称返回，不存在则返回nil
func GetStructAttrByName(s *astp.Element, attrName string) *Attribute {
	attrName = strings.ToUpper(attrName)
	if cmdCache, ok := cmdStructCaches[s]; ok {
		for _, cmd := range cmdCache {
			if cmd.Name == attrName {
				return cmd
			}
		}
	} else {
		cmdCache = GetStructAttrs(s)
		if cmdCache == nil || len(cmdCache) <= 0 {
			return nil
		}
		return GetStructAttrByName(s, attrName)
	}
	return nil
}

// GetStructAttrAsMiddleware 返回结构体上的中间件类attr
func GetStructAttrAsMiddleware(s *astp.Element) []*Attribute {
	results := make([]*Attribute, 0)
	attrs := GetStructAttrs(s)
	for _, attr := range attrs {
		if attr.Type == TypeMiddleware {
			results = append(results, attr)
		}
	}
	return results
}
