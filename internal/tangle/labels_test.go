package tangle

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseLabels covers the promise the API makes about its label query
// parameters: every pair a caller sends either reaches the selector, or the
// request is refused naming the pair that couldn't. Silently dropping one
// returns a result set wider than the caller asked for, with nothing in the
// response to say so.
//
// The error text is asserted, not just the fact of an error — the message is
// what the caller receives in the ErrorResponse body, so it's part of the
// contract rather than an implementation detail.
func TestParseLabels(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    map[string]string
		wantErr string
	}{
		{
			name: "empty",
			raw:  "",
			want: map[string]string{},
		},
		{
			name: "single pair",
			raw:  "env:test",
			want: map[string]string{"env": "test"},
		},
		{
			name: "multiple pairs",
			raw:  "env:test,foo:bar",
			want: map[string]string{"env": "test", "foo": "bar"},
		},
		{
			name: "same key and value repeated across different keys",
			raw:  "env:shared,team:shared",
			want: map[string]string{"env": "shared", "team": "shared"},
		},
		{
			name:    "no separator",
			raw:     "env",
			wantErr: `invalid label "env" in %s: expected key:value`,
		},
		{
			name:    "too many separators",
			raw:     "env:test:extra",
			wantErr: `invalid label "env:test:extra" in %s: expected key:value`,
		},
		{
			name:    "trailing comma",
			raw:     "env:test,",
			wantErr: `invalid label "" in %s: expected key:value`,
		},
		{
			name:    "empty key",
			raw:     ":test",
			wantErr: `invalid label ":test" in %s: expected key:value`,
		},
		{
			name:    "empty value",
			raw:     "env:",
			wantErr: `invalid label "env:" in %s: expected key:value`,
		},
		{
			// The valid pair must not rescue the request: returning
			// {team: platform} would silently answer a narrower question
			// than the caller asked.
			name:    "malformed alongside valid",
			raw:     "env,team:platform",
			wantErr: `invalid label "env" in %s: expected key:value`,
		},
		{
			name:    "duplicate key",
			raw:     "env:test,env:prod",
			wantErr: `duplicate label key "env" in %s: each key may appear at most once`,
		},
		{
			// Still rejected even though the last-value-wins result would
			// have been correct — a caller who typed it twice meant
			// something, and guessing which is how the bug started.
			name:    "duplicate key, same value",
			raw:     "env:test,env:test",
			wantErr: `duplicate label key "env" in %s: each key may appear at most once`,
		},
	}

	// Run every case against both parameter names, so the message is proven
	// to name the parameter the caller actually sent rather than a
	// hard-coded one.
	for _, param := range []string{"labels", "excludeLabels"} {
		for _, test := range tests {
			t.Run(param+"/"+test.name, func(t *testing.T) {
				got, err := parseLabels(param, test.raw)

				if test.wantErr != "" {
					assert.Error(t, err)
					assert.Equal(t, fmt.Sprintf(test.wantErr, param), err.Error())
					assert.Nil(t, got, "no partial map on error")
					return
				}

				assert.NoError(t, err)
				assert.Equal(t, test.want, got)
			})
		}
	}
}

// TestConflictingLabels pins the one cross-parameter rule: a key carrying the
// same value in both maps becomes "key=value,key!=value", which no
// application can satisfy. A key in both with different values is redundant
// but answerable, and must not be rejected.
func TestConflictingLabels(t *testing.T) {
	tests := []struct {
		name          string
		labels        map[string]string
		excludeLabels map[string]string
		wantErr       string
	}{
		{
			name:          "no overlap",
			labels:        map[string]string{"foo": "bar"},
			excludeLabels: map[string]string{"env": "test"},
		},
		{
			name:          "same key, different values",
			labels:        map[string]string{"env": "test"},
			excludeLabels: map[string]string{"env": "prod"},
		},
		{
			name:          "same key, same value",
			labels:        map[string]string{"env": "test"},
			excludeLabels: map[string]string{"env": "test"},
			wantErr:       `label "env"="test" appears in both labels and excludeLabels: no application can match`,
		},
		{
			// Sorted iteration means the reported key doesn't depend on Go's
			// map ordering, so this assertion is stable rather than flaky.
			name:          "several conflicts reports the first by key order",
			labels:        map[string]string{"zone": "a", "env": "test"},
			excludeLabels: map[string]string{"zone": "a", "env": "test"},
			wantErr:       `label "env"="test" appears in both labels and excludeLabels: no application can match`,
		},
		{
			name:          "both empty",
			labels:        map[string]string{},
			excludeLabels: map[string]string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := conflictingLabels(test.labels, test.excludeLabels)

			if test.wantErr != "" {
				assert.Error(t, err)
				assert.Equal(t, test.wantErr, err.Error())
				return
			}

			assert.NoError(t, err)
		})
	}
}
