package executor

import (
	"reflect"
	"sync"
	"testing"
)

func TestResolvePayload(t *testing.T) {
	results := map[string]map[string]any{
		"fetch_content": {
			"generated_content": "This is the generated content.",
			"word_count":        5,
		},
		"user_info": {
			"is_admin": true,
		},
	}
	var mu sync.Mutex

	testCases := []struct {
		name            string
		inputPayload    map[string]any
		expectedPayload map[string]any
	}{
		{
			name: "Successful placeholder replacement",
			inputPayload: map[string]any{
				"path":    "output.txt",
				"content": "@results.fetch_content.generated_content",
			},
			expectedPayload: map[string]any{
				"path":    "output.txt",
				"content": "This is the generated content.",
			},
		},
		{
			name: "Non-string values should be preserved",
			inputPayload: map[string]any{
				"count":    123,
				"is_ready": true,
				"details":  "@results.fetch_content.generated_content",
			},
			expectedPayload: map[string]any{
				"count":    123,
				"is_ready": true,
				"details":  "This is the generated content.",
			},
		},
		{
			name: "Placeholder for a non-existent action ID",
			inputPayload: map[string]any{
				"content": "@results.non_existent_action.text",
			},
			expectedPayload: map[string]any{
				"content": "",
			},
		},
		{
			name: "Placeholder for a non-existent output key",
			inputPayload: map[string]any{
				"content": "@results.fetch_content.non_existent_key",
			},
			expectedPayload: map[string]any{
				"content": "",
			},
		},
		{
			name: "String without a placeholder should be preserved",
			inputPayload: map[string]any{
				"greeting": "Hello, world!",
			},
			expectedPayload: map[string]any{
				"greeting": "Hello, world!",
			},
		},
		{
			name:            "Empty input payload",
			inputPayload:    map[string]any{},
			expectedPayload: map[string]any{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolvedPayload := resolvePayload(tc.inputPayload, results, &mu)

			if !reflect.DeepEqual(resolvedPayload, tc.expectedPayload) {
				t.Errorf("mismatched payload: \n got:  %v\n want: %v", resolvedPayload, tc.expectedPayload)
			}
		})
	}
}
