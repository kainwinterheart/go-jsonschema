package generator

import (
	"fmt"
	"math"
	"strings"

	"github.com/kainwinterheart/go-jsonschema/pkg/codegen"
)

const (
	formatJSON = "json"
)

var ErrCannotUnmarshalEnum = fmt.Errorf("cannot unmarshal enum")

type jsonFormatter struct{}

// rawType converts a codegen.Type to its raw (non-immutable) equivalent
// suitable for json.Unmarshal into helper structs.
func rawType(t codegen.Type) codegen.Type {
	if t == nil {
		return nil
	}
	switch x := t.(type) {
	case codegen.ArrayType:
		return codegen.RawArrayType{Type: rawType(x.Type)}
	case *codegen.ArrayType:
		return &codegen.RawArrayType{Type: rawType(x.Type)}
	case codegen.MapType:
		return codegen.RawMapType{KeyType: rawType(x.KeyType), ValueType: rawType(x.ValueType)}
	case *codegen.MapType:
		return &codegen.RawMapType{KeyType: rawType(x.KeyType), ValueType: rawType(x.ValueType)}
	case *codegen.PointerType:
		if x.Type == nil {
			return nil
		}
		// For pointers to NamedTypes wrapping collections, unwrap to avoid double-pointer
		// in MarshalJSON helper struct (the conversion already handles the pointer)
		if nt, ok := x.Type.(*codegen.NamedType); ok && nt.Decl != nil {
			switch nt.Decl.Type.(type) {
			case codegen.ArrayType, *codegen.ArrayType, codegen.MapType, *codegen.MapType:
				return rawType(nt.Decl.Type)
			}
		}
		return &codegen.PointerType{Type: rawType(x.Type)}
	case codegen.NamedType:
		if x.Decl.Type == nil {
			return x
		}
		return rawType(x.Decl.Type)
	case *codegen.NamedType:
		// For pointers to NamedTypes, check if the underlying type is an array or map
		// If so, convert to the corresponding raw type
		switch mt := x.Decl.Type.(type) {
		case codegen.ArrayType:
			return codegen.RawArrayType{Type: rawType(mt.Type)}
		case *codegen.ArrayType:
			return &codegen.RawArrayType{Type: rawType(mt.Type)}
		case codegen.MapType:
			return codegen.RawMapType{KeyType: rawType(mt.KeyType), ValueType: rawType(mt.ValueType)}
		case *codegen.MapType:
			return &codegen.RawMapType{KeyType: rawType(mt.KeyType), ValueType: rawType(mt.ValueType)}
		}
		return &codegen.NamedType{Package: x.Package, Decl: x.Decl}
	default:
		return t
	}
}

// rawExportedName returns the exported name of a field (first char uppercase).
func rawExportedName(fieldName string) string {
	return strings.ToUpper(fieldName[:1]) + fieldName[1:]
}

