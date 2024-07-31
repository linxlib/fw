package main

import (
	"github.com/linxlib/fw"
)

//var genjson []byte

func main() {
	s := fw.New()
	s.RegisterRoute(new(Hello))
	s.Run()
}

//func init() {
//	parser := astp.NewParser()
//	f := &astp.File{
//		Name:        "main.go",
//		PackageName: "main",
//		Imports: []*astp.Import{
//			&astp.Import{
//				Name:       "fw",
//				Alias:      "_",
//				ImportPath: "github.com/linxlib/fw",
//				IsIgnore:   false,
//			},
//		},
//		PackagePath: "github.com/linxlib/fw/example",
//		FilePath:    "E:\\repos\\fw\\example\\main.go",
//		Structs: []*astp.Struct{
//			&astp.Struct{
//				Name:        "Hello",
//				PackagePath: "github.com/linxlib/fw/example",
//				KeyHash:     "",
//				Fields:      nil,
//				Methods: []*astp.Method{
//					&astp.Method{
//						Receiver: &astp.Receiver{
//							Name:       "h",
//							Pointer:    true,
//							TypeString: "Hello",
//							Type:       nil,
//						},
//						Index:       0,
//						PackagePath: "github.com/linxlib/fw/example",
//						Name:        "Hello",
//						Private:     false,
//						Signature:   "",
//						Docs: []string{
//							"Hello",
//							"@GET /hello",
//						},
//						Comments: "",
//						Params: []*astp.ParamField{
//							&astp.ParamField{
//								Index:       0,
//								Name:        "ctx",
//								PackagePath: "github.com/linxlib/fw",
//								Type:        nil,
//								HasTag:      false,
//								Tag:         "",
//								TypeString:  "*fw.Context",
//								InnerType:   true,
//								Private:     true,
//								Pointer:     true,
//								Slice:       false,
//								IsStruct:    true,
//								Docs:        nil,
//								Comment:     "",
//								IsGeneric:   false,
//							},
//						},
//						Results:    nil,
//						IsGeneric:  false,
//						TypeParams: nil,
//					},
//				},
//				HasParent:   false,
//				IsInterface: false,
//				Inter:       nil,
//				Docs: []string{
//					"Hello",
//					"@Controller",
//				},
//				Comment:    "",
//				IsGeneric:  false,
//				TypeParams: nil,
//			},
//		},
//		Docs:     nil,
//		Comments: nil,
//		Methods:  nil,
//		Funcs:    nil,
//		Consts:   nil,
//		Vars:     nil,
//	}
//	parser.Files["github.com/linxlib/fw/example.main.go"] = f
//	parser.WriteOut("./gen.json")
//}

// Hello
// @Controller
type Hello struct {
}

// Hello
// @GET /hello
func (h *Hello) Hello(c *fw.Context) {
	c.String(200, "Hello")
}
