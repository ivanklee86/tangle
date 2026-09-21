// This file maps TANGLE_-prefixed environment variables onto Tangle configuration keys, so that
// unrelated variables sharing the prefix are ignored instead of being decoded into the config.
package tangle

import (
	"reflect"
	"regexp"
	"strings"
)

// kubernetesServiceLinkValue matches the Docker-link-style URL Kubernetes puts in <SERVICE>_PORT
// for every Service sharing a namespace with a pod (e.g. "tcp://10.43.1.2:8080"). No Tangle
// configuration value takes this form, so the shape alone identifies an injected variable.
var kubernetesServiceLinkValue = regexp.MustCompile(`^(tcp|udp|sctp)://`)

// configSchema is one node of TangleConfig's key tree, built by reflecting over its `koanf` tags.
// Exactly one of children (a struct's keys), values (a map's element type) or neither (a leaf that
// holds a value) is set.
type configSchema struct {
	canonical string
	children  map[string]*configSchema
	values    *configSchema
}

var tangleConfigSchema = newConfigSchema(reflect.TypeOf(TangleConfig{}))

func newConfigSchema(configType reflect.Type) *configSchema {
	for configType.Kind() == reflect.Pointer {
		configType = configType.Elem()
	}

	schema := &configSchema{}
	switch configType.Kind() {
	case reflect.Struct:
		schema.children = map[string]*configSchema{}
		for i := range configType.NumField() {
			field := configType.Field(i)
			// Untagged fields (e.g. DoNotInstrument) are set programmatically, not by config.
			tag := field.Tag.Get("koanf")
			if tag == "" || tag == "-" {
				continue
			}

			child := newConfigSchema(field.Type)
			child.canonical = tag
			schema.children[strings.ToLower(tag)] = child
		}
	case reflect.Map:
		schema.values = newConfigSchema(configType.Elem())
	}

	return schema
}

// resolveConfigKey translates a lowercased, dot-delimited environment variable key into the
// canonically-spelled config key it sets, reporting false when TangleConfig has no such key.
// Canonicalizing the spelling (sortorder → sortOrder) is what lets an env var override the value
// the YAML file set under the same key rather than landing beside it as a second, distinct key.
func resolveConfigKey(key string) (string, bool) {
	node := tangleConfigSchema
	canonical := []string{}

	for _, segment := range strings.Split(key, ".") {
		switch {
		case node.children != nil:
			child, exists := node.children[segment]
			if !exists {
				return "", false
			}
			canonical = append(canonical, child.canonical)
			node = child
		case node.values != nil:
			// A map's keys are user-defined (e.g. an ArgoCD instance name), so they pass
			// through as written.
			canonical = append(canonical, segment)
			node = node.values
		default:
			// A leaf has no keys beneath it.
			return "", false
		}
	}

	// Only leaves hold values; naming a struct or a map on its own sets nothing.
	if node.children != nil || node.values != nil {
		return "", false
	}

	return strings.Join(canonical, "."), true
}
