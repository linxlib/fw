package attribute

//var attrMethodCaches = make(map[*astp.Element][]*Attribute)
//
//// GetMethodAttributes 解析方法上的注释
//func GetMethodAttributes(m *astp.Element) []*Attribute {
//	if cmdCache, ok := attrMethodCaches[m]; ok {
//		return cmdCache
//	}
//	cmdCache := ParseDoc(m.Docs, m.Name)
//	attrMethodCaches[m] = cmdCache
//	return cmdCache
//}
//
//// GetMethodAttributesAsMiddleware 提取方法上的中间件标记
//func GetMethodAttributesAsMiddleware(m *astp.Element) []*Attribute {
//	results := make([]*Attribute, 0)
//	attrs := GetMethodAttributes(m)
//	for _, attr := range attrs {
//		if attr.Type == TypeMiddleware {
//			results = append(results, attr)
//		}
//	}
//	return results
//}
