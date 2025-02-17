package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateApplicationsUrl(t *testing.T) {
	tests := []struct {
		domain   string
		insecure bool
		labels   map[string]string
		expected string
	}{
		{
			domain:   "test.domain",
			insecure: true,
			labels:   map[string]string{},
			expected: "http://test.domain/api/applications",
		},
		{
			domain:   "test.domain",
			insecure: false,
			labels:   map[string]string{},
			expected: "https://test.domain/api/applications",
		},
		{
			domain:   "test.domain",
			insecure: false,
			labels: map[string]string{
				"label1": "value1",
			},
			expected: "https://test.domain/api/applications?labels=label1:value1",
		},
		{
			domain:   "test.domain",
			insecure: false,
			labels: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
			expected: "https://test.domain/api/applications?labels=label1:value1,label2:value2",
		},
	}

	for _, test := range tests {
		actual := GenerateApplicationsUrl(test.domain, test.insecure, test.labels)
		assert.Equal(t, test.expected, actual)
	}
}

func TestGenerateDiffUrl(t *testing.T) {
	tests := []struct {
		domain   string
		insecure bool
		labels   map[string]string
		gitRef   string
		expected string
	}{
		{
			domain:   "test.domain",
			insecure: true,
			labels:   map[string]string{},
			gitRef:   "main",
			expected: "http://test.domain/api/diffs?gitRef=main",
		},
		{
			domain:   "test.domain",
			insecure: false,
			labels:   map[string]string{},
			gitRef:   "main",
			expected: "https://test.domain/api/diffs?gitRef=main",
		},
		{
			domain:   "test.domain",
			insecure: false,
			labels: map[string]string{
				"label1": "value1",
			},
			gitRef:   "main",
			expected: "https://test.domain/api/diffs?labels=label1:value1&gitRef=main",
		},
		{
			domain:   "test.domain",
			insecure: false,
			labels: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
			gitRef:   "main",
			expected: "https://test.domain/api/diffs?labels=label1:value1,label2:value2&gitRef=main",
		},
	}

	for _, test := range tests {
		actual := GenerateDiffUrl(test.domain, test.insecure, test.labels, test.gitRef)
		assert.Equal(t, test.expected, actual)
	}
}

func TestGetApplications(t *testing.T) {
	tests := []struct {
		name        string
		domain      string
		insecure    bool
		labels      map[string]string
		lengthTest  int
		lengthProd  int
		expectError bool
	}{
		{
			name:        "get all applications",
			domain:      "localhost:8081",
			insecure:    true,
			labels:      map[string]string{},
			lengthTest:  2,
			lengthProd:  1,
			expectError: false,
		},
		{
			name:     "get test applications",
			domain:   "localhost:8081",
			insecure: true,
			labels: map[string]string{
				"env": "test",
			},
			lengthTest:  1,
			lengthProd:  0,
			expectError: false,
		},
		{
			name:        "error",
			domain:      "1localhost:8081",
			insecure:    true,
			labels:      map[string]string{},
			lengthTest:  0,
			lengthProd:  0,
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := GetApplications(GenerateApplicationsUrl(test.domain, test.insecure, test.labels))
			if test.expectError {
				assert.Error(t, err)
			} else {
				for _, result := range resp.Results {
					if result.Name == "test" {
						assert.Len(t, result.Applications, test.lengthTest)
					} else if result.Name == "prod" {
						assert.Len(t, result.Applications, test.lengthProd)
					}
				}
			}
		})
	}
}
