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
	for i, st := range p.SampleType {
		if st.Type == "inuse_space" {
			valueIndex = i
			break
		}
	}
	// 回退方案：如果找不到 inuse_space，则尝试 alloc_space
	if valueIndex == -1 {
		for i, st := range p.SampleType {
			if st.Type == "alloc_space" {
				valueIndex = i
				log.Printf("Warning: 'inuse_space' not found, falling back to 'alloc_space'")
				break
			}
		}
	}
	// 回退方案：如果未找到特定类型，则尝试最后一个值 (通常是 inuse_space)
	if valueIndex == -1 && len(p.SampleType) > 0 {
		valueIndex = len(p.SampleType) - 1
		log.Printf("Warning: Could not find 'inuse_space' or 'alloc_space', defaulting to last sample type index %d: %s/%s", valueIndex, p.SampleType[valueIndex].Type, p.SampleType[valueIndex].Unit)
	}

	if valueIndex == -1 {
		return "", fmt.Errorf("无法从 profile 样本类型中确定值类型 (例如 inuse_space bytes)")
	}
	valueUnit := p.SampleType[valueIndex].Unit
	valueType := p.SampleType[valueIndex].Type
	log.Printf("使用索引 %d (%s/%s) 进行 Heap 分析", valueIndex, valueType, valueUnit)

	// --- 2. 按函数聚合内存使用值 ---
	// 对于 Heap profile，我们通常关注基于样本调用堆栈归因于函数的累积值。
	// 为简单起见，我们将对出现在堆栈中任何位置的每个函数的值求和，
	// 尽管要获得完美的准确性可能需要更复杂的归因方法。
	// 这里我们基于直接负责分配的函数 (像 CPU flat 那样的顶层框架) 进行聚合。
	// 按直接负责分配的函数 (顶层框架) 进行聚合。
	funcValue := make(map[string]int64)
	totalValue := int64(0)

	for _, s := range p.Sample {
		if len(s.Location) > 0 && len(s.Value) > valueIndex {
			v := s.Value[valueIndex]
			totalValue += v
			// 将内存归因于发生分配的堆栈中最顶层的函数
			loc := s.Location[0]
			for _, line := range loc.Line {
				if line.Function != nil {
					funcValue[line.Function.Name] += v
					break // 只归因于顶层框架中找到的第一个函数
				}
			}
		}
	}

	if totalValue == 0 {
		log.Printf("Warning: Total value for the selected sample type (%s/%s) is zero.", valueType, valueUnit)
	}

	// --- 3. 按聚合值对函数进行排序 ---
	stats := make([]functionStat, 0, len(funcValue)) // 使用 types.go 中的结构体
	for name, val := range funcValue {
		stats = append(stats, functionStat{Name: name, Flat: val}) // 使用 Flat 字段存储聚合值
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Flat > stats[j].Flat // 降序排列
	})

	// --- 4. 格式化输出 ---
	var b strings.Builder
	limit := topN
	if limit > len(stats) {
		limit = len(stats)
	}

	switch format {
	case "text", "markdown":
		if format == "markdown" {
			b.WriteString("```text\n")
		}
		b.WriteString(fmt.Sprintf("Heap Profile Analysis (Top %d Functions by %s)\n", topN, valueType))
		b.WriteString(fmt.Sprintf("Total %s (%s): %s\n", valueType, valueUnit, FormatBytes(totalValue))) // 使用导出的 FormatBytes
		b.WriteString("--------------------------------------------------\n")
		b.WriteString(fmt.Sprintf("%-15s %-15s %s\n", valueType, "%", "Function Name"))
		b.WriteString("--------------------------------------------------\n")
		for i := 0; i < limit; i++ {
			stat := stats[i]
			percent := 0.0
			if totalValue != 0 {
				percent = (float64(stat.Flat) / float64(totalValue)) * 100
			}
			// 对 Heap 值使用 FormatBytes
			b.WriteString(fmt.Sprintf("%-15s %-15.2f %s\n", FormatBytes(stat.Flat), percent, stat.Name)) // 使用导出的 FormatBytes
		}
		if format == "markdown" {
			b.WriteString("```\n")
		}
	case "json":
		result := HeapAnalysisResult{ // 使用 types.go 中的结构体
			ProfileType:         "heap",
			ValueType:           valueType,
			ValueUnit:           valueUnit,
			TotalValue:          totalValue,
			TotalValueFormatted: FormatBytes(totalValue), // 使用导出的 FormatBytes
			TopN:                limit,
			Functions:           make([]HeapFunctionStat, 0, limit), // 使用 types.go 中的结构体
		}

		for i := 0; i < limit; i++ {
			stat := stats[i]
			percent := 0.0
			if totalValue != 0 {
				percent = (float64(stat.Flat) / float64(totalValue)) * 100
			}
			result.Functions = append(result.Functions, HeapFunctionStat{ // 使用 types.go 中的结构体
				FunctionName:   stat.Name,
				Value:          stat.Flat,
				ValueFormatted: FormatBytes(stat.Flat), // 使用导出的 FormatBytes
				Percentage:     percent,
			})
		}

		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Printf("Error marshaling Heap analysis to JSON: %v", err)
			errorResult := ErrorResult{Error: fmt.Sprintf("Failed to marshal result to JSON: %v", err)} // 使用 types.go 中的结构体
			errJsonBytes, _ := json.Marshal(errorResult)
			return string(errJsonBytes), nil
		}
		return string(jsonBytes), nil
	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}

	return b.String(), nil
}
