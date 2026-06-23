package generator

import (
	"fmt"
	"math"
	"strings"

	"github.com/atombender/go-jsonschema/pkg/codegen"
)

const (
	formatJSON = "json"
)

var ErrCannotUnmarshalEnum = fmt.Errorf("cannot unmarshal enum")

type jsonFormatter struct{}

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
			// Generate helper struct with exported fields
			helperName := declType.Name + "Helper"
			if helperName == declType.Name {
				for i := 0; !output.isUniqueTypeName(helperName) && i < math.MaxInt; i++ {
					helperName = fmt.Sprintf("%s_%d", declType.Name+"Helper", i)
				}
			}

			// Also declare Plain type alias for additionalProperties handling
			plainTypeName := typePlain
			if plainTypeName == declType.Name {
				for i := 0; !output.isUniqueTypeName(plainTypeName) && i < math.MaxInt; i++ {
					plainTypeName = fmt.Sprintf("%s_%d", typePlain, i)
				}
			}

			// Declare helper struct type
			out.Printlnf("type %s struct {", helperName)
			for _, f := range structType.Fields {
				if f.Name == additionalProperties {
					continue
				}
				exportedName := strings.ToUpper(f.Name[:1]) + f.Name[1:]
				out.Printf("\t%s ", exportedName)
				if err := f.Type.Generate(out); err != nil {
					return err
				}
				tag := fmt.Sprintf(`json:"%s"`, f.JSONName)
				if !isRequiredField(f, structType) {
					tag += ",omitempty"
				}
				out.Printf("`%s`", tag)
				out.Newline()
			}
			out.Printlnf("}")

			// Declare Plain type alias
			out.Printlnf("type %s %s", plainTypeName, declType.Name)

			// Unmarshal into helper
			out.Printlnf("var helper %s", helperName)
			out.Printlnf("if err := %s.Unmarshal(value, &helper); err != nil { return err }", formatJSON)

			// Copy values from helper to private fields via Plain alias
			out.Printlnf("var %s %s", varNamePlainStruct, plainTypeName)
			for _, f := range structType.Fields {
				if f.Name == additionalProperties {
					continue
				}
				exportedName := strings.ToUpper(f.Name[:1]) + f.Name[1:]
				out.Printlnf("%s.%s = helper.%s", varNamePlainStruct, f.Name, exportedName)
			}
		} else {
			// Original approach for structs with only exported fields
			tp := typePlain
			if tp == declType.Name {
				for i := 0; !output.isUniqueTypeName(tp) && i < math.MaxInt; i++ {
					tp = fmt.Sprintf("%s_%d", typePlain, i)
				}
			}
			out.Printlnf("type %s %s", tp, declType.Name)
			out.Printlnf("var %s %s", varNamePlainStruct, tp)
			out.Printlnf("if err := %s.Unmarshal(value, &%s); err != nil { return err }",
				formatJSON, varNamePlainStruct)
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
					out.Printlnf("if err := mapstructure.Decode(raw, &plain.AdditionalProperties); err != nil {")
					out.Indent(1)
					out.Printlnf("return err")
					out.Indent(-1)
					out.Printlnf("}")

					break
				}
			}
		}

		if !hasPrivateFields {
			out.Printlnf("*j = %s(%s)", declType.Name, varNamePlainStruct)
		} else {
			// Copy from Plain to target
			out.Printlnf("*j = %s(%s)", declType.Name, varNamePlainStruct)
		}

		out.Printlnf("return nil")
		out.Indent(-1)
		out.Printlnf("}")

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

				return
			}
		}
	}
}

func (jf *jsonFormatter) getName() string {
	return "json"
}
