package attribute

import "github.com/linxlib/astp"

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

func GetLastAttr(f *astp.Element) *Attribute {
	as := GetFieldAttributeAsParamType(f)
	return as[len(as)-1]
}