func (jf *jsonFormatter) generate(
	output *output,
	declType *codegen.TypeDecl,
	validators []validator,
) func(*codegen.Emitter) error {
	var (
		beforeValidators []validator
		afterValidators  []validator
	)

	forceBefore := false

	for _, v := range validators {
		desc := v.desc()
		if desc.beforeJSONUnmarshal {
			beforeValidators = append(beforeValidators, v)
		} else {
			afterValidators = append(afterValidators, v)
			forceBefore = forceBefore || desc.requiresRawAfter
		}
	}

	if structType, ok := declType.Type.(*codegen.StructType); ok {
		for _, f := range structType.Fields {
			if f.Name == additionalProperties {
				forceBefore = true
				break
			}
		}
	}

	return func(out *codegen.Emitter) error {
		out.Commentf("Unmarshal%s implements %s.Unmarshaler.", strings.ToUpper(formatJSON), formatJSON)
		out.Printlnf("func (j *%s) Unmarshal%s(value []byte) error {", declType.Name, strings.ToUpper(formatJSON))
		out.Indent(1)

		if forceBefore || len(beforeValidators) != 0 {
			out.Printlnf("var %s map[string]interface{}", varNameRawMap)
			out.Printlnf("if err := %s.Unmarshal(value, &%s); err != nil { return err }", formatJSON, varNameRawMap)
		}

		for _, v := range beforeValidators {
			if err := v.generate(out, "json"); err != nil {
				return fmt.Errorf("cannot generate before validators: %w", err)
			}
		}

		// Check if struct has private fields
		hasPrivateFields := false
		var structType *codegen.StructType
		if st, ok := declType.Type.(*codegen.StructType); ok {
			structType = st
			for _, f := range st.Fields {
				if len(f.Name) > 0 && f.Name[0] >= 'a' && f.Name[0] <= 'z' {
					hasPrivateFields = true
					break
				}
			}
		}

		if hasPrivateFields {
			// Generate helper struct with raw types for unmarshalling
			helperName := declType.Name + "Helper"
			if helperName == declType.Name {
				for i := 0; !output.isUniqueTypeName(helperName) && i < math.MaxInt; i++ {
					helperName = fmt.Sprintf("%s_%d", declType.Name+"Helper", i)
				}
			}

			plainTypeName := typePlain
			if plainTypeName == declType.Name {
				for i := 0; !output.isUniqueTypeName(plainTypeName) && i < math.MaxInt; i++ {
					plainTypeName = fmt.Sprintf("%s_%d", typePlain, i)
				}
			}

			// Helper struct uses raw (non-immutable) types
			out.Printlnf("type %s struct {", helperName)
			for _, f := range structType.Fields {
				if f.Name == additionalProperties {
					continue
				}
				exportedName := rawExportedName(f.Name)
				out.Printf("\t%s ", exportedName)
				rawFieldType := rawType(f.Type)
				if rawFieldType != nil {
					if err := rawFieldType.Generate(out); err != nil {
						return err
					}
				} else {
					out.Printf("interface{}")
				}
				jsonTag := f.JSONName
				if !isRequiredField(f, structType) {
					jsonTag += ",omitempty"
				}
				tag := fmt.Sprintf(`json:"%s"`, jsonTag)
				out.Printf("`%s`", tag)
				out.Newline()
			}
			out.Printlnf("}")

			out.Printlnf("type %s %s", plainTypeName, declType.Name)

			out.Printlnf("var helper %s", helperName)
			out.Printlnf("if err := %s.Unmarshal(value, &helper); err != nil { return err }", formatJSON)

			out.Printlnf("var %s %s", varNamePlainStruct, plainTypeName)
			for _, f := range structType.Fields {
				if f.Name == additionalProperties {
					continue
				}
				exportedName := rawExportedName(f.Name)
				conv := rawToImmutableConversion(f.Type, "helper."+exportedName)
				out.Printlnf("%s.%s = %s", varNamePlainStruct, f.Name, conv)
			}
		} else {
			// No private fields: use raw helper struct, then convert
			if structType == nil {
				out.Printlnf("type Plain %s", declType.Name)
				out.Printlnf("var plain Plain")
				out.Printlnf("if err := %s.Unmarshal(value, &plain); err != nil { return err }", formatJSON)
				for _, v := range afterValidators {
					if err := v.generate(out, "json"); err != nil {
						return fmt.Errorf("cannot generate after validators: %w", err)
					}
				}
				out.Printlnf("*j = %s(plain)", declType.Name)
				out.Printlnf("return nil")
				out.Indent(-1)
				out.Printlnf("}")
				return nil
			}
			rawTypeName := typePlain + "Raw"
			out.Printlnf("type %s struct {", rawTypeName)
			if structType != nil {
				for _, f := range structType.Fields {
					exportedName := rawExportedName(f.Name)
					out.Printf("\t%s ", exportedName)
					if err := rawType(f.Type).Generate(out); err != nil {
						return err
					}
					jsonTag := f.JSONName
					if !isRequiredField(f, structType) {
						jsonTag += ",omitempty"
					}
					tag := fmt.Sprintf(`json:"%s"`, jsonTag)
					out.Printf("`%s`", tag)
					out.Newline()
				}
			}
			out.Printlnf("}")

			tp := typePlain
			if tp == declType.Name {
				for i := 0; !output.isUniqueTypeName(tp) && i < math.MaxInt; i++ {
					tp = fmt.Sprintf("%s_%d", typePlain, i)
				}
			}
			out.Printlnf("type %s %s", tp, declType.Name)
			out.Printlnf("var rawStruct %s", rawTypeName)
			out.Printlnf("if err := %s.Unmarshal(value, &rawStruct); err != nil { return err }", formatJSON)
			out.Printlnf("var plain %s", tp)
			for _, f := range structType.Fields {
				exportedName := rawExportedName(f.Name)
				if f.Type == nil {
					out.Printlnf("plain.%s = rawStruct.%s", f.Name, exportedName)
					continue
				}
				conv := rawToImmutableConversion(f.Type, "rawStruct."+exportedName)
				out.Printlnf("plain.%s = %s", f.Name, conv)
			}
			out.Printlnf(varNamePlainStruct + " = plain")
		}

		for _, v := range afterValidators {
			if err := v.generate(out, "json"); err != nil {
				return fmt.Errorf("cannot generate after validators: %w", err)
			}
		}

		if structType != nil {
			for _, f := range structType.Fields {
				if f.Name == additionalProperties {
					out.Printlnf("st := reflect.TypeOf(Plain{})")
					out.Printlnf("for i := range st.NumField() {")
					out.Indent(1)
					out.Printlnf("delete(raw, st.Field(i).Name)")
					out.Printlnf("delete(raw, strings.Split(st.Field(i).Tag.Get(\"json\"), \",\")[0])")
					out.Indent(-1)
					out.Printlnf("}")
					out.Printlnf("var additionalPropsRaw map[string]interface{}")
					out.Printlnf("if err := mapstructure.Decode(raw, &additionalPropsRaw); err != nil { return err }")
					out.Printlnf("plain.AdditionalProperties = func() interface{} {")
					out.Printlnf("	if additionalPropsRaw == nil { return nil }")
					out.Printlnf("	return additionalPropsRaw")
					out.Printlnf("}()")
					break
				}
			}
		}

		if !hasPrivateFields {
			out.Printlnf("*j = %s(%s)", declType.Name, varNamePlainStruct)
		} else {
			out.Printlnf("*j = %s(%s)", declType.Name, varNamePlainStruct)
		}

		out.Printlnf("return nil")
		out.Indent(-1)
		out.Printlnf("}")

		// MarshalJSON
		if structType != nil {
			out.Printlnf("")
			out.Commentf("Marshal%s implements %s.Marshaler.", strings.ToUpper(formatJSON), formatJSON)
			out.Printlnf("func (j *%s) Marshal%s() ([]byte, error) {", declType.Name, strings.ToUpper(formatJSON))
			out.Indent(1)

			hasPrivate := false
			for _, f := range structType.Fields {
				if len(f.Name) > 0 && f.Name[0] >= 'a' && f.Name[0] <= 'z' {
					hasPrivate = true
					break
				}
			}

			if hasPrivate {
				marshalHelperName := declType.Name + "MarshalHelper"
				if marshalHelperName == declType.Name {
					for i := 0; !output.isUniqueTypeName(marshalHelperName) && i < math.MaxInt; i++ {
						marshalHelperName = fmt.Sprintf("%s_%d", declType.Name+"MarshalHelper", i)
					}
				}

				out.Printlnf("type %s struct {", marshalHelperName)
				for _, f := range structType.Fields {
					if f.Name == additionalProperties {
						continue
					}
					exportedName := rawExportedName(f.Name)
					out.Printf("\t%s ", exportedName)
					// For struct-valued maps, use original type (NamedType) instead of raw type
					rf := rawType(f.Type)
					useRawType := true
					if rf != nil {
						if rm, ok := rf.(*codegen.RawMapType); ok {
							if _, isStruct := rm.ValueType.(*codegen.StructType); isStruct {
								useRawType = false
							}
						}
					}
					if useRawType && rf != nil {
						if err := rf.Generate(out); err != nil {
							return err
						}
					} else {
						// Use original field type for struct-valued maps
						if err := f.Type.Generate(out); err != nil {
							return err
						}
					}
					jsonTag := f.JSONName
					if !isRequiredField(f, structType) {
						jsonTag += ",omitempty"
					}
					tag := fmt.Sprintf(`json:"%s"`, jsonTag)
					out.Printf("`%s`", tag)
					out.Newline()
				}
				out.Printlnf("}")

				out.Printlnf("helper := %s {", marshalHelperName)
				for _, f := range structType.Fields {
					if f.Name == additionalProperties {
						continue
					}
					exportedName := rawExportedName(f.Name)
					// For struct-valued maps, use direct assignment (original type matches)
					rf := rawType(f.Type)
					if rf != nil {
						// Check for RawMapType with StructType value
						if rm, ok := rf.(*codegen.RawMapType); ok {
							if _, isStruct := rm.ValueType.(*codegen.StructType); isStruct {
								out.Printf("\t%s: j.%s,\n", exportedName, f.Name)
								continue
							}
						}
						if _, ok := rf.(*codegen.StructType); ok {
							out.Printf("\t%s: j.%s,\n", exportedName, f.Name)
							continue
						}
					}
					conv := immutableToRawConversion(f.Type, "j."+f.Name)
					out.Printf("\t%s: %s,\n", exportedName, conv)
				}
				out.Printlnf("}")
				out.Printlnf("return json.Marshal(helper)")
			} else {
				out.Printlnf("return json.Marshal(struct {")
				for _, f := range structType.Fields {
					if f.Name == additionalProperties {
						continue
					}
					exportedName := rawExportedName(f.Name)
					jsonTag := f.JSONName
					if !isRequiredField(f, structType) {
						jsonTag += ",omitempty"
					}
					tag := fmt.Sprintf(`json:"%s"`, jsonTag)
					out.Printf("\t%s ", exportedName)
					if err := rawType(f.Type).Generate(out); err != nil {
						return err
					}
					out.Printf("`%s`\n", tag)
				}
				out.Printlnf("}{")
				for i, f := range structType.Fields {
					if f.Name == additionalProperties {
						continue
					}
					if i > 0 {
						out.Printlnf(",")
					}
					exportedName := rawExportedName(f.Name)
					conv := immutableToRawConversion(f.Type, "j."+f.Name)
					out.Printf("\t%s: %s", exportedName, conv)
				}
				out.Printlnf("}")
				out.Printlnf("})")
			}

			out.Indent(-1)
			out.Printlnf("}")
		}

		return nil
	}
}

