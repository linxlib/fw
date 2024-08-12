package attribute

import "github.com/linxlib/astp"

// GetFieldAttributeAsParamType 返回参数对应类型上的注解
func GetFieldAttributeAsParamType(f *astp.Element) []*Attribute {
	results := make([]*Attribute, 0)
	if f.Item != nil {
		attrs := GetStructAttrs(f.Item)
		for _, attr := range attrs {
			if attr.Type == TypeParam {
				results = append(results, attr)
			}
		}
	} else {
		attr := new(Attribute)
		attr.Type = TypeInner
		attr.Name = f.Name
		attr.Value = ""
		attr.Index = 0
		results = append(results, attr)
	}
	return results
}

// GetLastAttr 返回参数类型上注解的最后一个
func GetLastAttr(f *astp.Element) *Attribute {
	as := GetFieldAttributeAsParamType(f)
	return as[len(as)-1]
}
