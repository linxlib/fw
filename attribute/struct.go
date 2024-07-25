package attribute

import "github.com/linxlib/astp"

var innerStructAttrNames = map[string]AttributeType{
	"OpenApiDoc": TypeDoc,
	"Inject":     TypeOther,
	"Route":      TypeMiddleware,
	"Controller": TypeTagger,
	"Ctl":        TypeTagger,
	"Base":       TypeTagger,

	"Tag":        TypeDoc,
	"Deprecated": TypeDoc,
	"Ignore":     TypeDoc,

	"Body":      TypeParam,
	"Json":      TypeParam,
	"Path":      TypeParam,
	"Form":      TypeParam,
	"Header":    TypeParam,
	"Query":     TypeParam,
	"Cookie":    TypeParam,
	"XML":       TypeParam,
	"Multipart": TypeParam,
	"Service":   TypeParam,
	"Plain":     TypeParam,
}
var cmdStructCaches = make(map[*astp.Struct][]*Attribute)

func AddStructAttributeType(name string, t AttributeType) {
	innerStructAttrNames[name] = t
}

func GetStructAttrs(s *astp.Struct) []*Attribute {
	if cmdCache, ok := cmdStructCaches[s]; ok {
		return cmdCache
	}
	cmdCache := ParseDoc(s.Docs, s.Name, innerStructAttrNames)
	cmdStructCaches[s] = cmdCache
	return cmdCache
}

func HasAttribute(s *astp.Struct, name string) bool {
	if cmdCache, ok := cmdStructCaches[s]; ok {
		for _, cmd := range cmdCache {
			if cmd.Name == name {
				return true
			}
		}
	} else {
		cmdCache = GetStructAttrs(s)
		if cmdCache == nil || len(cmdCache) <= 0 {
			return false
		}
		return GetStructAttrByName(s, name) != nil
	}
	return false
}
func GetStructAttrByName(s *astp.Struct, name string) *Attribute {
	if cmdCache, ok := cmdStructCaches[s]; ok {
		for _, cmd := range cmdCache {
			if cmd.Name == name {
				return cmd
			}
		}
	} else {
		cmdCache = GetStructAttrs(s)
		if cmdCache == nil || len(cmdCache) <= 0 {
			return nil
		}
		return GetStructAttrByName(s, name)
	}
	return nil
}

func GetStructAttrAsMiddleware(s *astp.Struct) []*Attribute {
	results := make([]*Attribute, 0)
	attrs := GetStructAttrs(s)
	for _, attr := range attrs {
		if attr.Type == TypeMiddleware {
			results = append(results, attr)
		}
	}
	return results
}
