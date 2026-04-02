package astp

import (
	"strings"
)

type Query struct {
	project *Project
}

func NewQuery(project *Project) *Query {
	return &Query{project: project}
}

func (q *Query) FindType(name string) *Type {
	if q.project == nil {
		return nil
	}

	for _, pkg := range q.project.Packages {
		if t, ok := pkg.Types[name]; ok {
			return t
		}
	}
	return nil
}

func (q *Query) FindTypeByPath(pkgPath, name string) *Type {
	if q.project == nil {
		return nil
	}

	pkg, ok := q.project.Packages[pkgPath]
	if !ok {
		return nil
	}
	return pkg.Types[name]
}

func (q *Query) FindFunc(name string) *Func {
	if q.project == nil {
		return nil
	}

	for _, pkg := range q.project.Packages {
		if f, ok := pkg.Functions[name]; ok {
			return f
		}
	}
	return nil
}

func (q *Query) FindImplementations(interfaceName string) []*Type {
	if q.project == nil {
		return nil
	}

	var results []*Type
	for _, pkg := range q.project.Packages {
		for _, t := range pkg.Types {
			if t.Kind == KindStruct {
				for _, impl := range t.Implements {
					if impl == interfaceName {
						results = append(results, t)
						break
					}
				}
			}
		}
	}
	return results
}

func (q *Query) FindByAnnotation(annotation string) []interface{} {
	if q.project == nil {
		return nil
	}

	var results []interface{}

	for _, pkg := range q.project.Packages {
		for _, t := range pkg.Types {
			if HasAnnotation(t.Doc, annotation) {
				results = append(results, t)
			}
		}

		for _, f := range pkg.Functions {
			if HasAnnotation(f.Doc, annotation) {
				results = append(results, f)
			}
		}
	}

	return results
}

func (q *Query) FindTypesByAnnotation(annotation string) []*Type {
	if q.project == nil {
		return nil
	}

	var results []*Type
	for _, pkg := range q.project.Packages {
		for _, t := range pkg.Types {
			if HasAnnotation(t.Doc, annotation) {
				results = append(results, t)
			}
		}
	}
	return results
}

func (q *Query) FindFuncsByAnnotation(annotation string) []*Func {
	if q.project == nil {
		return nil
	}

	var results []*Func
	for _, pkg := range q.project.Packages {
		for _, f := range pkg.Functions {
			if HasAnnotation(f.Doc, annotation) {
				results = append(results, f)
			}
		}
		for _, t := range pkg.Types {
			for _, m := range t.Methods {
				if HasAnnotation(m.Doc, annotation) {
					results = append(results, m)
				}
			}
		}
	}
	return results
}

func (q *Query) FindByTag(tagKey string) []*Field {
	if q.project == nil {
		return nil
	}

	var results []*Field
	for _, pkg := range q.project.Packages {
		for _, t := range pkg.Types {
			if t.Kind == KindStruct {
				for _, f := range t.Fields {
					if HasTag(f.Tag, tagKey) {
						results = append(results, f)
					}
				}
			}
		}
	}
	return results
}

func (q *Query) FindFieldsByTag(tagKey, tagValue string) []*Field {
	if q.project == nil {
		return nil
	}

	var results []*Field
	for _, pkg := range q.project.Packages {
		for _, t := range pkg.Types {
			if t.Kind == KindStruct {
				for _, f := range t.Fields {
					if v := GetTag(f.Tag, tagKey); v != "" {
						if tagValue == "" || strings.Contains(v, tagValue) {
							results = append(results, f)
						}
					}
				}
			}
		}
	}
	return results
}

func (q *Query) ListPackages() []string {
	if q.project == nil {
		return nil
	}

	var paths []string
	for path := range q.project.Packages {
		paths = append(paths, path)
	}
	return paths
}

func (q *Query) ListTypes(pkgPath string) []*Type {
	if q.project == nil {
		return nil
	}

	pkg, ok := q.project.Packages[pkgPath]
	if !ok {
		return nil
	}

	var types []*Type
	for _, t := range pkg.Types {
		types = append(types, t)
	}
	return types
}

func (q *Query) GetStruct(name string) *Type {
	t := q.FindType(name)
	if t == nil || t.Kind != KindStruct {
		return nil
	}
	return t
}

func (q *Query) GetInterface(name string) *Type {
	t := q.FindType(name)
	if t == nil || t.Kind != KindInterface {
		return nil
	}
	return t
}

func (q *Query) ResolveTypeRef(ref *TypeRef) *Type {
	if ref == nil || q.project == nil {
		return nil
	}

	if ref.Kind == KindBasic {
		return nil
	}

	if ref.Kind == KindPointer && ref.Name != "" {
		return q.FindType(ref.Name)
	}

	if ref.Kind == KindSlice || ref.Kind == KindMap {
		return nil
	}

	if ref.Name != "" {
		return q.FindType(ref.Name)
	}

	return nil
}

func (q *Query) ResolveParamType(param *Param) *Type {
	if param == nil {
		return nil
	}
	return q.ResolveTypeRef(param.Type)
}

func (q *Query) ResolveFieldType(field *Field) *Type {
	if field == nil {
		return nil
	}
	return q.ResolveTypeRef(field.Type)
}

func (q *Query) GetMethodParams(method *Func) (params, results []*Type) {
	if method == nil {
		return nil, nil
	}

	for _, p := range method.Params {
		if t := q.ResolveParamType(p); t != nil {
			params = append(params, t)
		}
	}

	for _, r := range method.Results {
		if t := q.ResolveParamType(r); t != nil {
			results = append(results, t)
		}
	}

	return params, results
}

func (q *Query) FindEnum(name string) *Enum {
	if q.project == nil {
		return nil
	}

	for _, pkg := range q.project.Packages {
		if pkg.Enums != nil {
			if e, ok := pkg.Enums[name]; ok {
				return e
			}
		}
	}
	return nil
}

func (q *Query) ListEnums() []*Enum {
	if q.project == nil {
		return nil
	}

	var enums []*Enum
	for _, pkg := range q.project.Packages {
		if pkg.Enums != nil {
			for _, e := range pkg.Enums {
				enums = append(enums, e)
			}
		}
	}
	return enums
}

func (q *Query) GetEnumValues(enumName string) []*EnumValue {
	e := q.FindEnum(enumName)
	if e == nil {
		return nil
	}
	return e.Values
}
