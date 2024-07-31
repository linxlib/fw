package fw

import (
	"encoding/xml"
	"fmt"
	"github.com/gookit/color"
	"github.com/linxlib/conv"
	"github.com/linxlib/fw/internal/json"
	"github.com/sirupsen/logrus"
	"reflect"
	"strings"
	"unicode"
)

type H map[string]any

func (h H) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{
		Space: "",
		Local: "map",
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for key, value := range h {
		elem := xml.StartElement{
			Name: xml.Name{Space: "", Local: key},
			Attr: []xml.Attr{},
		}
		if err := e.EncodeElement(value, elem); err != nil {
			return err
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func joinRoute(base string, r string) string {
	var result = base
	if r == "/" || r == "" {

		if strings.HasSuffix(result, "/") {
			result = strings.TrimSuffix(result, "/")
			r = ""
		} else {
			r = strings.TrimSuffix(r, "/")
			result += r
		}
	} else {
		if strings.HasSuffix(result, "/") {
			r = strings.TrimPrefix(r, "/")
			result += r
		} else {
			r = strings.TrimPrefix(r, "/")
			result += "/" + r
		}
	}
	return result
}
func Stringify(v interface{}) string {
	var ret string
	if IsObject(v) || isMap(v) {
		ret = marshal(v)
	} else {
		ret = fmt.Sprint(v)
	}
	return ret
}
func IsObject(v interface{}) bool {
	return reflect.ValueOf(v).Kind() == reflect.Struct
}
func marshal(o interface{}) string {
	str, ok := o.(string)
	if ok {
		return str
	}
	data, err := json.Marshal(o)
	if err != nil {
		return fmt.Sprint(o)
	}
	return string(data)
}
func unmarshal(data string, o interface{}) error {
	return json.Unmarshal(conv.Bytes(data), o)
}
func marshalIndent(o interface{}) string {
	str, ok := o.(string)
	if ok {
		return str
	}
	m, err := json.MarshalIndent(o, "", " ")
	if err != nil {
		return fmt.Sprint(o)
	}
	return string(m)
}
func isMap(v interface{}) bool {
	_, isMap := v.(map[string]interface{})
	_, isLogFields := v.(logrus.Fields)

	return isMap || isLogFields
}

var (
	white        = color.HiWhite.Render
	red          = color.HiRed.Render
	green        = color.HiGreen.Render
	blue         = color.HiBlue.Render
	darkBlue     = color.Blue.Render
	cyan         = color.HiCyan.Render
	gray         = color.Gray.Render
	yellow       = color.HiYellow.Render
	magenta      = color.HiMagenta.Render
	lightmagenta = color.LightMagenta.Render
	info         = color.Info.Render
	note         = color.Note.Render
	err          = color.Error.Render
	danger       = color.Danger.Render
	success      = color.Success.Render
	warning      = color.Warn.Render
	question     = color.Question.Render
	primary      = color.Primary.Render
	secondary    = color.Secondary.Render
)
