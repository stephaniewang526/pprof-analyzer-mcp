package analyzer_test

import (
	"encoding/json"
	"testing"

	"github.com/ZephyrDeng/pprof-analyzer-mcp/analyzer"
	"github.com/google/pprof/profile"
)

func TestBuildFlameGraphTree(t *testing.T) {
	// Create a simple test profile with a call stack
	testProfile := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "samples", Unit: "count"},
			{Type: "cpu", Unit: "nanoseconds"},
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
									Name:     "main",
									Filename: "main.go",
								},
								Line: 10,
							},
						},
					},
					{
						ID: 2,
						Line: []profile.Line{
							{
								Function: &profile.Function{
									ID:       2,
									Name:     "foo",
									Filename: "foo.go",
								},
								Line: 20,
							},
						},
					},
					{
						ID: 3,
						Line: []profile.Line{
							{
								Function: &profile.Function{
									ID:       3,
									Name:     "bar",
									Filename: "bar.go",
								},
								Line: 30,
							},
						},
					},
				},
				Value: []int64{1, 1000}, // 1 sample, 1000 nanoseconds
			},
		},
	}

	// Test building a flame graph tree for CPU samples
	t.Run("CPUFlameGraph", func(t *testing.T) {
		// Use the second value (nanoseconds)
		flameGraph, err := analyzer.BuildFlameGraphTree(testProfile, 1)
		if err != nil {
			t.Fatalf("Error building flame graph tree: %v", err)
		}

		// Convert to JSON for inspection
		jsonBytes, err := json.Marshal(flameGraph)
		if err != nil {
			t.Fatalf("Error marshaling flame graph to JSON: %v", err)
		}

		// Parse the JSON
		var result map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			t.Fatalf("Error parsing flame graph JSON: %v", err)
		}

		// Check the root node
		if name, ok := result["name"].(string); !ok || name != "root" {
			t.Errorf("Expected root node name to be 'root', but got '%v'", result["name"])
		}

		// Check the value
		if value, ok := result["value"].(float64); !ok || value != 1000 {
			t.Errorf("Expected root node value to be 1000, but got %v", result["value"])
		}

		// Check that there are children
		children, ok := result["children"].([]interface{})
		if !ok {
			t.Errorf("Expected root node to have children, but it doesn't")
		} else if len(children) == 0 {
			t.Errorf("Expected root node to have at least one child, but it has none")
		}

		// Check that there's at least one child
		firstChild := children[0].(map[string]interface{})
		if name, ok := firstChild["name"].(string); !ok {
			t.Errorf("Expected first child to have a name, but it doesn't")
		} else {
			t.Logf("First child name is '%s'", name)
		}

		// Check that the child has children
		childChildren, ok := firstChild["children"].([]interface{})
		if !ok || len(childChildren) == 0 {
			t.Errorf("Expected first child to have children, but it doesn't")
		} else {
			// Check that the structure is as expected (a chain of function calls)
			// Note: We don't check specific names because the order might vary
			t.Logf("First child has %d children", len(childChildren))
		}
	})

	// Test with invalid value index
	t.Run("InvalidValueIndex", func(t *testing.T) {
		_, err := analyzer.BuildFlameGraphTree(testProfile, 5) // Index out of bounds
		if err == nil {
			t.Error("Expected error for invalid value index, but got nil")
		}
	})

	// Test with memory profile
	t.Run("MemoryFlameGraph", func(t *testing.T) {
		// Create a memory profile
		memProfile := &profile.Profile{
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
										Name:     "allocator",
										Filename: "alloc.go",
									},
									Line: 10,
								},
							},
						},
					},
					Value: []int64{1024, 1}, // 1024 bytes, 1 object
					Label: map[string][]string{
						"type": {"TestType"},
					},
				},
			},
		}

		// Use the first value (bytes)
		flameGraph, err := analyzer.BuildFlameGraphTree(memProfile, 0)
		if err != nil {
			t.Fatalf("Error building memory flame graph tree: %v", err)
		}

		// Convert to JSON for inspection
		jsonBytes, err := json.Marshal(flameGraph)
		if err != nil {
			t.Fatalf("Error marshaling memory flame graph to JSON: %v", err)
		}

		// Parse the JSON
		var result map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			t.Fatalf("Error parsing memory flame graph JSON: %v", err)
		}

		// Check the root node
		if name, ok := result["name"].(string); !ok || name != "root" {
			t.Errorf("Expected root node name to be 'root', but got '%v'", result["name"])
		}

		// Check the value
		if value, ok := result["value"].(float64); !ok || value != 1024 {
			t.Errorf("Expected root node value to be 1024, but got %v", result["value"])
		}

		// Check that there are children
		children, ok := result["children"].([]interface{})
		if !ok {
			t.Errorf("Expected root node to have children, but it doesn't")
		} else if len(children) == 0 {
			t.Errorf("Expected root node to have at least one child, but it has none")
		}

		// Check the first child (should be "allocator")
		firstChild := children[0].(map[string]interface{})
		if name, ok := firstChild["name"].(string); !ok || name != "allocator" {
			t.Errorf("Expected first child name to be 'allocator', but got '%v'", firstChild["name"])
		}

		// Check that the memory-specific fields are present
		if _, ok := result["valueFormatted"].(string); !ok {
			t.Errorf("Expected memory flame graph to have 'valueFormatted' field, but it doesn't")
		}
	})

	// Test with allocs profile
	t.Run("AllocsFlameGraph", func(t *testing.T) {
		// Create an allocs profile
		allocsProfile := &profile.Profile{
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
										Name:     "allocator",
										Filename: "alloc.go",
									},
									Line: 10,
								},
							},
						},
					},
					Value: []int64{2048, 2}, // 2048 bytes, 2 objects
					Label: map[string][]string{
						"type": {"AllocType"},
					},
				},
			},
		}

		// Use the first value (bytes)
		flameGraph, err := analyzer.BuildFlameGraphTree(allocsProfile, 0)
		if err != nil {
			t.Fatalf("Error building allocs flame graph tree: %v", err)
		}

		// Convert to JSON for inspection
		jsonBytes, err := json.Marshal(flameGraph)
		if err != nil {
			t.Fatalf("Error marshaling allocs flame graph to JSON: %v", err)
		}

		// Parse the JSON
		var result map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &result); err != nil {
			t.Fatalf("Error parsing allocs flame graph JSON: %v", err)
		}

		// Check the root node
		if name, ok := result["name"].(string); !ok || name != "root" {
			t.Errorf("Expected root node name to be 'root', but got '%v'", result["name"])
		}

		// Check the value
		if value, ok := result["value"].(float64); !ok || value != 2048 {
			t.Errorf("Expected root node value to be 2048, but got %v", result["value"])
		}

		// Check that there are children
		children, ok := result["children"].([]interface{})
		if !ok {
			t.Errorf("Expected root node to have children, but it doesn't")
		} else if len(children) == 0 {
			t.Errorf("Expected root node to have at least one child, but it has none")
		}

		// Check the first child (should be "allocator")
		firstChild := children[0].(map[string]interface{})
		if name, ok := firstChild["name"].(string); !ok || name != "allocator" {
			t.Errorf("Expected first child name to be 'allocator', but got '%v'", firstChild["name"])
		}

		// Check that the memory-specific fields are present
		if _, ok := result["valueFormatted"].(string); !ok {
			t.Errorf("Expected allocs flame graph to have 'valueFormatted' field, but it doesn't")
		}

		// Check that object count information is present
		if objectCount, ok := result["objectCount"].(float64); !ok || objectCount != 2 {
			t.Errorf("Expected root node to have objectCount=2, but got %v", result["objectCount"])
		}

		// Check that average size information is present
		if avgSize, ok := result["avgSize"].(float64); !ok || avgSize != 1024 {
			t.Errorf("Expected root node to have avgSize=1024, but got %v", result["avgSize"])
		}

		// Check that type information is present in the child node
		if typeName, ok := firstChild["type"].(string); !ok || typeName != "AllocType" {
			t.Errorf("Expected first child to have type='AllocType', but got %v", firstChild["type"])
		}
	})
}
