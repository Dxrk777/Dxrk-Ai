package synthtools

import (
	"gopkg.in/yaml.v3"
)

// yamlUnmarshal decodes YAML bytes into a generic value.
func yamlUnmarshal(data []byte, out *any) error {
	return yaml.Unmarshal(data, out)
}
