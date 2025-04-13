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
		mcp.WithDescription("分析指定的 Go pprof 文件 (支持 cpu, heap, goroutine, allocs, mutex, block 类型)。"),
		// mcp.WithAnnotation("title", "Analyze Go pprof Profile"), // TODO: 检查如何在 mcp-go 中设置注解
		// mcp.WithAnnotation("readOnlyHint", true),             // TODO: 检查如何在 mcp-go 中设置注解

		mcp.WithString("profile_uri", // 参数名称
			mcp.Description("要分析的 pprof 文件的 URI (必须是 'file://' 协议，例如 'file:///path/to/profile.pb.gz')。"),
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
			mcp.Description("分析结果的输出格式。"),
			mcp.DefaultString("text"),
			mcp.Enum("text", "markdown", "json"),
		),
	)

	// 3. 定义 generate_flamegraph 工具
	flamegraphTool := mcp.NewTool("generate_flamegraph",
		mcp.WithDescription("使用 'go tool pprof' 生成指定 pprof 文件的火焰图 (SVG 格式)。"),
		mcp.WithString("profile_uri",
			mcp.Description("要生成火焰图的 pprof 文件的 URI (必须是 'file://' 协议)。"),
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

	// 4. 将工具及其处理器函数添加到服务器
	mcpServer.AddTool(analyzeTool, handleAnalyzePprof)
	mcpServer.AddTool(flamegraphTool, handleGenerateFlamegraph) // 添加新工具和处理器

	// 5. Start the server using stdio transport
	log.Println("Starting PprofAnalyzer MCP server via stdio...")
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
