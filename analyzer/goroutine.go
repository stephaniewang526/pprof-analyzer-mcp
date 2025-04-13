package analyzer

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

// --- 内部辅助结构体 ---
// stackInfo 保存有关唯一 goroutine 堆栈跟踪的信息。
// 注意：保持未导出，因为它只在包内部使用。
type stackInfo struct {
	Stack []string // 格式化的堆栈跟踪行
	Count int64    // 具有此堆栈的 goroutine 数量
}

// AnalyzeGoroutineProfile 分析 Goroutine profile 并返回格式化结果。
func AnalyzeGoroutineProfile(p *profile.Profile, topN int, format string) (string, error) {
	log.Printf("Analyzing Goroutine profile (Top %d, Format: %s)", topN, format)

	// --- 1. 确定 Goroutine 计数的样本值索引 ---
	// Goroutine profile 通常只有一个样本类型："goroutines" / "count"
	valueIndex := 0 // 假设第一个样本类型是 goroutine 计数
	if len(p.SampleType) == 0 {
		return "", fmt.Errorf("goroutine profile 没有样本类型")
	}
	if p.SampleType[0].Type != "goroutines" {
		log.Printf("Warning: Expected 'goroutines' sample type, found: %v. Using index 0.", p.SampleType)
	}
	valueType := p.SampleType[valueIndex].Type
	valueUnit := p.SampleType[valueIndex].Unit
	log.Printf("使用索引 %d (%s/%s) 进行 Goroutine 分析", valueIndex, valueType, valueUnit)

	// --- 2. 按堆栈跟踪聚合 Goroutine ---
	stackCounts := make(map[string]*stackInfo) // Map 的键是堆栈的字符串表示形式
	totalGoroutines := int64(0)

	for _, s := range p.Sample {
		if len(s.Value) > valueIndex {
			count := s.Value[valueIndex] // 此堆栈的 Goroutine 数量
			totalGoroutines += count

			var stackKey strings.Builder
			var formattedStack []string
			// 同时构建字符串键和格式化的堆栈
			// 遍历样本堆栈跟踪中的 location
			// location 通常按从最新到最旧的帧排序
			for _, loc := range s.Location {
				// 每个 location 可能有多行 (由于内联)
				// 为了简化聚合键，我们只取第一行
				if len(loc.Line) > 0 {
					line := loc.Line[0] // 使用第一行信息
					if line.Function != nil {
						funcName := line.Function.Name
						fileName := line.Function.Filename
						lineNumber := line.Line
						// 格式化用于显示
						lineStr := fmt.Sprintf("%s\n\t%s:%d", funcName, fileName, lineNumber)
						formattedStack = append(formattedStack, lineStr)
						// 格式化用于唯一键 (不易受微小格式更改影响)
						keyLine := fmt.Sprintf("%s;%s;%d", funcName, fileName, lineNumber)
						stackKey.WriteString(keyLine)
						stackKey.WriteRune('|') // 键的唯一性分隔符
					}
				}
			}

			key := stackKey.String()
			if key == "" { // 跳过没有 location 信息的样本
				continue
			}

			if info, ok := stackCounts[key]; ok {
				info.Count += count
			} else {
				// 仅当键是新的时候才存储格式化的堆栈
				stackCounts[key] = &stackInfo{Stack: formattedStack, Count: count}
			}
		}
	}

	// --- 3. 按 Goroutine 数量对堆栈进行排序 ---
	stats := make([]*stackInfo, 0, len(stackCounts))
	for _, info := range stackCounts {
		stats = append(stats, info)
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count // 降序排列
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
		b.WriteString(fmt.Sprintf("Goroutine Profile Analysis (Top %d Stacks by Count)\n", topN))
		b.WriteString(fmt.Sprintf("Total Goroutines (%s/%s): %d\n", valueType, valueUnit, totalGoroutines))
		b.WriteString("--------------------------------------------------\n")
		for i := 0; i < limit; i++ {
			stat := stats[i]
			b.WriteString(fmt.Sprintf("\n%d goroutines with stack:\n", stat.Count))
			// 打印堆栈跟踪
			for _, line := range stat.Stack {
				b.WriteString(fmt.Sprintf("  %s\n", line)) // 缩进堆栈行
			}
			b.WriteString("--------------------------------------------------\n")
		}
		if format == "markdown" {
			b.WriteString("```\n")
		}
	case "json":
		result := GoroutineAnalysisResult{ // 使用 types.go 中的结构体
			ProfileType:     "goroutine",
			TotalGoroutines: totalGoroutines,
			TopN:            limit,
			Stacks:          make([]GoroutineStackInfo, 0, limit), // 使用 types.go 中的结构体
		}

		for i := 0; i < limit; i++ {
			stat := stats[i]
			// 注意：这里直接复制了 stat.Stack。如果 StackInfo.Stack 在其他地方被修改，这里也会受影响。
			// 但在这个场景下，stat 是局部变量，应该没问题。
			result.Stacks = append(result.Stacks, GoroutineStackInfo{ // 使用 types.go 中的结构体
				Count:      stat.Count,
				StackTrace: stat.Stack, // 直接使用已格式化的堆栈
			})
		}

		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Printf("Error marshaling Goroutine analysis to JSON: %v", err)
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