func isRequiredField(f codegen.StructField, st *codegen.StructType) bool {
	for _, req := range st.RequiredJSONFields {
		if req == f.JSONName {
			return true
		}
	}
	return false
}

func (jf *jsonFormatter) enumMarshal(declType *codegen.TypeDecl) func(*codegen.Emitter) error {
	return func(out *codegen.Emitter) error {
		out.Commentf("Marshal%s implements %s.Marshaler.", strings.ToUpper(formatJSON), formatJSON)
		out.Printlnf("func (j *%s) Marshal%s() ([]byte, error) {", declType.Name, strings.ToUpper(formatJSON))
		out.Indent(1)
		out.Printlnf("return %s.Marshal(j.value)", formatJSON)
		out.Indent(-1)
		out.Printlnf("}")

		return nil
	}
}

func (jf *jsonFormatter) enumUnmarshal(
	declType codegen.TypeDecl,
	enumType codegen.Type,
	valueConstant *codegen.Var,
	wrapInStruct bool,
) func(*codegen.Emitter) error {
	return func(out *codegen.Emitter) error {
		out.Comment("UnmarshalJSON implements json.Unmarshaler.")
		out.Printlnf("func (j *%s) UnmarshalJSON(value []byte) error {", declType.Name)
		out.Indent(1)
		out.Printf("var v ")

		if err := enumType.Generate(out); err != nil {
			return fmt.Errorf("%w: %w", ErrCannotUnmarshalEnum, err)
		}

		out.Newline()

		varName := "v"
		if wrapInStruct {
			varName += ".value"
		}

		out.Printlnf("if err := json.Unmarshal(value, &%s); err != nil { return err }", varName)
		out.Printlnf("var ok bool")
		out.Printlnf("for _, expected := range %s {", valueConstant.Name)
		out.Printlnf("if reflect.DeepEqual(%s, expected) { ok = true; break }", varName)
		out.Printlnf("}")
		out.Printlnf("if !ok {")
		out.Printlnf(`return fmt.Errorf("invalid value (expected one of %%#v): %%#v", %s, %s)`,
			valueConstant.Name, varName)
		out.Printlnf("}")
		out.Printlnf(`*j = %s(v)`, declType.Name)
		out.Printlnf(`return nil`)
		out.Indent(-1)
		out.Printlnf("}")

		return nil
	}
}

