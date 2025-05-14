package analyzer

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

// AnalyzeHeapProfile 分析 Heap profile (主要关注 inuse_space) 并返回格式化结果。
func AnalyzeHeapProfile(p *profile.Profile, topN int, format string) (string, error) {
	log.Printf("Analyzing Heap profile (Top %d, Format: %s)", topN, format)

	// --- 1. 查找 'inuse_space' 的样本值索引 ---
	// 常见的索引：0:alloc_objects, 1:alloc_space, 2:inuse_objects, 3:inuse_space
	valueIndex := -1
	objectsIndex := -1 // For tracking object counts

	for i, st := range p.SampleType {
		if st.Type == "inuse_space" && st.Unit == "bytes" {
			valueIndex = i
		}
		if st.Type == "inuse_objects" && st.Unit == "count" {
			objectsIndex = i
		}
	}
	// 回退方案：如果找不到 inuse_space，则尝试 alloc_space
	if valueIndex == -1 {
		for i, st := range p.SampleType {
			if st.Type == "alloc_space" && st.Unit == "bytes" {
				valueIndex = i
				log.Printf("Warning: 'inuse_space' not found, falling back to 'alloc_space'")
				break
			}
		}
	}

	// Fallback: If inuse_objects is not found, try alloc_objects
	if objectsIndex == -1 {
		for i, st := range p.SampleType {
			if st.Type == "alloc_objects" && st.Unit == "count" {
				objectsIndex = i
				log.Printf("Warning: 'inuse_objects' not found, falling back to 'alloc_objects'")
				break
			}
		}
	}

	// 回退方案：如果未找到特定类型，则尝试最后一个值 (通常是 inuse_space)
	if valueIndex == -1 && len(p.SampleType) > 0 {
		valueIndex = len(p.SampleType) - 1
		log.Printf("Warning: Could not find 'inuse_space' or 'alloc_space', defaulting to last sample type index %d: %s/%s",
			valueIndex, p.SampleType[valueIndex].Type, p.SampleType[valueIndex].Unit)
	}

	if valueIndex == -1 {
		return "", fmt.Errorf("无法从 profile 样本类型中确定值类型 (例如 inuse_space bytes)")
	}

	valueUnit := p.SampleType[valueIndex].Unit
	valueType := p.SampleType[valueIndex].Type
	log.Printf("使用索引 %d (%s/%s) 进行 Heap 分析", valueIndex, valueType, valueUnit)
	if objectsIndex >= 0 {
		log.Printf("使用索引 %d (%s/%s) 进行对象计数", objectsIndex, p.SampleType[objectsIndex].Type, p.SampleType[objectsIndex].Unit)
	}

	// --- 2. Aggregate memory usage values by function and allocation site ---
	// Create two maps: one for aggregating by function, one for aggregating by allocation site
	funcValue := make(map[string]int64)        // Aggregate by function name
	allocSiteValue := make(map[string]int64)   // Aggregate by allocation site (function+file+line)
	funcObjects := make(map[string]int64)      // Object count aggregated by function
	allocSiteObjects := make(map[string]int64) // Object count aggregated by allocation site

	// Maps for storing type information
	typeValue := make(map[string]int64)   // Memory usage aggregated by type
	typeObjects := make(map[string]int64) // Object count aggregated by type

	totalValue := int64(0)
	totalObjects := int64(0)

	for _, s := range p.Sample {
		if len(s.Location) > 0 && len(s.Value) > valueIndex {
			v := s.Value[valueIndex] // Memory usage (bytes)
			totalValue += v

			// If object count information is available, collect it too
			var objCount int64 = 0
			if objectsIndex >= 0 && len(s.Value) > objectsIndex {
				objCount = s.Value[objectsIndex]
				totalObjects += objCount
			}

			// Extract type information (if available)
			typeName := "unknown"
			if len(s.Label) > 0 {
				if typeLabels, ok := s.Label["type"]; ok && len(typeLabels) > 0 {
					typeName = typeLabels[0]
				} else if objLabels, ok := s.Label["object"]; ok && len(objLabels) > 0 {
					typeName = objLabels[0]
				}
			}

			// Aggregate by type
			typeValue[typeName] += v
			if objCount > 0 {
				typeObjects[typeName] += objCount
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

	// --- 3. Sort functions, allocation sites, and types by aggregated values ---
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

	// Sort by type
	type typeStat struct {
		Type  string
		Value int64
		Count int64
	}
	typeStats := make([]typeStat, 0, len(typeValue))
	for typeName, val := range typeValue {
		count := typeObjects[typeName]
		typeStats = append(typeStats, typeStat{Type: typeName, Value: val, Count: count})
	}
	sort.Slice(typeStats, func(i, j int) bool {
		return typeStats[i].Value > typeStats[j].Value // Sort in descending order
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

	typeLimit := limit
	if typeLimit > len(typeStats) {
		typeLimit = len(typeStats)
	}

	switch format {
	case "text", "markdown":
		if format == "markdown" {
			b.WriteString("```text\n")
		}
		b.WriteString(fmt.Sprintf("Heap Profile Analysis (Top %d Functions by %s)\n", topN, valueType))
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

		if len(typeStats) > 0 && typeStats[0].Type != "unknown" {
			b.WriteString("\n=== By Type ===\n")
			b.WriteString("--------------------------------------------------\n")
			b.WriteString(fmt.Sprintf("%-15s %-15s %-15s %s\n", valueType, "%", "Avg Size", "Type"))
			b.WriteString("--------------------------------------------------\n")
			for i := 0; i < typeLimit; i++ {
				stat := typeStats[i]
				percent := 0.0
				if totalValue != 0 {
					percent = (float64(stat.Value) / float64(totalValue)) * 100
				}

				avgSize := int64(0)
				if stat.Count > 0 {
					avgSize = stat.Value / stat.Count
				}

				b.WriteString(fmt.Sprintf("%-15s %-15.2f %-15s %s (%d objects)\n",
					FormatBytes(stat.Value), percent, FormatBytes(avgSize), stat.Type, stat.Count))
			}
		}
		if format == "markdown" {
			b.WriteString("```\n")
		}
	case "json":

		result := struct {
			ProfileType         string             `json:"profileType"`
			ValueType           string             `json:"valueType"`
			ValueUnit           string             `json:"valueUnit"`
			TotalValue          int64              `json:"totalValue"`
			TotalValueFormatted string             `json:"totalValueFormatted"`
			TotalObjects        int64              `json:"totalObjects,omitempty"`
			TopN                int                `json:"topN"`
			Functions           []HeapFunctionStat `json:"functions"`
			AllocationSites     []AllocSiteStat    `json:"allocationSites,omitempty"`
			Types               []TypeStat         `json:"types,omitempty"`
		}{
			ProfileType:         "heap",
			ValueType:           valueType,
			ValueUnit:           valueUnit,
			TotalValue:          totalValue,
			TotalValueFormatted: FormatBytes(totalValue), // 使用导出的 FormatBytes
			TopN:                limit,
			Functions:           make([]HeapFunctionStat, 0, limit),
		}

		if totalObjects > 0 {
			result.TotalObjects = totalObjects
		}

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

		if len(allocSiteStats) > 0 {
			result.AllocationSites = make([]AllocSiteStat, 0, allocSiteLimit)
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
					avgSize := stat.Value / stat.Count
					siteStat.AvgSize = avgSize
					siteStat.AvgSizeFormatted = FormatBytes(avgSize)
				}

				result.AllocationSites = append(result.AllocationSites, siteStat)
			}
		}

		if len(typeStats) > 0 && typeStats[0].Type != "unknown" {
			result.Types = make([]TypeStat, 0, typeLimit)
			for i := 0; i < typeLimit; i++ {
				stat := typeStats[i]
				percent := 0.0
				if totalValue != 0 {
					percent = (float64(stat.Value) / float64(totalValue)) * 100
				}

				typeStat := TypeStat{
					Type:           stat.Type,
					Value:          stat.Value,
					ValueFormatted: FormatBytes(stat.Value),
					Percentage:     percent,
				}

				if stat.Count > 0 {
					typeStat.ObjectCount = stat.Count
					avgSize := stat.Value / stat.Count
					typeStat.AvgSize = avgSize
					typeStat.AvgSizeFormatted = FormatBytes(avgSize)
				}

				result.Types = append(result.Types, typeStat)
			}
		}

		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Printf("Error marshaling Heap analysis to JSON: %v", err)
			errorResult := ErrorResult{Error: fmt.Sprintf("Failed to marshal result to JSON: %v", err)} // 使用 types.go 中的结构体
			errJsonBytes, _ := json.Marshal(errorResult)
			return string(errJsonBytes), nil
		}
		return string(jsonBytes), nil

	case "flamegraph-json":
		log.Printf("Generating flame graph JSON for Heap profile (%s) using value index %d", valueType, valueIndex)
		// BuildFlameGraphTree will automatically detect this is a memory profile and find the objectsIndex
		// based on the valueType and valueUnit
		flameGraphRoot, err := BuildFlameGraphTree(p, valueIndex)
		if err != nil {
			log.Printf("Error building flame graph tree for heap: %v", err)
			errorResult := ErrorResult{Error: fmt.Sprintf("Failed to build flame graph tree for heap: %v", err)}
			errJsonBytes, _ := json.Marshal(errorResult)
			return string(errJsonBytes), nil
		}
		jsonBytes, err := json.Marshal(flameGraphRoot) // 使用 Marshal 生成紧凑 JSON
		if err != nil {
			log.Printf("Error marshaling heap flame graph tree to JSON: %v", err)
			errorResult := ErrorResult{Error: fmt.Sprintf("Failed to marshal heap flame graph tree to JSON: %v", err)}
			errJsonBytes, _ := json.Marshal(errorResult)
			return string(errJsonBytes), nil
		}
		return string(jsonBytes), nil
	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}

	return b.String(), nil
}
