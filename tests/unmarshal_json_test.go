package tests_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	testAdditionalProperties "github.com/kainwinterheart/go-jsonschema/tests/data/core/additionalProperties"
	testAllOf "github.com/kainwinterheart/go-jsonschema/tests/data/core/allOf"
	testAnyOf "github.com/kainwinterheart/go-jsonschema/tests/data/core/anyOf"
	test "github.com/kainwinterheart/go-jsonschema/tests/data/extraImports/gopkgYAMLv3"
	testValudationRequiredFields "github.com/kainwinterheart/go-jsonschema/tests/data/validation/requiredFields"
)

func TestJsonUnmarshalValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		json string
		target any
	}{
		{
			desc:   "requiredFields - nullable",
			json:   `{"myNullableObject": null, "myNullableString": null, "myNullableStringArray": null}`,
			target: &testValudationRequiredFields.RequiredNullableJson{},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			if err := json.Unmarshal([]byte(tC.json), tC.target); err != nil {
				t.Fatalf("unmarshal error: %s", err)
			}
		})
	}
}

func TestJsonUmarshalAnyOf(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		json string
		target any
	}{
		{
			desc: "anyOf.1 - 1",
			json: `{
				"configurations": [
					{"foo": "hello"},
					{"bar": 2.2},
					{"baz": true}
				],
				"flags": "hello"
			}`,
			target: &testAnyOf.AnyOf1Json{},
		},
		{
			desc: "anyOf.1 - 2",
			json: `{
				"configurations": [
					{"foo": "ciao"},
					{"bar": 200}
				],
				"flags": true
			}`,
			target: &testAnyOf.AnyOf1Json{},
		},
		{
			desc: "anyOf.2 - 1",
			json: `{
				"configurations": [
					{"foo": "ciao"},
					{"bar": 2},
					{"baz": false}
				]
			}`,
			target: &testAnyOf.AnyOf2Json{},
		},
		{
			desc:   "anyOf.3 - 1",
			json:   `{"foo": "ciao"}`,
			target: &testAnyOf.AnyOf3Json{},
		},
		{
			desc:   "anyOf.3 - 2",
			json:   `{"bar": 2.0}`,
			target: &testAnyOf.AnyOf3Json{},
		},
		{
			desc:   "anyOf.3 - 3",
			json:   `{"configurations": ["ciao"]}`,
			target: &testAnyOf.AnyOf3Json{},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			if err := json.Unmarshal([]byte(tC.json), tC.target); err != nil {
				t.Fatalf("unmarshal error: %s", err)
			}
		})
	}
}

func TestJsonUmarshalAllOf(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc string
		json string
		target any
	}{
		{
			desc: "allOf.1 - 1",
			json: `{
				"configurations": [
					{"foo": "hello", "bar": 2.2}
				]
			}`,
			target: &testAllOf.AllOf1Json{},
		},
		{
			desc: "allOf.2 - 1",
			json: `{
				"configurations": [
					{"foo": "hello", "bar": 2.2, "baz": true}
				]
			}`,
			target: &testAllOf.AllOf2Json{},
		},
		{
			desc: "allOf.3 - 1",
			json: `{
				"foo": "hello",
				"bar": 2.2,
				"configurations": ["ciao"]
			}`,
			target: &testAllOf.AllOf3Json{},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			if err := json.Unmarshal([]byte(tC.json), tC.target); err != nil {
				t.Fatalf("unmarshal error: %s", err)
			}
		})
	}
}