func (jf *jsonFormatter) addImport(out *codegen.File, declType *codegen.TypeDecl) {
	out.Package.AddImport("encoding/json", "")

	if structType, ok := declType.Type.(*codegen.StructType); ok {
		for _, f := range structType.Fields {
			if f.Name == additionalProperties {
				out.Package.AddImport("reflect", "")
				out.Package.AddImport("strings", "")
				out.Package.AddImport("github.com/go-viper/mapstructure/v2", "")
			}
		}
	}
}


func (jf *jsonFormatter) getName() string {
	return "json"
}

// immutableToRawConversion generates the Go expression to convert an immutable type to its raw equivalent.
func immutableToRawConversion(t codegen.Type, expr string) string {
	switch x := t.(type) {
	case codegen.ArrayType, *codegen.ArrayType:
		innerType := getType(t)
		if innerType == nil {
			innerType = codegen.EmptyInterfaceType{}
		}
		// For NamedType wrapping MapType/ArrayType, use rawTypeName to get the raw type
		// For other NamedTypes (e.g. wrapping primitive), preserve the NamedType name
		innerResultType := getResultElementType(innerType)
		// Store element in variable first to allow taking its address for Iterator calls
		elemName := "__elem"
		innerConv := immutableToRawConversion(innerType, elemName)
		// Convert expression to underlying *immutable.List[T] type to enable method calls
		return fmt.Sprintf("func() []%s { if %s == nil { return nil }; lst := (*immutable.List[%s])(%s); l := make([]%s, lst.Len()); for i := 0; i < lst.Len(); i++ { %s := lst.Get(i); l[i] = %s }; return l }()",
			innerResultType, expr, getImmutablePointerTypeName(innerType), expr, innerResultType, elemName, innerConv)
	case codegen.MapType:
		valType := rawTypeName(getType(t))
		// Non-pointer immutable.Map can't be nil and uses manual iteration API
		return fmt.Sprintf("func() map[string]%s { m := make(map[string]%s); iter := (*immutable.Map[string,%s])(&%s).Iterator(); for iter.First(); !iter.Done(); { k, v, ok := iter.Next(); if !ok { break }; m[k] = v }; return m }()",
			valType, valType, valType, expr)
	case *codegen.MapType:
		valType := rawTypeName(getType(t))
		// Non-pointer immutable.Map can't be nil and uses manual iteration API
		return fmt.Sprintf("func() map[string]%s { m := make(map[string]%s); iter := (*immutable.Map[string,%s])(&%s).Iterator(); for iter.First(); !iter.Done(); { k, v, ok := iter.Next(); if !ok { break }; m[k] = v }; return m }()",
			valType, valType, valType, expr)
	case codegen.NullType, codegen.EmptyInterfaceType:
		return expr
	case codegen.NamedType:
		return immutableToRawConversion(x.Decl.Type, expr)
	case *codegen.NamedType:
		return immutableToRawConversion(x.Decl.Type, expr)
	case *codegen.PointerType:
		if x.Type == nil {
			return expr
		}
		// For pointers to named types wrapping collections, use direct conversion
		return immutableToRawConversionDirect(x.Type, expr)
	default:
		return expr
	}
}

