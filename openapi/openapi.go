package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type RouteInfo struct {
	Method               string
	Path                 string
	OperationID          string
	OperationDescription string
	TagName              string
	TagDescription       string
	Parameters           []Parameter
	RequestBody          *RequestBody
	ResponseSchema       *Schema
	// ResponseExample 是响应的完整示例值(整个 body). 留空时 openapi 层用
	// ResponseSchema 自带的 Example 兜底.
	ResponseExample any
	Security        []SecurityRequirement
}

// SecurityRequirement maps a security scheme name to its scopes (empty for most cases).
type SecurityRequirement map[string][]string

// SecurityScheme represents an OpenAPI security scheme definition.
type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	In           string `json:"in,omitempty"`
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
}

// Components holds reusable OpenAPI components.
type Components struct {
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

type Document struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Paths      map[string]PathItem `json:"paths"`
	Tags       []Tag               `json:"tags,omitempty"`
	Components *Components         `json:"components,omitempty"`
	Extra      map[string]any      `json:"-"`
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type Info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type PathItem map[string]Operation

type Operation struct {
	OperationID string                   `json:"operationId,omitempty"`
	Summary     string                   `json:"summary,omitempty"`
	Description string                   `json:"description,omitempty"`
	Tags        []string                 `json:"tags,omitempty"`
	Parameters  []Parameter              `json:"parameters,omitempty"`
	RequestBody *RequestBody             `json:"requestBody,omitempty"`
	Responses   map[string]ResponseEntry `json:"responses"`
	Security    []SecurityRequirement    `json:"security,omitempty"`
}

type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
	Schema      Schema `json:"schema"`
}

type RequestBody struct {
	Required bool                 `json:"required,omitempty"`
	Content  map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema Schema `json:"schema"`
	// Example 是该 media type 的完整示例值(整个 body), 优先级高于各字段自己的 example.
	Example any `json:"example,omitempty"`
}

type Schema struct {
	Type                 string            `json:"type,omitempty"`
	Format               string            `json:"format,omitempty"`
	Description          string            `json:"description,omitempty"`
	Required             []string          `json:"required,omitempty"`
	Enum                 []any             `json:"enum,omitempty"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	AdditionalProperties *Schema           `json:"additionalProperties,omitempty"`
	// Default 是该字段的默认值. 优先取 model 字段上 default tag 的显式标注,
	// 未标注时按类型给出零值(string -> "", 数值 -> 0, bool -> false).
	Default any `json:"default,omitempty"`
	// Example 是该字段的示例值. 优先取 model 字段上 example tag 的显式标注,
	// 未标注时按类型给出占位示例(string -> "string", 整数 -> 1, 浮点 -> 1.5, bool -> true).
	Example any `json:"example,omitempty"`
}

type ResponseEntry struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

func Generate(outputPath string, title string, version string, routes []RouteInfo, securitySchemes map[string]SecurityScheme) error {
	if outputPath == "" {
		outputPath = "openapi.json"
	}
	if title == "" {
		title = "FW API"
	}
	if version == "" {
		version = "1.0.0"
	}
	doc := Document{
		OpenAPI: "3.0.3",
		Info:    Info{Title: title, Version: version},
		Paths:   make(map[string]PathItem),
	}
	tagMap := make(map[string]Tag)
	for _, route := range routes {
		p := toOpenAPIPath(route.Path)
		method := strings.ToLower(strings.TrimSpace(route.Method))
		if method == "" || p == "" {
			continue
		}
		item := doc.Paths[p]
		if item == nil {
			item = make(PathItem)
		}
		opID := route.OperationID
		if opID == "" {
			opID = method + "_" + strings.ReplaceAll(strings.Trim(p, "/"), "/", "_")
		}
		if route.TagName != "" {
			t := tagMap[route.TagName]
			t.Name = route.TagName
			if t.Description == "" {
				t.Description = route.TagDescription
			}
			tagMap[route.TagName] = t
		}
		resp := ResponseEntry{Description: "OK"}
		if route.ResponseSchema != nil {
			resp.Content = map[string]MediaType{
				"application/json": {
					Schema:  *route.ResponseSchema,
					Example: responseExample(route),
				},
			}
		}
		summary := strings.TrimSpace(route.OperationDescription)
		if summary == "" {
			summary = strings.ToUpper(method) + " " + p
		}
		op := Operation{
			OperationID: opID,
			Summary:     summary,
			Tags:        nilIfEmpty(route.TagName),
			Parameters:  route.Parameters,
			RequestBody: route.RequestBody,
			Responses: map[string]ResponseEntry{
				"200": resp,
			},
		}
		if len(route.Security) > 0 {
			op.Security = route.Security
		}
		item[method] = op
		doc.Paths[p] = item
	}
	if len(securitySchemes) > 0 {
		doc.Components = &Components{SecuritySchemes: securitySchemes}
	}
	if len(tagMap) > 0 {
		names := make([]string, 0, len(tagMap))
		for name := range tagMap {
			names = append(names, name)
		}
		sort.Strings(names)
		doc.Tags = make([]Tag, 0, len(names))
		for _, name := range names {
			doc.Tags = append(doc.Tags, tagMap[name])
		}
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal openapi: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil && filepath.Dir(outputPath) != "." {
		return fmt.Errorf("create openapi output dir: %w", err)
	}
	if err := os.WriteFile(outputPath, b, 0644); err != nil {
		return fmt.Errorf("write openapi: %w", err)
	}
	return nil
}

func nilIfEmpty(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return []string{s}
}

func toOpenAPIPath(path string) string {
	if path == "" {
		return "/"
	}
	parts := strings.Split(path, "/")
	for i := range parts {
		if strings.HasPrefix(parts[i], ":") && len(parts[i]) > 1 {
			parts[i] = "{" + parts[i][1:] + "}"
		}
	}
	out := strings.Join(parts, "/")
	if !strings.HasPrefix(out, "/") {
		out = "/" + out
	}
	return out
}

// responseExample 取出响应的完整示例值: 优先 RouteInfo.ResponseExample,
// 否则退回 schema 自身的 Example(顶层 envelope 已带上).
func responseExample(route RouteInfo) any {
	if route.ResponseExample != nil {
		return route.ResponseExample
	}
	if route.ResponseSchema != nil {
		return route.ResponseSchema.Example
	}
	return nil
}
