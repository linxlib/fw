package attribute

import "github.com/linxlib/astp"

func GetFieldAttributeAsParamType(f *astp.ParamField) []*Attribute {
	results := make([]*Attribute, 0)
	if f.Type != nil {
		attrs := GetStructAttrs(f.Type)
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

func GetLastAttr(f *astp.ParamField) *Attribute {
	as := GetFieldAttributeAsParamType(f)
	return as[len(as)-1]
}