// getType extracts the inner type from an ArrayType or MapType.
func getType(t codegen.Type) codegen.Type {
	switch x := t.(type) {
	case codegen.ArrayType:
		return x.Type
	case *codegen.ArrayType:
		return x.Type
	case codegen.MapType:
		return x.ValueType
	case *codegen.MapType:
		return x.ValueType
	default:
		return t
	}
}

// rawToImmutableConversion generates the Go expression to convert a raw value to its immutable type.
func rawToImmutableConversion(t codegen.Type, expr string) string {
	switch x := t.(type) {
	case codegen.ArrayType, *codegen.ArrayType:
		innerType := getType(t)
		if innerType == nil {
			innerType = codegen.EmptyInterfaceType{}
		}
		innerTypeName := getImmutablePointerTypeName(innerType)
		innerConv := rawToImmutableConversion(innerType, "v")
		// If inner type is a NamedType, we need to cast the conversion result to the named type
		if _, isNamed := innerType.(*codegen.NamedType); isNamed {
			if nt, ok := innerType.(*codegen.NamedType); ok && nt.Decl != nil {
				innerConv = fmt.Sprintf("(%s)(%s)", typeArgName(nt), innerConv)
			} else if nt, ok := innerType.(codegen.NamedType); ok && nt.Decl != nil {
				innerConv = fmt.Sprintf("(%s)(%s)", typeArgName(nt), innerConv)
			}
		}
		return fmt.Sprintf("func() *immutable.List[%s] { if %s == nil { return nil }; l := make([]%s, 0, len(%s)); for _, v := range %s { l = append(l, %s) }; return immutable.NewList(l...) }()",
			innerTypeName, expr, innerTypeName, expr, expr, innerConv)
	case codegen.MapType, *codegen.MapType:
		return fmt.Sprintf("*immutable.NewMapOf[string](nil, %s)", expr)
	case *codegen.PointerType:
		if x.Type == nil {
			return expr
		}
		// For pointers to non-array/map types, just pass through
		innerType := x.Type
		_, isCollection := innerType.(codegen.ArrayType)
		_, isPtrArray := innerType.(*codegen.ArrayType)
		_, isMap := innerType.(codegen.MapType)
		_, isPtrMap := innerType.(*codegen.MapType)
		// Also check if it's a NamedType wrapping an array or map
		if nt, ok := innerType.(*codegen.NamedType); ok {
			_, isCollection = nt.Decl.Type.(codegen.ArrayType)
			_, isPtrArray = nt.Decl.Type.(*codegen.ArrayType)
			_, isMap = nt.Decl.Type.(codegen.MapType)
			_, isPtrMap = nt.Decl.Type.(*codegen.MapType)
		}
		if !isCollection && !isPtrArray && !isMap && !isPtrMap {
			return expr
		}
		// For *PointerType wrapping NamedType wrapping collection, the expr is a pointer to the raw type
		// Pass dereferenced expr to inner conversion
		innerTypeName := getImmutablePointerTypeName(innerType)

		// Check if inner type is NamedType wrapping collection
		isNamedCollection := false
		if nt, ok := innerType.(*codegen.NamedType); ok && nt.Decl != nil {
			switch nt.Decl.Type.(type) {
			case codegen.ArrayType, *codegen.ArrayType, codegen.MapType, *codegen.MapType:
				isNamedCollection = true
			}
		}

		// For *PointerType wrapping NamedType wrapping collection:
		// The helper field is *[]T (pointer to slice), and the struct field is *NamedType (= **immutable.List[T])
		if isNamedCollection {
			if nt, ok := innerType.(*codegen.NamedType); ok && nt.Decl != nil {
				innerTypeForGen := nt.Decl.Type
				innerElemType := getType(innerTypeForGen)
				if innerElemType == nil {
					innerElemType = codegen.EmptyInterfaceType{}
				}
				innerTypeName := getImmutablePointerTypeName(innerElemType)
				innerConv := rawToImmutableConversion(innerElemType, "v")

				// Result type is the NamedType name (which is already a pointer type)
				resultTypeName := typeArgName(nt)

				return fmt.Sprintf("func() *%s { if %s == nil { return nil }; raw := %s; l := make([]%s, 0, len(raw)); for _, v := range raw { l = append(l, %s) }; nv := %s(immutable.NewList(l...)); return &nv }()",
					resultTypeName, expr, expr, innerTypeName, innerConv, resultTypeName)
			}
		}

		// For other *PointerType cases, use the standard conversion
		innerConv := rawToImmutableConversion(innerType, expr)

		// If result type starts with *, take address of v
		if strings.HasPrefix(innerTypeName, "*") {
			return fmt.Sprintf("func() %s { v := %s; if v == nil { return nil }; return &v }()",
				innerTypeName, innerConv)
		}
		return fmt.Sprintf("func() %s { v := %s; if v == nil { return nil }; return v }()",
			innerTypeName, innerConv)
	case codegen.NullType, codegen.EmptyInterfaceType:
		return expr
	case codegen.NamedType:
		switch mt := x.Decl.Type.(type) {
		case codegen.ArrayType, *codegen.ArrayType:
			innerType := getType(mt)
			if innerType == nil {
				innerType = codegen.EmptyInterfaceType{}
			}
			innerTypeName := getImmutablePointerTypeName(innerType)
			innerConv := rawToImmutableConversion(innerType, "v")
			resultType := innerTypeName
			if x.Decl != nil {
				resultType = typeArgName(x)
			}
			// NamedType wrapping ArrayType already represents the pointer type,
			// so don't add extra * to the result
			// NamedType wrapping ArrayType: helper field type depends on rawType
			// If rawType produces a pointer, dereference expr
			// For simplicity, assume helper field is the raw type (not pointer)
			return fmt.Sprintf("func() %s { if %s == nil { return nil }; l := make([]%s, 0, len(%s)); for _, v := range %s { l = append(l, %s) }; return (%s)(immutable.NewList(l...)) }()",
				resultType, expr, innerTypeName, expr, expr, innerConv, resultType)
		case codegen.MapType, *codegen.MapType:
			if x.Decl != nil {
				return fmt.Sprintf("(%s)(*immutable.NewMapOf[string](nil, %s))", typeArgName(x), expr)
			}
			return fmt.Sprintf("*immutable.NewMapOf[string](nil, %s)", expr)
		}
		// Scalar type alias (e.g. SerializableDate): helper field is interface{}, cast to named type
		if x.Decl.Type == nil && x.Decl != nil {
			return fmt.Sprintf("(%s)(%s)", typeArgName(x), expr)
		}
		return rawToImmutableConversion(x.Decl.Type, expr)
	case *codegen.NamedType:
		// The helper field is a pointer to the raw type, so we need to dereference
		switch mt := x.Decl.Type.(type) {
		case codegen.ArrayType, *codegen.ArrayType:
			innerType := getType(mt)
			if innerType == nil {
				innerType = codegen.EmptyInterfaceType{}
			}
			innerTypeName := getImmutablePointerTypeName(innerType)
			innerConv := rawToImmutableConversion(innerType, "v")
			// Cast to the named type if available
			resultType := innerTypeName
			if x.Decl != nil {
				resultType = typeArgName(x)
			}
			// NamedType wrapping ArrayType already represents the pointer type,
			// so don't add extra * to the result
			// NamedType wrapping ArrayType: helper field type depends on rawType
			// If rawType produces a pointer, dereference expr
			// For simplicity, assume helper field is the raw type (not pointer)
			return fmt.Sprintf("func() %s { if %s == nil { return nil }; l := make([]%s, 0, len(%s)); for _, v := range %s { l = append(l, %s) }; return (%s)(immutable.NewList(l...)) }()",
				resultType, expr, innerTypeName, expr, expr, innerConv, resultType)
		case codegen.MapType, *codegen.MapType:
			// Cast to the named type if available
			if x.Decl != nil {
				return fmt.Sprintf("(%s)(*immutable.NewMapOf[string](nil, %s))", typeArgName(x), expr)
			}
			return fmt.Sprintf("*immutable.NewMapOf[string](nil, %s)", expr)
		}
		// Scalar type alias (e.g. SerializableDate): helper field is interface{}, cast to named type
		if x.Decl.Type == nil && x.Decl != nil {
			return fmt.Sprintf("(%s)(%s)", typeArgName(x), expr)
		}
		return rawToImmutableConversion(x.Decl.Type, expr)
	default:
		return expr
	}
}

