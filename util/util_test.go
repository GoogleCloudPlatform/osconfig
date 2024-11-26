package util

import (
	"testing"
)

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
	}{
		{
			name:           "Basic file name",
			input:          "test.yaml",
			expectedOutput: "test.yaml",
		},
		{
			name:           "Basic full path",
			input:          "/x/test.yaml",
			expectedOutput: "/x/test.yaml",
		},
		{
			name:           "Relative path",
			input:          "x/test.yaml",
			expectedOutput: "x/test.yaml",
		},
		{
			name:           "Relative path with traversal segment",
			input:          "../x/test.yaml",
			expectedOutput: "x/test.yaml",
		},
		{
			name:           "Relative path with traversal segment",
			input:          "/../x/test.yaml",
			expectedOutput: "/x/test.yaml",
		},
	}

	for _, tt := range tests {
		if result := SanitizePath(tt.input); result != tt.expectedOutput {
			t.Errorf("Test %q failed, expectedOutput %q, got %q", tt.name, tt.expectedOutput, result)
		}

	}
}
