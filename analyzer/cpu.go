package analyzer

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/pprof/profile"
)

// AnalyzeCPUProfile 分析 CPU profile 文件并返回格式化结果。
func AnalyzeCPUProfile(p *profile.Profile, topN int, format string) (string, error) {
	log.Printf("Analyzing CPU profile (Top %d, Format: %s)", topN, format)

	// --- 1. 确定用于分析的值的索引 (通常是 CPU 时间) ---
	valueIndex := -1 // CPU 时间样本值的索引 (通常是 1, 'samples/count' 是 0)
	for i, st := range p.SampleType {
		// 查找 'cpu' 和 'nanoseconds' 或类似的样本类型
		if (st.Type == "cpu" || st.Type == "samples") && (st.Unit == "nanoseconds" || st.Unit == "count") {
			// 优先选择 'cpu'/'nanoseconds'，否则选择 'samples'/'count'
			if valueIndex == -1 || st.Type == "cpu" {
				valueIndex = i
			}
		}
	}
	if valueIndex == -1 {
		if len(p.SampleType) > 1 {
			valueIndex = 1 // 如果未找到特定类型，则默认为第二个值类型
			log.Printf("Warning: Could not definitively identify CPU time value type, defaulting to index 1: %s/%s", p.SampleType[valueIndex].Type, p.SampleType[valueIndex].Unit)
		} else if len(p.SampleType) == 1 {
			valueIndex = 0 // 使用唯一可用的类型
			log.Printf("Warning: Only one sample type found, using index 0: %s/%s", p.SampleType[valueIndex].Type, p.SampleType[valueIndex].Unit)
		} else {
			return "", fmt.Errorf("无法从 profile 样本类型中确定值类型 (例如 cpu nanoseconds)")
		}
	}
	valueUnit := p.SampleType[valueIndex].Unit
	log.Printf("使用索引 %d (%s/%s) 进行 CPU 分析", valueIndex, p.SampleType[valueIndex].Type, valueUnit)

	// --- 2. 按函数聚合 Flat 时间 ---
	flatTime := make(map[string]int64)
	totalValue := int64(0)

	for _, s := range p.Sample {
		if len(s.Location) > 0 && len(s.Value) > valueIndex {
			v := s.Value[valueIndex]
			totalValue += v
			// Flat 时间归因于堆栈中最顶层的函数
			loc := s.Location[0]
			for _, line := range loc.Line {
				if line.Function != nil {
					flatTime[line.Function.Name] += v
					// 每个样本的顶层框架只计算一次函数
					break
				}
			}
		}
	}

	if totalValue == 0 {
		log.Printf("Warning: Total value for the selected sample type (%s/%s) is zero.", p.SampleType[valueIndex].Type, valueUnit)
		// 继续处理，可能只是一个空的 profile 或选择了错误的样本类型
	}

	// --- 3. 按 Flat 时间对函数进行排序 ---
	stats := make([]functionStat, 0, len(flatTime))
	for name, flat := range flatTime {
		stats = append(stats, functionStat{Name: name, Flat: flat})
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

	// 获取总持续时间 (用于计算百分比)
	totalDuration := time.Duration(p.DurationNanos) * time.Nanosecond
	if totalDuration == 0 && totalValue > 0 && valueUnit == "nanoseconds" {
		// 如果 DurationNanos 为零，则从样本总值估算持续时间
		totalDuration = time.Duration(totalValue) * time.Nanosecond
		log.Printf("Profile DurationNanos is 0, estimated total duration from samples: %s", totalDuration)
	}

	switch format {
	case "text", "markdown": // 目前两者使用相似格式
		if format == "markdown" {
			b.WriteString("```text\n") // 使用文本块以获得更好的对齐效果
		}
		b.WriteString(fmt.Sprintf("CPU Profile Analysis (Top %d Functions by Flat Time)\n", topN))
		b.WriteString(fmt.Sprintf("Total Samples/Time (%s): %s\n", valueUnit, FormatSampleValue(totalValue, valueUnit))) // 使用导出的 FormatSampleValue
		if totalDuration > 0 {
			b.WriteString(fmt.Sprintf("Total Duration: %s\n", totalDuration))
		}
		b.WriteString("--------------------------------------------------\n")
		b.WriteString(fmt.Sprintf("%-15s %-15s %s\n", "Flat Time", "%", "Function Name"))
		b.WriteString("--------------------------------------------------\n")
		for i := 0; i < limit; i++ {
			stat := stats[i]
			percent := 0.0
			// 如果 totalValue 不为零，则计算百分比
			if totalValue != 0 {
				percent = (float64(stat.Flat) / float64(totalValue)) * 100
			}
			b.WriteString(fmt.Sprintf("%-15s %-15.2f %s\n", FormatSampleValue(stat.Flat, valueUnit), percent, stat.Name)) // 使用导出的 FormatSampleValue
		}
		if format == "markdown" {
			b.WriteString("```\n")
		}
	case "json":
		result := CPUAnalysisResult{ // 使用 types.go 中的结构体
			ProfileType:         "cpu",
			ValueType:           p.SampleType[valueIndex].Type,
			ValueUnit:           valueUnit,
			TotalValue:          totalValue,
			TotalValueFormatted: FormatSampleValue(totalValue, valueUnit), // 使用导出的 FormatSampleValue
			TopN:                limit,
			Functions:           make([]CPUFunctionStat, 0, limit), // 使用 types.go 中的结构体
		}
		if totalDuration > 0 {
			result.TotalDurationNanos = totalDuration.Nanoseconds()
		}

		for i := 0; i < limit; i++ {
			stat := stats[i]
			percent := 0.0
			if totalValue != 0 {
				percent = (float64(stat.Flat) / float64(totalValue)) * 100
			}
			result.Functions = append(result.Functions, CPUFunctionStat{ // 使用 types.go 中的结构体
				FunctionName:       stat.Name,
				FlatValue:          stat.Flat,
				FlatValueFormatted: FormatSampleValue(stat.Flat, valueUnit), // 使用导出的 FormatSampleValue
				Percentage:         percent,
			})
		}

		jsonBytes, err := json.MarshalIndent(result, "", "  ") // 使用缩进美化输出
		if err != nil {
			log.Printf("Error marshaling CPU analysis to JSON: %v", err)
			// 返回一个简单的 JSON 错误
			errorResult := ErrorResult{Error: fmt.Sprintf("Failed to marshal result to JSON: %v", err)} // 使用 types.go 中的结构体
			errJsonBytes, _ := json.Marshal(errorResult)
			return string(errJsonBytes), nil // 返回错误信息，但不标记为分析错误
		}
		return string(jsonBytes), nil

	case "flamegraph-json":
		log.Printf("Generating flame graph JSON for CPU profile using value index %d", valueIndex)
		flameGraphRoot, err := BuildFlameGraphTree(p, valueIndex) // 调用新函数
		if err != nil {
			log.Printf("Error building flame graph tree: %v", err)
			errorResult := ErrorResult{Error: fmt.Sprintf("Failed to build flame graph tree: %v", err)}
			errJsonBytes, _ := json.Marshal(errorResult)
			return string(errJsonBytes), nil // 返回错误信息，但不标记为分析错误
		}
		jsonBytes, err := json.Marshal(flameGraphRoot) // 使用 Marshal 生成紧凑 JSON
		if err != nil {
			log.Printf("Error marshaling flame graph tree to JSON: %v", err)
			errorResult := ErrorResult{Error: fmt.Sprintf("Failed to marshal flame graph tree to JSON: %v", err)}
			errJsonBytes, _ := json.Marshal(errorResult)
			return string(errJsonBytes), nil // 返回错误信息，但不标记为分析错误
		}
		return string(jsonBytes), nil

	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}

	return b.String(), nil
}
