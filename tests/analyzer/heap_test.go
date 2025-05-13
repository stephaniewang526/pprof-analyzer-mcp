package analyzer_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ZephyrDeng/pprof-analyzer-mcp/analyzer"
	"github.com/google/pprof/profile"
)

func TestAnalyzeHeapProfile(t *testing.T) {
	// Create a simple test profile
	testProfile := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "inuse_space", Unit: "bytes"},
			{Type: "inuse_objects", Unit: "count"},
		},
		Sample: []*profile.Sample{
			{
				Location: []*profile.Location{
					{
						ID: 1,
						Line: []profile.Line{
							{
								Function: &profile.Function{
									ID:       1,
									Name:     "TestFunction1",
									Filename: "test.go",
								},
								Line: 10,
							},
						},
					},
				},
				Value: []int64{1024, 10}, // 1024 bytes, 10 objects
				Label: map[string][]string{
					"type": {"TestType1"},
				},
			},
			{
				Location: []*profile.Location{
					{
						ID: 2,
						Line: []profile.Line{
							{
								Function: &profile.Function{
									ID:       2,
									Name:     "TestFunction2",
									Filename: "test.go",
								},
								Line: 20,
							},
						},
					},
				},
				Value: []int64{2048, 20}, // 2048 bytes, 20 objects
				Label: map[string][]string{
					"type": {"TestType2"},
				},
			},
		},
	}

	// Test text format
	t.Run("TextFormat", func(t *testing.T) {
		result, err := analyzer.AnalyzeHeapProfile(testProfile, 5, "text")
		if err != nil {
			t.Fatalf("Error analyzing heap profile with text format: %v", err)
		}

		// Check that the result contains expected information
		expectedStrings := []string{
			"Heap Profile Analysis",
			"TestFunction1",
			"TestFunction2",
			"By Function",
		}

		for _, expected := range expectedStrings {
			if !strings.Contains(result, expected) {
				t.Errorf("Expected result to contain '%s', but it doesn't.\nResult: %s", expected, result)
			}
		}
	})

	// Test markdown format
	t.Run("MarkdownFormat", func(t *testing.T) {
		result, err := analyzer.AnalyzeHeapProfile(testProfile, 5, "markdown")
		if err != nil {
			t.Fatalf("Error analyzing heap profile with markdown format: %v", err)
		}

		// Check that the result contains markdown formatting
		if !strings.Contains(result, "```text") || !strings.Contains(result, "```") {
			t.Errorf("Expected markdown result to be wrapped in code blocks, but it isn't.\nResult: %s", result)
		}
	})

	// Test JSON format
	t.Run("JSONFormat", func(t *testing.T) {
		result, err := analyzer.AnalyzeHeapProfile(testProfile, 5, "json")
		if err != nil {
			t.Fatalf("Error analyzing heap profile with JSON format: %v", err)
		}

		// Parse the JSON result
		var jsonResult map[string]interface{}
		if err := json.Unmarshal([]byte(result), &jsonResult); err != nil {
			t.Fatalf("Error parsing JSON result: %v", err)
		}

		// Check that the JSON contains expected fields
		expectedFields := []string{
			"profileType",
			"valueType",
			"valueUnit",
			"totalValue",
			"totalValueFormatted",
			"functions",
		}

		for _, field := range expectedFields {
			if _, ok := jsonResult[field]; !ok {
				t.Errorf("Expected JSON result to contain field '%s', but it doesn't.\nResult: %s", field, result)
			}
		}

		// Check that the functions array contains the expected functions
		functions, ok := jsonResult["functions"].([]interface{})
		if !ok {
			t.Errorf("Expected 'functions' field to be an array, but it isn't.\nResult: %s", result)
		} else if len(functions) != 2 {
			t.Errorf("Expected 'functions' array to contain 2 items, but it contains %d.\nResult: %s", len(functions), result)
		}
	})

	// Test flamegraph-json format
	t.Run("FlamegraphJSONFormat", func(t *testing.T) {
		result, err := analyzer.AnalyzeHeapProfile(testProfile, 5, "flamegraph-json")
		if err != nil {
			t.Fatalf("Error analyzing heap profile with flamegraph-json format: %v", err)
		}

		// Parse the JSON result
		var jsonResult map[string]interface{}
		if err := json.Unmarshal([]byte(result), &jsonResult); err != nil {
			t.Fatalf("Error parsing flamegraph JSON result: %v", err)
		}

		// Check that the JSON contains expected fields for a flamegraph
		expectedFields := []string{
			"name",
			"value",
			"children",
		}

		for _, field := range expectedFields {
			if _, ok := jsonResult[field]; !ok {
				t.Errorf("Expected flamegraph JSON result to contain field '%s', but it doesn't.\nResult: %s", field, result)
			}
		}
	})

	// Test with invalid format
	t.Run("InvalidFormat", func(t *testing.T) {
		_, err := analyzer.AnalyzeHeapProfile(testProfile, 5, "invalid-format")
		if err == nil {
			t.Error("Expected error for invalid format, but got nil")
		}
	})

	// Test with fallback to alloc_space
	t.Run("FallbackToAllocSpace", func(t *testing.T) {
		fallbackProfile := &profile.Profile{
			SampleType: []*profile.ValueType{
				{Type: "alloc_space", Unit: "bytes"}, // Not inuse_space but should work as fallback
			},
			Sample: []*profile.Sample{
				{
					Location: []*profile.Location{
						{
							ID: 1,
							Line: []profile.Line{
								{
									Function: &profile.Function{
										ID:   1,
										Name: "TestFunction",
									},
								},
							},
						},
					},
					Value: []int64{1024}, // 1024 bytes
				},
			},
		}

		result, err := analyzer.AnalyzeHeapProfile(fallbackProfile, 5, "text")
		if err != nil {
			t.Fatalf("Error analyzing heap profile with fallback type: %v", err)
		}

		if !strings.Contains(result, "TestFunction") {
			t.Errorf("Expected result to contain function name with fallback type, but it doesn't.\nResult: %s", result)
		}
	})

	// Test with zero samples
	t.Run("ZeroSamples", func(t *testing.T) {
		emptyProfile := &profile.Profile{
			SampleType: []*profile.ValueType{
				{Type: "inuse_space", Unit: "bytes"},
				{Type: "inuse_objects", Unit: "count"},
			},
			Sample: []*profile.Sample{}, // No samples
		}

		result, err := analyzer.AnalyzeHeapProfile(emptyProfile, 5, "text")
		if err != nil {
			t.Fatalf("Error analyzing heap profile with zero samples: %v", err)
		}

		if !strings.Contains(result, "Total inuse_space (bytes): 0 B") {
			t.Errorf("Expected result to show zero total for empty profile, but it doesn't.\nResult: %s", result)
		}
	})
}
