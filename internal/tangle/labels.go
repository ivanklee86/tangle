package tangle

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// parseLabels splits a comma-separated "key:value,key:value" query parameter
// into a map.
//
// Anything that can't be turned into a well-formed Kubernetes label selector
// is an error rather than a silently skipped segment: dropping a pair changes
// which applications come back, and a caller who mistyped one of their filters
// would otherwise get a 200 and a result set wider than they asked for. param
// is the query parameter's name ("labels" or "excludeLabels"), interpolated
// into the message so the caller knows which one to fix.
//
// An empty raw yields an empty map and no error — an unfiltered request is
// valid.
func parseLabels(param string, raw string) (map[string]string, error) {
	labels := map[string]string{}
	if len(raw) == 0 {
		return labels, nil
	}

	for _, segment := range strings.Split(raw, ",") {
		key, value, found := strings.Cut(segment, ":")
		// strings.Cut splits on the *first* separator, so "env:test:extra"
		// would otherwise pass as {env: "test:extra"}. A colon in the
		// remainder means the segment had more than one separator.
		if !found || len(key) == 0 || len(value) == 0 || strings.Contains(value, ":") {
			return nil, fmt.Errorf("invalid label %q in %s: expected key:value", segment, param)
		}

		// Well-shaped isn't enough: Argo CD parses the selector itself, so a
		// value like "te=st" came back as a 500 from every instance, and a key
		// like " env" slipped past the duplicate check below while the
		// selector parser trimmed it back to "env". Kubernetes' own label
		// rules decide instead.
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return nil, fmt.Errorf("invalid label key %q in %s: %s", key, param, strings.Join(errs, "; "))
		}
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			return nil, fmt.Errorf("invalid label value %q in %s: %s", value, param, strings.Join(errs, "; "))
		}

		if _, duplicate := labels[key]; duplicate {
			// The two pairs are ANDed into one selector, so a repeated key is
			// either a typo or a misunderstanding. Letting the last value win
			// silently picks one of the two things the caller might have
			// meant.
			return nil, fmt.Errorf("duplicate label key %q in %s: each key may appear at most once", key, param)
		}

		labels[key] = value
	}

	return labels, nil
}

// conflictingLabels reports a key carrying the same value in both maps. That
// pair becomes "key=value,key!=value" in the selector, which no application
// can ever satisfy, so the request is a mistake rather than a query with an
// empty answer.
//
// A key in both maps with *different* values is fine — "env=test,env!=prod" is
// redundant but perfectly satisfiable — so only an exact match is reported.
// Keys are checked in sorted order so a request with several conflicts always
// names the same one.
func conflictingLabels(labels map[string]string, excludeLabels map[string]string) error {
	for _, key := range slices.Sorted(maps.Keys(labels)) {
		if excluded, ok := excludeLabels[key]; ok && excluded == labels[key] {
			return fmt.Errorf("label %q=%q appears in both labels and excludeLabels: no application can match", key, labels[key])
		}
	}

	return nil
}
