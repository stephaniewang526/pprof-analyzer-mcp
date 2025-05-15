package main

import (
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// handleAnalyzePprof 函数已移至 handler.go

func main() {
	// 1. 初始化 MCP 服务器
	mcpServer := server.NewMCPServer(
		"PprofAnalyzer",       // 服务器名称
		"0.1.0",               // 服务器版本
		server.WithLogging(),  // 启用日志记录
		server.WithRecovery(), // 启用 panic 恢复
	)

	// 2. 定义 analyze_pprof 工具及其参数
	analyzeTool := mcp.NewTool("analyze_pprof",
		mcp.WithDescription("分析指定的 Go pprof 文件，并返回序列化的分析结果 (例如 Top N 列表或火焰图 JSON)。"), // 更新描述
		// mcp.WithAnnotation("title", "Analyze Go pprof Profile"), // TODO: 检查如何在 mcp-go 中设置注解
		// mcp.WithAnnotation("readOnlyHint", true),             // TODO: 检查如何在 mcp-go 中设置注解

		mcp.WithString("profile_uri", // 参数名称
			mcp.Description("要分析的 pprof 文件的 URI (支持 'file://', 'http://', 'https://' 协议)。例如 'file:///path/to/profile.pb.gz' 或 'https://example.com/profile.pb.gz'。"),
			mcp.Required(),
		),
		mcp.WithString("profile_type", // 参数名称
			mcp.Description("要分析的 pprof profile 的类型。"),
			mcp.Required(),
			mcp.Enum("cpu", "heap", "goroutine", "allocs", "mutex", "block"),
		),
		mcp.WithNumber("top_n", // 参数名称
			mcp.Description("返回结果的数量上限 (例如 Top 5, Top 10)。"),
			mcp.DefaultNumber(5.0), // MCP Go SDK 使用 float64 表示数字，默认为 5
		),
		mcp.WithString("output_format", // 参数名称
			mcp.Description("分析结果的输出格式。'flamegraph-json' 仅适用于 'cpu' 和 'heap' 类型，用于生成层级化的 JSON 数据。"),
			mcp.DefaultString("flamegraph-json"),                    // 将默认值改为 flamegraph-json
			mcp.Enum("text", "markdown", "json", "flamegraph-json"), // 添加新格式
		),
	)

	// 3. 定义 generate_flamegraph 工具
	flamegraphTool := mcp.NewTool("generate_flamegraph",
		mcp.WithDescription("使用 'go tool pprof' 为指定的 pprof 文件生成火焰图 (SVG 格式)，将其保存到指定路径，并返回路径和 SVG 内容。"), // 更新描述
		mcp.WithString("profile_uri",
			mcp.Description("要生成火焰图的 pprof 文件的 URI (支持 'file://', 'http://', 'https://' 协议)。"),
			mcp.Required(),
		),
		mcp.WithString("profile_type",
			mcp.Description("要生成火焰图的 pprof profile 的类型。"),
			mcp.Required(),
			mcp.Enum("cpu", "heap", "allocs", "goroutine", "mutex", "block"), // 支持的类型
		),
		mcp.WithString("output_svg_path",
			mcp.Description("生成的 SVG 火焰图文件的保存路径 (必须是绝对路径或相对于工作区的路径)。"),
			mcp.Required(),
		),
	)

	// 4. detect_memory_leaks
	memoryLeakTool := mcp.NewTool("detect_memory_leaks",
		mcp.WithDescription("Compare two heap profile files to identify potential memory leaks."),
		mcp.WithString("old_profile_uri",
			mcp.Description("The URI of the older heap profile, supporting 'file://', 'http://', 'https://' protocols."),
			mcp.Required(),
		),
		mcp.WithString("new_profile_uri",
			mcp.Description("The URI of the newer heap profile, supporting 'file://', 'http://', 'https://' protocols."),
			mcp.Required(),
		),
		mcp.WithNumber("threshold",
			mcp.Description("The growth threshold for detecting memory leaks (0.1 represents a 10% increase)."),
			mcp.DefaultNumber(0.1),
		),
		mcp.WithNumber("limit",
			mcp.Description("The maximum number of potential memory leak types to return."),
			mcp.DefaultNumber(10.0),
		),
	)

	// 5. 定义 open_interactive_pprof 工具 (仅限 macOS)
	openInteractiveTool := mcp.NewTool("open_interactive_pprof",
		mcp.WithDescription("【仅限 macOS】尝试在后台启动 'go tool pprof' 交互式 Web UI。成功启动后会返回进程 PID，用于后续手动断开连接。"),
		mcp.WithString("profile_uri",
			mcp.Description("要分析的 pprof 文件的 URI (支持 'file://', 'http://', 'https://' 或本地路径)。"),
			mcp.Required(),
		),
		mcp.WithString("http_address",
			mcp.Description("指定 pprof Web UI 的监听地址和端口 (例如 ':8081')。如果省略，默认为 ':8081'。"),
			// mcp.Optional(), // 不提供 Required() 即为可选
		),
	)

	// 6. 定义 disconnect_pprof_session 工具
	disconnectTool := mcp.NewTool("disconnect_pprof_session",
		mcp.WithDescription("尝试终止由 'open_interactive_pprof' 启动的指定后台 pprof 进程。"),
		mcp.WithNumber("pid", // 使用 Number 类型，因为 JSON 通常将数字表示为 float64
			mcp.Description("要终止的后台 pprof 进程的 PID (由 'open_interactive_pprof' 返回)。"),
			mcp.Required(),
		),
		mcp.WithString("http_address", // 可选参数
			mcp.Description("指定 pprof Web UI 的监听地址和端口 (例如 ':8081')。如果省略，pprof 会自动选择。"),
			// mcp.Optional(), // mcp-go SDK 可能没有显式的 Optional()，不提供 Required() 即为可选
		),
	)

	// 7. 将所有工具及其处理器函数添加到服务器
	mcpServer.AddTool(analyzeTool, handleAnalyzePprof)
	mcpServer.AddTool(flamegraphTool, handleGenerateFlamegraph)
	mcpServer.AddTool(memoryLeakTool, handleDetectMemoryLeaks)
	mcpServer.AddTool(openInteractiveTool, handleOpenInteractivePprof)
	mcpServer.AddTool(disconnectTool, handleDisconnectPprofSession) // 注册断开连接工具

	// 8. 设置信号处理程序以进行清理
	setupSignalHandler() // 在服务器启动前设置

	// 9. Start the server using stdio transport
	log.Println("Starting PprofAnalyzer MCP server via stdio...")
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