func TestJSONUnmarshalAdditionalProperties(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc   string
		json   string
		target any
		assertFn func(target any)
	}{
		{
			desc: "array",
			json: `{
				"name": "hello world",
				"property1": ["one", "two"],
				"property2": [3, 4]
			}`,
			target: &testAdditionalProperties.ArrayAdditionalPropertiesJson{},
			assertFn: func(target any) {
				addProps := target.(*testAdditionalProperties.ArrayAdditionalPropertiesJson).AdditionalProperties
				assert.Equal(t, map[string][]any{"property1": {"one", "two"}, "property2": {3.0, 4.0}}, addProps)
			},
		},
		{
			desc: "bool",
			json: `{
				"name": "hello world",
				"property1": true,
				"property2": false
			}`,
			target: &testAdditionalProperties.BoolAdditionalPropertiesJson{},
			assertFn: func(target any) {
				addProps := target.(*testAdditionalProperties.BoolAdditionalPropertiesJson).AdditionalProperties
				assert.Equal(t, map[string]bool{"property1": true, "property2": false}, addProps)
			},
		},
		{
			desc: "int",
			json: `{
				"name": "hello world",
				"property1": 1,
				"property2": 2
			}`,
			target: &testAdditionalProperties.IntAdditionalPropertiesJson{},
			assertFn: func(target any) {
				addProps := target.(*testAdditionalProperties.IntAdditionalPropertiesJson).AdditionalProperties
				assert.Equal(t, map[string]int{"property1": 1, "property2": 2}, addProps)
			},
		},
		{
			desc: "number",
			json: `{
				"name": "hello world",
				"property1": 1.1,
				"property2": 2.3
			}`,
			target: &testAdditionalProperties.NumberAdditionalPropertiesJson{},
			assertFn: func(target any) {
				addProps := target.(*testAdditionalProperties.NumberAdditionalPropertiesJson).AdditionalProperties
				assert.Equal(t, map[string]float64{"property1": 1.1, "property2": 2.3}, addProps)
			},
		},
		{
			desc: "object",
			json: `{
				"name": "hello world",
				"surname": {
					"hello": 1.1,
					"world": "what's up?"
				}
			}`,
			target: &testAdditionalProperties.ObjectAdditionalPropertiesJson{},
			assertFn: func(target any) {
				addProps := target.(*testAdditionalProperties.ObjectAdditionalPropertiesJson).AdditionalProperties
				assert.Equal(t, map[string]any{"surname": map[string]any{"hello": 1.1, "world": "what's up?"}}, addProps)
			},
		},
		{
			desc: "object with props",
			json: `{
				"foo": "foo value",
				"bar": "bar value",
				"baz": {
					"property1": "hello",
					"property2": 123
				}
			}`,
			target: &testAdditionalProperties.ObjectWithPropsAdditionalPropertiesJson{},
			assertFn: func(target any) {
				addProps := target.(*testAdditionalProperties.ObjectWithPropsAdditionalPropertiesJson).AdditionalProperties
				assert.Equal(t, map[string]any{"baz": map[string]any{"property1": "hello", "property2": 123.0}}, addProps)
			},
		},
		{
			desc: "string",
			json: `{
				"name": "hello world",
				"property1": "hello",
				"property2": "world"
			}`,
			target: &testAdditionalProperties.StringAdditionalPropertiesJson{},
			assertFn: func(target any) {
				addProps := target.(*testAdditionalProperties.StringAdditionalPropertiesJson).AdditionalProperties
				assert.Equal(t, map[string]string{"property1": "hello", "property2": "world"}, addProps)
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			if err := json.Unmarshal([]byte(tC.json), tC.target); err != nil {
				t.Fatalf("unmarshal error: %s", err)
			}

			tC.assertFn(tC.target)
		})
	}
}

func formatGopkgYAMLv3(v test.GopkgYAMLv3Json) string {
	ms := ""
	if v.MyString() != nil {
		ms = *v.MyString()
	}
	mn := 0.0
	if v.MyNumber() != nil {
		mn = *v.MyNumber()
	}
	mi := 0
	if v.MyInteger() != nil {
		mi = *v.MyInteger()
	}
	mb := false
	if v.MyBoolean() != nil {
		mb = *v.MyBoolean()
	}
	me := ""
	if v.MyEnum() != nil {
		me = string(*v.MyEnum())
	}
	return fmt.Sprintf(
		"GopkgYAMLv3Json{MyString: %s, MyNumber: %f, MyInteger: %d, MyBoolean: %t, MyNull: %v, MyEnum: %v}",
		ms, mn, mi, mb, nil, me,
	)
}

func ptr[T any](v T) *T {
	return &v
}
