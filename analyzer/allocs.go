package analyzer

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

// AnalyzeAllocsProfile analyzes an Allocs profile (allocation patterns) and returns formatted results.
func AnalyzeAllocsProfile(p *profile.Profile, topN int, format string) (string, error) {
	log.Printf("Analyzing Allocs profile (Top %d, Format: %s)", topN, format)

	// --- 1. Find the 'alloc_space' sample value index ---
	valueIndex := -1
	objectsIndex := -1 // For tracking object counts

	for i, st := range p.SampleType {
		if st.Type == "alloc_space" && st.Unit == "bytes" {
			valueIndex = i
		}
		if st.Type == "alloc_objects" && st.Unit == "count" {
			objectsIndex = i
		}
	}

	// If alloc_space is not found, try other possible memory allocation types
	if valueIndex == -1 && len(p.SampleType) > 0 {
		for i, st := range p.SampleType {
			if (st.Type == "alloc" || st.Type == "allocation") && st.Unit == "bytes" {
				valueIndex = i
				log.Printf("Warning: 'alloc_space' not found, using '%s/%s' instead", st.Type, st.Unit)
				break
			}
		}
	}

	// Final fallback
	if valueIndex == -1 && len(p.SampleType) > 0 {
		valueIndex = 0 // Use the first sample type
		log.Printf("Warning: Could not find allocation space sample type, defaulting to index 0: %s/%s",
			p.SampleType[valueIndex].Type, p.SampleType[valueIndex].Unit)
	}

	if valueIndex == -1 {
		return "", fmt.Errorf("could not determine value type from profile sample types (e.g., alloc_space bytes)")
	}

	valueUnit := p.SampleType[valueIndex].Unit
	valueType := p.SampleType[valueIndex].Type
	log.Printf("Using index %d (%s/%s) for Allocs analysis", valueIndex, valueType, valueUnit)

	// --- 2. Aggregate memory allocation values by function and allocation site ---
	// Create two maps: one for aggregating by function, one for aggregating by allocation site
	funcValue := make(map[string]int64)        // Aggregate by function name
	allocSiteValue := make(map[string]int64)   // Aggregate by allocation site (function+file+line)
	funcObjects := make(map[string]int64)      // Object count aggregated by function
	allocSiteObjects := make(map[string]int64) // Object count aggregated by allocation site

	totalValue := int64(0)
	totalObjects := int64(0)

	for _, s := range p.Sample {
		if len(s.Location) > 0 && len(s.Value) > valueIndex {
			v := s.Value[valueIndex] // Allocated bytes
			totalValue += v

			// If object count information is available, collect it too
			var objCount int64 = 0
			if objectsIndex >= 0 && len(s.Value) > objectsIndex {
				objCount = s.Value[objectsIndex]
				totalObjects += objCount
			}

			// Attribute memory to the topmost function in the allocation stack
			loc := s.Location[0]
			for _, line := range loc.Line {
				if line.Function != nil {
					funcName := line.Function.Name
					fileName := line.Function.Filename
					lineNum := line.Line

					// Aggregate by function
					funcValue[funcName] += v
					if objCount > 0 {
						funcObjects[funcName] += objCount
					}

					// Aggregate by allocation site (function+file+line)
					allocSiteKey := fmt.Sprintf("%s at %s:%d", funcName, fileName, lineNum)
					allocSiteValue[allocSiteKey] += v
					if objCount > 0 {
						allocSiteObjects[allocSiteKey] += objCount
					}

					break // Only attribute to the first function found in the top frame
				}
			}
		}
	}

	if totalValue == 0 {
		log.Printf("Warning: Total value for the selected sample type (%s/%s) is zero.", valueType, valueUnit)
	}

	// --- 3. Sort functions and allocation sites by aggregated values ---
	// Sort by function
	funcStats := make([]functionStat, 0, len(funcValue))
	for name, val := range funcValue {
		funcStats = append(funcStats, functionStat{Name: name, Flat: val})
	}
	sort.Slice(funcStats, func(i, j int) bool {
		return funcStats[i].Flat > funcStats[j].Flat // Sort in descending order
	})

	// Sort by allocation site
	type allocSiteStat struct {
		Site  string
		Value int64
		Count int64
	}
	allocSiteStats := make([]allocSiteStat, 0, len(allocSiteValue))
	for site, val := range allocSiteValue {
		count := allocSiteObjects[site]
		allocSiteStats = append(allocSiteStats, allocSiteStat{Site: site, Value: val, Count: count})
	}
	sort.Slice(allocSiteStats, func(i, j int) bool {
		return allocSiteStats[i].Value > allocSiteStats[j].Value // Sort in descending order
	})

	// --- 4. Format output ---
	var b strings.Builder
	limit := topN
	if limit > len(funcStats) {
		limit = len(funcStats)
	}

	allocSiteLimit := limit
	if allocSiteLimit > len(allocSiteStats) {
		allocSiteLimit = len(allocSiteStats)
	}

	switch format {
	case "text", "markdown":
		if format == "markdown" {
			b.WriteString("```text\n")
		}
		b.WriteString(fmt.Sprintf("Allocation Profile Analysis (Top %d Functions by %s)\n", topN, valueType))
		b.WriteString(fmt.Sprintf("Total %s (%s): %s\n", valueType, valueUnit, FormatBytes(totalValue)))
		if totalObjects > 0 {
			b.WriteString(fmt.Sprintf("Total Objects: %d\n", totalObjects))
		}

		// Output by function
		b.WriteString("\n=== By Function ===\n")
		b.WriteString("--------------------------------------------------\n")
		b.WriteString(fmt.Sprintf("%-15s %-15s %s\n", valueType, "%", "Function Name"))
		b.WriteString("--------------------------------------------------\n")
		for i := 0; i < limit; i++ {
			stat := funcStats[i]
			percent := 0.0
			if totalValue != 0 {
				percent = (float64(stat.Flat) / float64(totalValue)) * 100
			}
			objStr := ""
			if count, ok := funcObjects[stat.Name]; ok && count > 0 {
				objStr = fmt.Sprintf(" (%d objects)", count)
			}
			b.WriteString(fmt.Sprintf("%-15s %-15.2f %s%s\n",
				FormatBytes(stat.Flat), percent, stat.Name, objStr))
		}

		// Output by allocation site
		b.WriteString("\n=== By Allocation Site ===\n")
		b.WriteString("--------------------------------------------------\n")
		b.WriteString(fmt.Sprintf("%-15s %-15s %s\n", valueType, "%", "Allocation Site"))
		b.WriteString("--------------------------------------------------\n")
		for i := 0; i < allocSiteLimit; i++ {
			stat := allocSiteStats[i]
			percent := 0.0
			if totalValue != 0 {
				percent = (float64(stat.Value) / float64(totalValue)) * 100
			}
			objStr := ""
			if stat.Count > 0 {
				objStr = fmt.Sprintf(" (%d objects)", stat.Count)
			}
			b.WriteString(fmt.Sprintf("%-15s %-15.2f %s%s\n",
				FormatBytes(stat.Value), percent, stat.Site, objStr))
		}

		if format == "markdown" {
			b.WriteString("```\n")
		}

	case "json":
		// Use JSON output structure from types.go

		result := struct {
			ProfileType         string             `json:"profileType"`
			ValueType           string             `json:"valueType"`
			ValueUnit           string             `json:"valueUnit"`
			TotalValue          int64              `json:"totalValue"`
			TotalValueFormatted string             `json:"totalValueFormatted"`
			TotalObjects        int64              `json:"totalObjects,omitempty"`
			TopN                int                `json:"topN"`
			Functions           []HeapFunctionStat `json:"functions"`
			AllocationSites     []AllocSiteStat    `json:"allocationSites"`
		}{
			ProfileType:         "allocs",
			ValueType:           valueType,
			ValueUnit:           valueUnit,
			TotalValue:          totalValue,
			TotalValueFormatted: FormatBytes(totalValue),
			TopN:                limit,
			Functions:           make([]HeapFunctionStat, 0, limit),
			AllocationSites:     make([]AllocSiteStat, 0, allocSiteLimit),
		}

		if totalObjects > 0 {
			result.TotalObjects = totalObjects
		}

		// Add function statistics
		for i := 0; i < limit; i++ {
			stat := funcStats[i]
			percent := 0.0
			if totalValue != 0 {
				percent = (float64(stat.Flat) / float64(totalValue)) * 100
			}

			funcStat := HeapFunctionStat{
				FunctionName:   stat.Name,
				Value:          stat.Flat,
				ValueFormatted: FormatBytes(stat.Flat),
				Percentage:     percent,
			}

			result.Functions = append(result.Functions, funcStat)
		}

		// Add allocation site statistics
		for i := 0; i < allocSiteLimit; i++ {
			stat := allocSiteStats[i]
			percent := 0.0
			if totalValue != 0 {
				percent = (float64(stat.Value) / float64(totalValue)) * 100
			}

			siteStat := AllocSiteStat{
				Site:           stat.Site,
				Value:          stat.Value,
				ValueFormatted: FormatBytes(stat.Value),
				Percentage:     percent,
			}

			if stat.Count > 0 {
				siteStat.ObjectCount = stat.Count
				// Calculate average allocation size
				avgSize := stat.Value / stat.Count
				siteStat.AvgSize = avgSize
				siteStat.AvgSizeFormatted = FormatBytes(avgSize)
			}

			result.AllocationSites = append(result.AllocationSites, siteStat)
		}

		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Printf("Error marshaling Allocs analysis to JSON: %v", err)
			errorResult := ErrorResult{Error: fmt.Sprintf("Failed to marshal result to JSON: %v", err)}
			errJsonBytes, _ := json.Marshal(errorResult)
			return string(errJsonBytes), nil
		}
		return string(jsonBytes), nil

	case "flamegraph-json":
		log.Printf("Generating flame graph JSON for Allocs profile (%s) using value index %d", valueType, valueIndex)
		// BuildFlameGraphTree will automatically detect this is a memory profile and find the objectsIndex
		// based on the valueType and valueUnit
		flameGraphRoot, err := BuildFlameGraphTree(p, valueIndex)
		if err != nil {
			log.Printf("Error building flame graph tree for allocs: %v", err)
			errorResult := ErrorResult{Error: fmt.Sprintf("Failed to build flame graph tree for allocs: %v", err)}
			errJsonBytes, _ := json.Marshal(errorResult)
			return string(errJsonBytes), nil
		}
		jsonBytes, err := json.Marshal(flameGraphRoot)
		if err != nil {
			log.Printf("Error marshaling allocs flame graph tree to JSON: %v", err)
			errorResult := ErrorResult{Error: fmt.Sprintf("Failed to marshal allocs flame graph tree to JSON: %v", err)}
			errJsonBytes, _ := json.Marshal(errorResult)
			return string(errJsonBytes), nil
		}
		return string(jsonBytes), nil

	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}

	return b.String(), nil
}
