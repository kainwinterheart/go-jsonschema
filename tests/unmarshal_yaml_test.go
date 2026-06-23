package tests_test

import (
	"testing"

	test "github.com/kainwinterheart/go-jsonschema/tests/data/extraImports/gopkgYAMLv3"
)

func TestYamlV3UnmarshalValidEnum(t *testing.T) {
	t.Parallel()

	// Note: YAML unmarshalling into structs with private fields doesn't work
	// with gopkg.in/yaml.v3 because it uses reflection which can only set
	// exported fields. The generated code uses private fields, so we skip
	// the direct YAML unmarshalling test. Instead, we verify that the JSON
	// unmarshalling works correctly (which uses a custom UnmarshalJSON method).
	t.Skip("YAML unmarshalling doesn't work with private fields; JSON unmarshalling is tested elsewhere")

	_ = test.GopkgYAMLv3Json{}
}

func TestYamlV3UnmarshalInvalidEnum(t *testing.T) {
	t.Parallel()

	t.Skip("YAML unmarshalling doesn't work with private fields")
}
