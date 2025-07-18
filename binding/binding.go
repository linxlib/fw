package binding

import (
	valid "github.com/gookit/validate"
	"github.com/gookit/validate/locales/zhcn"
	"github.com/linxlib/astp/constants"
	"github.com/valyala/fasthttp"
	"strings"
)

// Content-Type MIME of the most common data formats.
const (
	MIMEJSON              = "application/json"
	MIMEHTML              = "text/html"
	MIMEXML               = "application/xml"
	MIMEXML2              = "text/xml"
	MIMEPlain             = "text/plain"
	MIMEPOSTForm          = "application/x-www-form-urlencoded"
	MIMEMultipartPOSTForm = "multipart/form-data"
	//MIMEPROTOBUF          = "application/x-protobuf"
	//MIMEMSGPACK           = "application/x-msgpack"
	//MIMEMSGPACK2          = "application/msgpack"
	//MIMEYAML              = "application/x-yaml"
	//MIMEYAML2             = "application/yaml"
	//MIMETOML              = "application/toml"
)

// Binding describes the interface which needs to be implemented for binding the
// data present in the request such as JSON request body, query parameters or
// the form POST.
type Binding interface {
	Name() string
	Bind(*fasthttp.RequestCtx, any) error
}

// BindingBody adds BindBody method to Binding. BindBody is similar with Bind,
// but it reads the body from supplied bytes instead of req.Body.
type BindingBody interface {
	Binding
	BindBody([]byte, any) error
}

// BindingUri adds BindUri method to Binding. BindUri is similar with Bind,
// but it reads the Params.
type BindingUri interface {
	Name() string
	BindUri(map[string][]string, any) error
}

// StructValidator is the minimal interface which needs to be implemented in
// order for it to be used as the validator engine for ensuring the correctness
// of the request. Gin provides a default implementation for this using
// https://github.com/go-playground/validator/tree/v10.6.1.
type StructValidator interface {
	// ValidateStruct can receive any kind of type and it should never panic, even if the configuration is not right.
	// If the received type is a slice|array, the validation should be performed travel on every element.
	// If the received type is not a struct or slice|array, any validation should be skipped and nil must be returned.
	// If the received type is a struct or pointer to a struct, the validation should be performed.
	// If the struct is not valid or the validation itself fails, a descriptive error should be returned.
	// Otherwise nil must be returned.
	ValidateStruct(any) error

	// Engine returns the underlying validator engine which powers the
	// StructValidator implementation.
	Engine() any
}

// Validator is the default validator which implements the StructValidator
// interface. It uses https://github.com/go-playground/validator/tree/v10.6.1
// under the hood.
//var Validator StructValidator = &defaultValidator{}

// These implement the Binding interface and can be used to bind the data
// present in the request to struct instances.
var (
	JSON          BindingBody = jsonBinding{}
	XML           BindingBody = xmlBinding{}
	Form          Binding     = formBinding{}
	Query         Binding     = queryBinding{}
	FormPost      Binding     = formPostBinding{}
	FormMultipart Binding     = formMultipartBinding{}
	Path          Binding     = pathBinding{}
	Cookie        Binding     = cookieBinding{}
	//ProtoBuf      BindingBody = protobufBinding{}
	//MsgPack       BindingBody = msgpackBinding{}
	//YAML          BindingBody = yamlBinding{}
	Uri    BindingUri  = uriBinding{}
	Header Binding     = headerBinding{}
	Plain  BindingBody = plainBinding{}
	//TOML          BindingBody = tomlBinding{}
)

// Default returns the appropriate Binding instance based on the HTTP method
// and the content type.
func Default(method, contentType string) Binding {
	if method == fasthttp.MethodGet {
		return Form
	}

	switch contentType {
	case MIMEJSON:
		return JSON
	case MIMEXML, MIMEXML2:
		return XML
	//case MIMEPROTOBUF:
	//	return ProtoBuf
	//case MIMEMSGPACK, MIMEMSGPACK2:
	//	return MsgPack
	//case MIMEYAML, MIMEYAML2:
	//	return YAML
	//case MIMETOML:
	//	return TOML
	case MIMEMultipartPOSTForm:
		return FormMultipart
	default: // case MIMEPOSTForm:
		return Form
	}
}
func Get(cmd string) Binding {
	switch cmd {
	case "cookie":
		return Cookie
	case "path":
		return Path
	case "header":
		return Header
	case "plain":
		return Plain
	case "query":
		return Query
	case "form":
		return Form
	case "multipart":
		return FormMultipart
	case "xml":
		return XML
	case "json", "body":
		return JSON
	default:
		return JSON
	}
}
func GetByAttr(attr constants.AttrType) Binding {
	return Get(strings.ToLower(constants.AttrNames[attr]))
}
func IsBodyBinder(bind Binding) bool {
	return bind == JSON || bind == XML || bind == Form || bind == FormMultipart || bind == Plain
}

func validate(obj any) error {
	v := valid.New(obj)
	v.Validate()

	return v.ValidateErr()
}

func init() {
	zhcn.RegisterGlobal()
}
