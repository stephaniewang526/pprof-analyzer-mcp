package analyzer_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ZephyrDeng/pprof-analyzer-mcp/analyzer"
	"github.com/google/pprof/profile"
)

func TestAnalyzeAllocsProfile(t *testing.T) {
	// Create a simple test profile
	testProfile := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "alloc_space", Unit: "bytes"},
			{Type: "alloc_objects", Unit: "count"},
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
		result, err := analyzer.AnalyzeAllocsProfile(testProfile, 5, "text")
		if err != nil {
			t.Fatalf("Error analyzing allocs profile with text format: %v", err)
		}

		// Check that the result contains expected information
		expectedStrings := []string{
			"Allocation Profile Analysis",
			"TestFunction1",
			"TestFunction2",
			"By Function",
			"By Allocation Site",
		}

		for _, expected := range expectedStrings {
			if !strings.Contains(result, expected) {
				t.Errorf("Expected result to contain '%s', but it doesn't.\nResult: %s", expected, result)
			}
		}
	})

	// Test markdown format
	t.Run("MarkdownFormat", func(t *testing.T) {
		result, err := analyzer.AnalyzeAllocsProfile(testProfile, 5, "markdown")
		if err != nil {
			t.Fatalf("Error analyzing allocs profile with markdown format: %v", err)
		}

		// Check that the result contains markdown formatting
		if !strings.Contains(result, "```text") || !strings.Contains(result, "```") {
			t.Errorf("Expected markdown result to be wrapped in code blocks, but it isn't.\nResult: %s", result)
		}
	})

	// Test JSON format
	t.Run("JSONFormat", func(t *testing.T) {
		result, err := analyzer.AnalyzeAllocsProfile(testProfile, 5, "json")
		if err != nil {
			t.Fatalf("Error analyzing allocs profile with JSON format: %v", err)
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
			"allocationSites",
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
		result, err := analyzer.AnalyzeAllocsProfile(testProfile, 5, "flamegraph-json")
		if err != nil {
			t.Fatalf("Error analyzing allocs profile with flamegraph-json format: %v", err)
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
		_, err := analyzer.AnalyzeAllocsProfile(testProfile, 5, "invalid-format")
		if err == nil {
			t.Error("Expected error for invalid format, but got nil")
		}
	})

	// Test with missing alloc_space sample type
	t.Run("MissingAllocSpace", func(t *testing.T) {
		invalidProfile := &profile.Profile{
			SampleType: []*profile.ValueType{
				{Type: "some_other_type", Unit: "bytes"},
			},
		}

		// The implementation falls back to using whatever sample type is available
		// rather than returning an error, so we should check that it works
		result, err := analyzer.AnalyzeAllocsProfile(invalidProfile, 5, "text")
		if err != nil {
			t.Fatalf("Unexpected error for missing alloc_space sample type: %v", err)
		}
		
		// Check that the result contains the fallback type
		if !strings.Contains(result, "some_other_type") {
			t.Errorf("Expected result to contain fallback type name, but it doesn't.\nResult: %s", result)
		}
	})

	// Test with fallback to other allocation type
	t.Run("FallbackToAlloc", func(t *testing.T) {
		fallbackProfile := &profile.Profile{
			SampleType: []*profile.ValueType{
				{Type: "alloc", Unit: "bytes"}, // Not alloc_space but should work as fallback
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

		result, err := analyzer.AnalyzeAllocsProfile(fallbackProfile, 5, "text")
		if err != nil {
			t.Fatalf("Error analyzing allocs profile with fallback type: %v", err)
		}

		if !strings.Contains(result, "TestFunction") {
			t.Errorf("Expected result to contain function name with fallback type, but it doesn't.\nResult: %s", result)
		}
	})

	// Test with zero samples
	t.Run("ZeroSamples", func(t *testing.T) {
		emptyProfile := &profile.Profile{
			SampleType: []*profile.ValueType{
				{Type: "alloc_space", Unit: "bytes"},
				{Type: "alloc_objects", Unit: "count"},
			},
			Sample: []*profile.Sample{}, // No samples
		}

		result, err := analyzer.AnalyzeAllocsProfile(emptyProfile, 5, "text")
		if err != nil {
			t.Fatalf("Error analyzing allocs profile with zero samples: %v", err)
		}

		if !strings.Contains(result, "Total alloc_space (bytes): 0 B") {
			t.Errorf("Expected result to show zero total for empty profile, but it doesn't.\nResult: %s", result)
		}
	})
}

// TestAnalyzeAllocsProfileWithRealProfiles tests the AnalyzeAllocsProfile function with real profiles if available
func TestAnalyzeAllocsProfileWithRealProfiles(t *testing.T) {
	// This test is skipped if the heap profiles don't exist
	// It's meant to be run manually after generating profiles with the example program
	t.Skip("Skipping test with real profiles - run manually after generating profiles")

	/*
		// To run this test:
		// 1. Generate profiles with the example program
		// 2. Remove the t.Skip line above
		// 3. Run the test with: go test -v -run TestAnalyzeAllocsProfileWithRealProfiles

		// Load a real profile
		profilePath := "../../examples/profiles/heap_after.pprof"
		f, err := os.Open(profilePath)
		if err != nil {
			t.Skipf("Skipping test with real profile: %v", err)
		}
		defer f.Close()

		prof, err := profile.Parse(f)
		if err != nil {
			t.Fatalf("Error parsing profile: %v", err)
		}

		// Test all formats
		formats := []string{"text", "markdown", "json", "flamegraph-json"}
		for _, format := range formats {
			result, err := analyzer.AnalyzeAllocsProfile(prof, 10, format)
			if err != nil {
				t.Errorf("Error analyzing real profile with format %s: %v", format, err)
			}
			if len(result) == 0 {
				t.Errorf("Empty result for format %s", format)
			}
		}
	*/
}