func getImmutablePointerTypeName(t codegen.Type) string {
	switch x := t.(type) {
	case codegen.ArrayType:
		return "*immutable.List[" + typeArgName(x.Type) + "]"
	case *codegen.ArrayType:
		return "*immutable.List[" + typeArgName(x.Type) + "]"
	case codegen.MapType:
		return "immutable.Map[" + typeArgName(x.KeyType) + ", " + typeArgName(x.ValueType) + "]"
	case *codegen.MapType:
		return "immutable.Map[" + typeArgName(x.KeyType) + ", " + typeArgName(x.ValueType) + "]"
	case *codegen.PointerType:
		return "*" + getImmutablePointerTypeName(x.Type)
	case codegen.NamedType:
		return typeArgName(x)
	case *codegen.NamedType:
		return typeArgName(x)
	case codegen.EmptyInterfaceType, codegen.NullType:
		return "interface{}"
	case codegen.PrimitiveType:
		return x.Type
	case codegen.DurationType:
		return "time.Duration"
	case codegen.CustomNameType:
		return x.Type
	case *codegen.CustomNameType:
		return x.Type
	default:
		return typeArgName(t)
	}
}

// getResultElementType returns the correct result element type for array conversions.
// For NamedType wrapping MapType/ArrayType, uses rawTypeName.
// For other types, preserves the type name.
func getResultElementType(t codegen.Type) string {
	switch x := t.(type) {
	case codegen.NamedType:
		switch x.Decl.Type.(type) {
		case codegen.MapType, *codegen.MapType, codegen.ArrayType, *codegen.ArrayType:
			return rawTypeName(t)
		default:
			return typeArgName(t)
		}
	case *codegen.NamedType:
		switch x.Decl.Type.(type) {
		case codegen.MapType, *codegen.MapType, codegen.ArrayType, *codegen.ArrayType:
			return rawTypeName(t)
		default:
			return typeArgName(t)
		}
	default:
		return typeArgName(t)
	}
}

