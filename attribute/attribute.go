package attribute

import "strings"

type AttributeType int

const (
	TypeHttpMethod AttributeType = iota //http 请求方法
	TypeOther                           //其他
	TypeDoc                             //注释内容
	TypeMiddleware                      //中间件类
	TypeParam                           //方法的参数和返回值专用
	TypeTagger                          //这种类型仅用于标记一些元素
	TypeInner
)

// Attribute 注解命令
type Attribute struct {
	Name  string
	Value string
	Type  AttributeType
	Index int
}

var innerAttributeTypes = map[string]AttributeType{
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
	"IGNORE":  TypeOther,

	"ROUTE":      TypeMiddleware,
	"CONTROLLER": TypeTagger,
	"CTL":        TypeTagger,
	"BASE":       TypeTagger,

	"BODY":      TypeParam,
	"JSON":      TypeParam,
	"PATH":      TypeParam,
	"FORM":      TypeParam,
	"HEADER":    TypeParam,
	"QUERY":     TypeParam,
	"COOKIE":    TypeParam,
	"XML":       TypeParam,
	"MULTIPART": TypeParam,
	"SERVICE":   TypeParam,
	"PLAIN":     TypeParam,
}

func RegAttributeType(name string, value AttributeType) {
	name = strings.ToUpper(name)
	if _, ok := innerAttributeTypes[name]; !ok {
		innerAttributeTypes[name] = value
	}
}

// ParseDoc 解析注解
func ParseDoc(doc []string, name string) []*Attribute {
	if len(doc) == 0 {
		return []*Attribute{
			{Name: name, Value: name, Type: TypeDoc},
		}
	}
	docs := make([]*Attribute, len(doc))
	if doc == nil {
		return docs
	}
	for j, s := range doc {
		// 以@开头，为attr，在 innerAttributeTypes 查找
		if strings.HasPrefix(s, "@") {
			ps := strings.SplitN(s, " ", 2)
			value := ""
			if len(ps) == 2 {
				value = strings.TrimSpace(ps[1])
			}
			docName := strings.TrimLeft(ps[0], "@")
			docName = strings.ToUpper(docName)
			docs[j] = &Attribute{
				Name:  docName,
				Value: value,
				Type:  innerAttributeTypes[docName],
			}
		} else if strings.HasPrefix(s, name) {
			// 如果是 当前结构、方法的名称开头
			// 视为文档注释类
			ps := strings.SplitN(s, " ", 2)
			value := ""
			if len(ps) == 2 {
				value = strings.TrimSpace(ps[1])
			}
			docs[j] = &Attribute{
				Name:  name,
				Value: value,
				Type:  TypeDoc,
			}
		} else {
			// 其他情况暂时以文档看待
			docs[j] = &Attribute{
				Name:  name,
				Value: s,
				Type:  TypeDoc,
			}
		}

	}
	return docs
}
