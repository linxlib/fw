package attribute

import "github.com/linxlib/astp"

var innerAttrNames = map[string]AttributeType{
	"GET":     TypeHttpMethod,
	"POST":    TypeHttpMethod,
	"PUT":     TypeHttpMethod,
	"DELETE":  TypeHttpMethod,
	"HEAD":    TypeHttpMethod,
	"OPTIONS": TypeHttpMethod,
	"TRACE":   TypeHttpMethod,
	"CONNECT": TypeHttpMethod,
	"ANY":     TypeHttpMethod,
	"WS":      TypeHttpMethod,
}

func AddMethodAttributeType(name string, typ AttributeType) {
	innerAttrNames[name] = typ
}

var attrMethodCaches = make(map[*astp.Element][]*Attribute)

func GetMethodAttributes(m *astp.Element) []*Attribute {
	if cmdCache, ok := attrMethodCaches[m]; ok {
		return cmdCache
	}
	cmdCache := ParseDoc(m.Docs, m.Name, innerAttrNames)
	attrMethodCaches[m] = cmdCache
	return cmdCache
}

func GetMethodAttributesAsMiddleware(m *astp.Element) []*Attribute {
	results := make([]*Attribute, 0)
	attrs := GetMethodAttributes(m)
	for _, attr := range attrs {
		if attr.Type == TypeMiddleware {
			results = append(results, attr)
		}
	}
	return results
}
