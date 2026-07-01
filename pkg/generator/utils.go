package generator

import (
	"fmt"
	"sort"

	"github.com/kainwinterheart/go-jsonschema/pkg/codegen"
	"github.com/kainwinterheart/go-jsonschema/pkg/schemas"
)

const additionalProperties = "AdditionalProperties"

func sortedKeys[T any](props map[string]T) []string {
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func sortDefinitionsByName(defs schemas.Definitions) []string {
	names := make([]string, 0, len(defs))

	for name := range defs {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func isNamedType(t codegen.Type) bool {
	switch x := t.(type) {
	case *codegen.NamedType:
		return true

	case *codegen.PointerType:
		if _, ok := x.Type.(*codegen.NamedType); ok {
			return true
		}
	}

	return false
}

func isMapType(t codegen.Type) bool {
	_, isMapType := t.(*codegen.MapType)

	return isMapType
}

// typeArgName returns the type argument name for use in immutable type generation.
func typeArgName(t codegen.Type) string {
	switch x := t.(type) {
	case codegen.PrimitiveType:
		return x.Type
	case codegen.NamedType:
		if x.Decl == nil {
			return "interface{}"
		}
		if x.Package != nil {
			return x.Package.Name() + "." + x.Decl.Name
		}
		return x.Decl.Name
	case *codegen.PointerType:
		return "*" + typeArgName(x.Type)
	case *codegen.NamedType:
		if x.Decl == nil {
			return "interface{}"
		}
		if x.Package != nil {
			return x.Package.Name() + "." + x.Decl.Name
		}
		return x.Decl.Name
	case *codegen.ArrayType:
		return "[]" + typeArgName(x.Type)
	case codegen.MapType:
		return "map[" + typeArgName(x.KeyType) + "]" + typeArgName(x.ValueType)
	case *codegen.MapType:
		return "map[" + typeArgName(x.KeyType) + "]" + typeArgName(x.ValueType)
	case codegen.NullType, codegen.EmptyInterfaceType:
		return "interface{}"
	case codegen.DurationType:
		return "time.Duration"
	case codegen.CustomNameType:
		return x.Type
	case *codegen.CustomNameType:
		return x.Type
	case *codegen.StructType:
		return "interface{}"
	default:
		panic(fmt.Sprintf("unknown type in typeArgName: %T", t))
	}
}
