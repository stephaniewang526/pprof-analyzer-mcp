package analyzer

// --- JSON 输出结构体定义 ---

// ErrorResult 用于在 JSON 格式中返回错误信息
type ErrorResult struct {
	Error string `json:"error"`
	TopN  int    `json:"topN,omitempty"` // omitempty 如果为 0 则不输出
}

// CPUFunctionStat 代表 CPU 分析中的单个函数统计信息 (JSON)
type CPUFunctionStat struct {
	FunctionName       string  `json:"functionName"`
	FlatValue          int64   `json:"flatValue"`          // 原始值
	FlatValueFormatted string  `json:"flatValueFormatted"` // 格式化后的值 (e.g., "1.23s")
	Percentage         float64 `json:"percentage"`         // 占总量的百分比
}

// CPUAnalysisResult 代表 CPU 分析的整体结果 (JSON)
type CPUAnalysisResult struct {
	ProfileType         string            `json:"profileType"`
	ValueType           string            `json:"valueType"`                    // e.g., "cpu", "samples"
	ValueUnit           string            `json:"valueUnit"`                    // e.g., "nanoseconds", "count"
	TotalValue          int64             `json:"totalValue"`                   // 样本总值
	TotalValueFormatted string            `json:"totalValueFormatted"`          // 格式化后的总值
	TotalDurationNanos  int64             `json:"totalDurationNanos,omitempty"` // 可选的总持续时间 (纳秒)
	TopN                int               `json:"topN"`                         // 返回的 Top N 数量
	Functions           []CPUFunctionStat `json:"functions"`                    // Top N 函数列表
}

// HeapFunctionStat 代表 Heap 分析中的单个函数统计信息 (JSON)
type HeapFunctionStat struct {
	FunctionName   string  `json:"functionName"`
	Value          int64   `json:"value"`          // 原始值 (bytes)
	ValueFormatted string  `json:"valueFormatted"` // 格式化后的值 (e.g., "1.23 MiB")
	Percentage     float64 `json:"percentage"`     // 占总量的百分比
}

// HeapAnalysisResult 代表 Heap 分析的整体结果 (JSON)
type HeapAnalysisResult struct {
	ProfileType         string             `json:"profileType"`
	ValueType           string             `json:"valueType"`           // e.g., "inuse_space", "alloc_space"
	ValueUnit           string             `json:"valueUnit"`           // e.g., "bytes"
	TotalValue          int64              `json:"totalValue"`          // 总值 (bytes)
	TotalValueFormatted string             `json:"totalValueFormatted"` // 格式化后的总值
	TopN                int                `json:"topN"`                // 返回的 Top N 数量
	Functions           []HeapFunctionStat `json:"functions"`           // Top N 函数列表
}

// GoroutineStackInfo 代表 Goroutine 分析中的单个堆栈信息 (JSON)
type GoroutineStackInfo struct {
	Count      int64    `json:"count"`      // 具有此堆栈的 Goroutine 数量
	StackTrace []string `json:"stackTrace"` // 格式化的堆栈跟踪行
}

// GoroutineAnalysisResult 代表 Goroutine 分析的整体结果 (JSON)
type GoroutineAnalysisResult struct {
	ProfileType     string               `json:"profileType"`
	TotalGoroutines int64                `json:"totalGoroutines"`
	TopN            int                  `json:"topN"`   // 返回的 Top N 数量
	Stacks          []GoroutineStackInfo `json:"stacks"` // Top N 堆栈列表
}

// FlameGraphNode 代表火焰图中的一个节点 (JSON)
// 用于生成层级化的 JSON 数据，适合 d3-flame-graph 等库使用
type FlameGraphNode struct {
	Name     string            `json:"name"`               // 函数名或其他标识符
	Value    int64             `json:"value"`              // 该节点及其子节点的总值
	Children []*FlameGraphNode `json:"children,omitempty"` // 子节点列表
	// 可以添加其他元数据字段，例如：
	// FlatValue int64 `json:"flatValue,omitempty"` // 仅该节点自身的值
	// FilePath string `json:"filePath,omitempty"` // 源码文件路径
	// LineNum int `json:"lineNum,omitempty"` // 源码行号
}

// --- 内部辅助结构体 ---

// functionStat 保存函数的聚合统计信息。
// 注意：保持未导出，因为它只在包内部使用。
type functionStat struct {
	Name string
	Flat int64 // 函数自身的消耗值 (例如 CPU 时间、内存分配)
	Cum  int64 // 函数及其调用链的总消耗值 (当前未使用)
}

// stackInfo 结构体已移至 goroutine.go
