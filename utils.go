package fw

import (
	"encoding/xml"
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
