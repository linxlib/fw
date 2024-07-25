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

// ParseDoc 解析注解
func ParseDoc(doc []string, name string, i map[string]AttributeType) []*Attribute {
	docs := make([]*Attribute, len(doc))
	if doc == nil {
		return docs
	}
	for j, s := range doc {
		if strings.HasPrefix(s, "@") {
			ps := strings.SplitN(s, " ", 2)
			value := ""
			if len(ps) == 2 {
				value = strings.TrimSpace(ps[1])
			}
			docName := strings.TrimLeft(ps[0], "@")
			docs[j] = &Attribute{
				Name:  docName,
				Value: value,
				Type:  i[docName],
			}
		} else if strings.HasPrefix(s, name) {
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
			docs[j] = &Attribute{
				Name:  name,
				Value: s,
				Type:  TypeDoc,
			}
		}

	}
	return docs
}
