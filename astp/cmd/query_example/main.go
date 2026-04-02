package main

import (
	"fmt"

	"github.com/linxlib/fw/astp"
)

func main() {
	project, err := astp.Load(`E:\linxlib\fw_example\.astp.json`)
	if err != nil {
		fmt.Printf("Load error: %v\n", err)
		return
	}

	query := astp.NewQuery(project)

	controller := query.FindType("HelloController")
	if controller == nil {
		fmt.Println("HelloController not found")
		return
	}

	var method *astp.Func
	for _, m := range controller.Methods {
		if m.Name == "TestTypeParam" {
			method = m
			break
		}
	}

	if method == nil {
		fmt.Println("TestTypeParam method not found")
		return
	}

	fmt.Printf("Method: %s\n", method.Name)
	fmt.Printf("Params count: %d\n", len(method.Params))

	for i, p := range method.Params {
		fmt.Printf("\nParam %d: %s\n", i, p.Name)
		printTypeRef(p.Type, query, "  ")
	}
}

func printTypeRef(ref *astp.TypeRef, query *astp.Query, indent string) {
	fmt.Printf("%sTypeRef: Name=%s, PkgPath=%s, Kind=%s\n", indent, ref.Name, ref.PkgPath, ref.Kind)

	if ref.Kind == astp.KindPointer {
		fmt.Printf("%s(Pointer to)\n", indent)
	}

	if ref.Generic != nil {
		fmt.Printf("%sGeneric args:\n", indent)
		for _, arg := range ref.Generic.Args {
			printTypeRef(arg, query, indent+"  ")
		}
	}

	if ref.Kind == astp.KindMap {
		fmt.Printf("%sKey: ", indent)
		if ref.KeyType != nil {
			printTypeRef(ref.KeyType, query, indent+"  ")
		}
		fmt.Printf("%sValue: ", indent)
		if ref.ElemType != nil {
			printTypeRef(ref.ElemType, query, indent+"  ")
		}
	}

	if ref.Kind == astp.KindSlice {
		fmt.Printf("%sElem: ", indent)
		if ref.ElemType != nil {
			printTypeRef(ref.ElemType, query, indent+"  ")
		}
	}

	resolved := query.ResolveTypeRef(ref)
	if resolved != nil {
		fmt.Printf("%sResolved type: %s (%s)\n", indent, resolved.Name, resolved.Kind)
		if resolved.Kind == astp.KindStruct {
			fmt.Printf("%sFields:\n", indent)
			for _, f := range resolved.Fields {
				if f.Name == "" {
					fmt.Printf("%s  - embedded: %s\n", indent, f.Type.Name)
				} else {
					tagInfo := ""
					if f.Tag != nil && len(f.Tag) > 0 {
						tagInfo = fmt.Sprintf(" %v", f.Tag)
					}
					fmt.Printf("%s  - %s (%s)%s\n", indent, f.Name, f.Type.Name, tagInfo)
				}
			}
		}
		if resolved.Doc != nil && len(resolved.Doc.Annotations) > 0 {
			fmt.Printf("%sAnnotations: %v\n", indent, resolved.Doc.Annotations)
		}
	}
}
