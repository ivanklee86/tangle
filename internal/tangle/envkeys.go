// This file maps TANGLE_-prefixed environment variables onto Tangle configuration keys, so that
// unrelated variables sharing the prefix are ignored instead of being decoded into the config.
package tangle

import (
	"reflect"
	"regexp"
	"slices"
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
//
// A map's keys (an ArgoCD instance name) are user-chosen, so they can't come from the schema.
// mapKeys returns the keys the config already holds at a canonical path, and a segment is matched
// against those without regard to case, over the rest of the variable name: `_` is both the env
// var delimiter and a legal character in an instance name, so TANGLE_ARGOCDS_PROD_EU_ADDRESS has
// to be able to reach an instance named prod_eu. The longest matching name wins. A variable that
// names no existing instance sets nothing: it would otherwise create a second, half-configured
// instance (#237).
func resolveConfigKey(key string, mapKeys func(path string) []string) (string, bool) {
	return resolveSegments(tangleConfigSchema, nil, strings.Split(key, "."), mapKeys)
}

func resolveSegments(node *configSchema, canonical []string, segments []string, mapKeys func(path string) []string) (string, bool) {
	if len(segments) == 0 {
		// Only leaves hold values; naming a struct or a map on its own sets nothing.
		if node.children != nil || node.values != nil {
			return "", false
		}
		return strings.Join(canonical, "."), true
	}

	switch {
	case node.children != nil:
		child, exists := node.children[segments[0]]
		if !exists {
			return "", false
		}
		return resolveSegments(child, append(canonical, child.canonical), segments[1:], mapKeys)
	case node.values != nil:
		for _, name := range mapKeysByLength(mapKeys(strings.Join(canonical, "."))) {
			nameSegments := strings.Split(strings.ToLower(strings.ReplaceAll(name, "_", ".")), ".")
			if len(nameSegments) > len(segments) || !slices.Equal(nameSegments, segments[:len(nameSegments)]) {
				continue
			}
			resolved, ok := resolveSegments(node.values, append(slices.Clone(canonical), name), segments[len(nameSegments):], mapKeys)
			if ok {
				return resolved, true
			}
		}
		return "", false
	default:
		// A leaf has no keys beneath it.
		return "", false
	}
}

// mapKeysByLength orders map keys longest first, so prod_eu is tried before prod.
func mapKeysByLength(keys []string) []string {
	sorted := slices.Clone(keys)
	slices.SortStableFunc(sorted, func(a, b string) int {
		return len(b) - len(a)
	})
	return sorted
}