// rawTypeName returns the raw (non-immutable, non-named) type name for use in immutableToRawConversion.
func rawTypeName(t codegen.Type) string {
	switch x := t.(type) {
	case codegen.ArrayType, *codegen.ArrayType:
		inner := getType(t)
		if inner == nil {
			inner = codegen.EmptyInterfaceType{}
		}
		return "[]" + typeArgName(inner)
	case codegen.MapType, *codegen.MapType:
		return "map[string]" + typeArgName(getType(t))
	case codegen.NamedType:
		return rawTypeName(x.Decl.Type)
	case *codegen.NamedType:
		return rawTypeName(x.Decl.Type)
	case codegen.NullType, codegen.EmptyInterfaceType:
		return "interface{}"
	case codegen.PrimitiveType:
		return x.Type
	case codegen.CustomNameType:
		return x.Type
	case *codegen.CustomNameType:
		return x.Type
	default:
		return typeArgName(t)
	}
}

func getKey(t codegen.Type) codegen.Type {
	switch x := t.(type) {
	case codegen.MapType:
		return x.KeyType
	case *codegen.MapType:
		return x.KeyType
	default:
		return t
	}
}


// immutableToRawConversionDirect converts an immutable type to raw, bypassing NamedType wrappers.
// This is needed because type aliases don't inherit methods from the underlying type.
func immutableToRawConversionDirect(t codegen.Type, expr string) string {
	// Unwrap NamedType to get the underlying type
	if nt, ok := t.(codegen.NamedType); ok {
		return immutableToRawConversionDirect(nt.Decl.Type, expr)
	}
	if pnt, ok := t.(*codegen.NamedType); ok {
		return immutableToRawConversionDirect(pnt.Decl.Type, expr)
	}
	switch t.(type) {
	case codegen.ArrayType, *codegen.ArrayType:
		innerType := getType(t)
		if innerType == nil {
			innerType = codegen.EmptyInterfaceType{}
		}
		// For NamedType wrapping MapType/ArrayType, use rawTypeName to get the raw type
		// For other NamedTypes (e.g. wrapping primitive), preserve the NamedType name
		innerResultType := getResultElementType(innerType)
		// Store element in variable first to allow taking its address for Iterator calls
		elemName := "__elem"
		innerConv := immutableToRawConversion(innerType, elemName)
		// Direct method calls on the expression (no unsafe needed)
		return fmt.Sprintf(
			"func() []%s { if %s == nil { return nil }; lst := (*immutable.List[%s])(*%s); l := make([]%s, lst.Len()); for i := 0; i < lst.Len(); i++ { %s := lst.Get(i); l[i] = %s }; return l }()",
			innerResultType, expr, innerResultType, expr, innerResultType, elemName, innerConv)
	case codegen.MapType, *codegen.MapType:
		innerType := getType(t)
		if innerType == nil {
			innerType = codegen.EmptyInterfaceType{}
		}
		innerTypeName := typeArgName(innerType)
		return fmt.Sprintf("func() map[string]%s { if %s == nil { return nil }; m := make(map[string]%s); iter := (*immutable.Map[string,%s])(&%s).Iterator(); for iter.First(); !iter.Done(); { k, v, ok := iter.Next(); if !ok { break }; m[k] = v }; return m }()",
			innerTypeName, expr, innerTypeName, innerTypeName, expr)
	case codegen.NullType, codegen.EmptyInterfaceType:
		return expr
	default:
		return expr
	}
}
