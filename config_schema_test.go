package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
)

func TestConfigSchemaValidatesExample(t *testing.T) {
	schema := compileConfigSchema(t)
	instance := yamlFileAsJSONValue(t, "config.example.yaml")

	require.NoError(t, schema.Validate(instance))
}

func TestConfigSchemaRejectsMismatchedDestinationBlock(t *testing.T) {
	schema := compileConfigSchema(t)
	instance := yamlBytesAsJSONValue(t, []byte(`
token:
  prefix: tt-
targets:
  - name: deploy
    gitlab:
      type: project
      path: group/example-project
    generatedToken:
      scopes:
        - read_repository
      lifetime: 5d
    rotation:
      threshold: 3d
      gracePeriod: 0d
    destination:
      type: none
      vault:
        mount: kv
        path: ignored
        key: ignored
`))

	require.Error(t, schema.Validate(instance))
}

func compileConfigSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()

	schemaJSON, err := os.Open("config.schema.json")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, schemaJSON.Close())
	}()

	doc, err := jsonschema.UnmarshalJSON(schemaJSON)
	require.NoError(t, err)

	compiler := jsonschema.NewCompiler()
	require.NoError(t, compiler.AddResource("config.schema.json", doc))

	schema, err := compiler.Compile("config.schema.json")
	require.NoError(t, err)
	return schema
}

func yamlFileAsJSONValue(t *testing.T, path string) any {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return yamlBytesAsJSONValue(t, content)
}

func yamlBytesAsJSONValue(t *testing.T, content []byte) any {
	t.Helper()

	var yamlValue any
	require.NoError(t, yaml.Unmarshal(content, &yamlValue))

	jsonBytes, err := json.Marshal(yamlValue)
	require.NoError(t, err)

	var jsonValue any
	decoder := json.NewDecoder(bytes.NewReader(jsonBytes))
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&jsonValue))
	return jsonValue
}
