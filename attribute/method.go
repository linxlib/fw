package attribute

import "github.com/linxlib/astp"

var innerAttrNames = map[string]AttributeType{
	"OpenApiDoc": TypeOther,
	"GET":        TypeHttpMethod,
	"POST":       TypeHttpMethod,
	"PUT":        TypeHttpMethod,
	"DELETE":     TypeHttpMethod,
	"HEAD":       TypeHttpMethod,
	"OPTIONS":    TypeHttpMethod,
	"TRACE":      TypeHttpMethod,
	"CONNECT":    TypeHttpMethod,
	"ANY":        TypeHttpMethod,
	"WS":         TypeHttpMethod,
	"Deprecated": TypeOther,
	"Ignore":     TypeOther,
}

func AddMethodAttributeType(name string, typ AttributeType) {
	innerAttrNames[name] = typ
}

var attrMethodCaches = make(map[*astp.Method][]*Attribute)

func GetMethodAttributes(m *astp.Method) []*Attribute {
	if cmdCache, ok := attrMethodCaches[m]; ok {
		return cmdCache
	}
	cmdCache := ParseDoc(m.Docs, m.Name, innerAttrNames)
	attrMethodCaches[m] = cmdCache
	return cmdCache
}

func GetMethodAttributesAsMiddleware(m *astp.Method) []*Attribute {
	results := make([]*Attribute, 0)
	attrs := GetMethodAttributes(m)
	for _, attr := range attrs {
		if attr.Type == TypeMiddleware {
			results = append(results, attr)
		}
	}
	return results
}
