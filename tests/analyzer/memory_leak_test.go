package analyzer_test

import (
	"strings"
	"testing"

	"github.com/ZephyrDeng/pprof-analyzer-mcp/analyzer"
	"github.com/google/pprof/profile"
)

func TestDetectPotentialMemoryLeaks(t *testing.T) {
	// Create a simple before profile
	beforeProfile := &profile.Profile{
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
									ID:   1,
									Name: "TestFunction",
								},
							},
						},
					},
				},
				Value: []int64{1000, 10}, // 1000 bytes, 10 objects
				Label: map[string][]string{
					"type": {"TestType"},
				},
			},
		},
	}

	// Create an after profile with growth
	afterProfile := &profile.Profile{
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
									ID:   1,
									Name: "TestFunction",
								},
							},
						},
					},
				},
				Value: []int64{2000, 20}, // 2000 bytes, 20 objects (100% growth)
				Label: map[string][]string{
					"type": {"TestType"},
				},
			},
		},
	}

	// Test with default threshold (10%)
	result, err := analyzer.DetectPotentialMemoryLeaks(beforeProfile, afterProfile, 0.1, 10)
	if err != nil {
		t.Fatalf("Error detecting memory leaks: %v", err)
	}

	// Verify the result contains the expected information
	expectedStrings := []string{
		"Memory Leak Detection Report",
		"TestType",
		"100.00%", // Growth percentage
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected result to contain '%s', but it doesn't.\nResult: %s", expected, result)
		}
	}

	// Test with higher threshold (should not detect the leak)
	result, err = analyzer.DetectPotentialMemoryLeaks(beforeProfile, afterProfile, 2.0, 10)
	if err != nil {
		t.Fatalf("Error detecting memory leaks with higher threshold: %v", err)
	}

	if !strings.Contains(result, "No significant memory growth detected") {
		t.Errorf("Expected no leaks to be detected with high threshold, but got:\n%s", result)
	}

	// Test with missing inuse_space sample type
	invalidProfile := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "some_other_type", Unit: "bytes"},
		},
	}

	_, err = analyzer.DetectPotentialMemoryLeaks(invalidProfile, afterProfile, 0.1, 10)
	if err == nil {
		t.Error("Expected error for missing inuse_space sample type, but got nil")
	}
}
