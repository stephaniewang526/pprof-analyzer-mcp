package main

import (
	"context"
	"fmt"
	"io" // 新增：用于 io.Copy 和 ReadCloser
	"log"
	"net/http" // 新增：用于 HTTP 请求
	"net/url"
	"os"
	"os/exec"
	"path/filepath" // 用于处理路径
	"strings"

	"github.com/google/pprof/profile"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/ZephyrDeng/pprof-analyzer-mcp/analyzer" // 导入新的 analyzer 包
)

// getProfileAsFile 获取 profile 文件。
// - 如果输入不包含 "://", 则视为本地文件路径（相对或绝对）。
// - 如果是 file:// URI，直接使用其路径。
// - 如果是 http:// 或 https:// URI，下载到临时文件并返回其路径。
// 返回最终的文件路径、一个用于清理临时文件的函数（如果创建了临时文件）以及错误。
func getProfileAsFile(uriStr string) (filePath string, cleanup func(), err error) {
	cleanup = func() {} // 默认清理函数为空操作

	// 检查输入是否包含协议头，如果没有，则假定为本地文件路径
	if !strings.Contains(uriStr, "://") {
		log.Printf("Input '%s' does not contain '://', treating as local file path.", uriStr)
		absPath, err := filepath.Abs(uriStr)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get absolute path for '%s': %w", uriStr, err)
		}
		log.Printf("Using absolute local path: %s", absPath)
		// 检查文件是否存在且可读 (可选但推荐)
		// _, statErr := os.Stat(absPath)
		// if statErr != nil {
		// 	 return "", nil, fmt.Errorf("local file '%s' (resolved to '%s') error: %w", uriStr, absPath, statErr)
		// }
		return absPath, cleanup, nil
	}

	// 如果包含 "://", 则按 URI 处理
	parsedURI, err := url.Parse(uriStr)
	if err != nil {
		return "", nil, fmt.Errorf("invalid profile URI '%s': %w", uriStr, err)
	}

	// (cleanup 默认值已在函数开头设置)

	switch parsedURI.Scheme {
	case "file":
		filePath = parsedURI.Path
		if filePath == "" {
			return "", nil, fmt.Errorf("invalid file path derived from URI '%s'", uriStr)
		}
		log.Printf("Using local profile file: %s", filePath)
		return filePath, cleanup, nil

	case "http", "https":
		log.Printf("Attempting to download profile from URL: %s", uriStr)
		resp, err := http.Get(uriStr)
		if err != nil {
			return "", nil, fmt.Errorf("failed to download profile from '%s': %w", uriStr, err)
		}
		defer resp.Body.Close() // 确保响应体被关闭

		if resp.StatusCode != http.StatusOK {
			return "", nil, fmt.Errorf("failed to download profile from '%s': received status code %d", uriStr, resp.StatusCode)
		}

		// 创建临时文件来存储下载的内容
		// 使用 "pprof-*.pb.gz" 作为模式，方便识别，后缀不影响内容
		tempFile, err := os.CreateTemp("", "pprof-*.pb.gz")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create temporary file for download: %w", err)
		}
		filePath = tempFile.Name()
		log.Printf("Downloading profile to temporary file: %s", filePath)

		// 定义清理函数，用于删除临时文件
		cleanup = func() {
			log.Printf("Cleaning up temporary file: %s", filePath)
			err := os.Remove(filePath)
			if err != nil && !os.IsNotExist(err) { // 忽略文件不存在的错误
				log.Printf("Warning: failed to remove temporary file '%s': %v", filePath, err)
			}
		}

		// 将下载的内容复制到临时文件
		_, err = io.Copy(tempFile, resp.Body)
		// 在复制后立即关闭文件句柄，以便后续可以重新打开或被其他进程使用
		closeErr := tempFile.Close()

		if err != nil {
			cleanup() // 如果复制失败，尝试清理临时文件
			return "", nil, fmt.Errorf("failed to write downloaded content to temporary file '%s': %w", filePath, err)
		}
		if closeErr != nil {
			// 虽然复制成功，但关闭文件失败也可能是问题
			log.Printf("Warning: failed to close temporary file handle for '%s': %v", filePath, closeErr)
			// 不立即清理，因为文件内容可能已写入，后续步骤可能仍需使用
		}

		log.Printf("Successfully downloaded profile to %s", filePath)
		return filePath, cleanup, nil

	default:
		return "", nil, fmt.Errorf("unsupported URI scheme '%s', only 'file://', 'http://', 'https://', or a plain local path are supported", parsedURI.Scheme)
	}
}

// handleAnalyzePprof 处理分析 pprof 文件的请求。
// 这是 MCP 工具 "analyze_pprof" 的处理器函数。
func handleAnalyzePprof(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	// --- 1. 获取并验证参数 ---
	profileURIStr, ok := args["profile_uri"].(string)
	if !ok || profileURIStr == "" {
		return nil, fmt.Errorf("missing or invalid required argument: profile_uri (string)")
	}
	profileType, ok := args["profile_type"].(string)
	if !ok || profileType == "" {
		return nil, fmt.Errorf("missing or invalid required argument: profile_type (string)")
	}
	outputFormat, ok := args["output_format"].(string)
	if !ok {
		outputFormat = "text" // 默认输出格式
	}
	topNFloat, ok := args["top_n"].(float64)
	if !ok {
		topNFloat = 5.0 // 默认 Top N 值
	}
	topN := int(topNFloat)
	if topN <= 0 {
		topN = 5 // 确保 topN 是正数
	}

	log.Printf("Handling analyze_pprof: URI=%s, Type=%s, TopN=%d, Format=%s", profileURIStr, profileType, topN, outputFormat)

	// --- 2. 获取 profile 文件（本地或下载）并解析 ---
	filePath, cleanup, err := getProfileAsFile(profileURIStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile file: %w", err)
	}
	defer cleanup() // 确保临时文件（如果创建了）被清理

	// 打开获取到的文件（可能是本地原文件或临时文件）
	file, err := os.Open(filePath)
	if err != nil {
		// 如果是临时文件打开失败，可能不需要再次 cleanup，但记录错误
		log.Printf("Error opening profile file '%s' (might be temporary): %v", filePath, err)
		return nil, fmt.Errorf("failed to open profile file '%s': %w", filePath, err)
	}
	defer file.Close() // 确保文件句柄被关闭

	// 解析 profile
	prof, err := profile.Parse(file)
	if err != nil {
		log.Printf("Error parsing profile file '%s': %v", filePath, err)
		return nil, fmt.Errorf("failed to parse profile file '%s': %w", filePath, err)
	}
	log.Printf("Successfully parsed profile file from path: %s", filePath)

	// --- 3. 根据 profile 类型分发到具体的分析函数 ---
	var analysisResult string
	var analysisErr error

	switch profileType {
	case "cpu":
		analysisResult, analysisErr = analyzer.AnalyzeCPUProfile(prof, topN, outputFormat)
	case "heap":
		analysisResult, analysisErr = analyzer.AnalyzeHeapProfile(prof, topN, outputFormat)
	case "goroutine":
		analysisResult, analysisErr = analyzer.AnalyzeGoroutineProfile(prof, topN, outputFormat)
	case "allocs":
		analysisResult, analysisErr = analyzer.AnalyzeAllocsProfile(prof, topN, outputFormat)
	case "mutex":
		analysisResult, analysisErr = analyzer.AnalyzeMutexProfile(prof, topN, outputFormat)
	case "block":
		analysisResult, analysisErr = analyzer.AnalyzeBlockProfile(prof, topN, outputFormat)
	default:
		analysisErr = fmt.Errorf("unsupported profile type: '%s'", profileType)
	}

	if analysisErr != nil {
		log.Printf("Analysis error for type '%s': %v", profileType, analysisErr)
		return nil, analysisErr // 返回具体的分析错误
	}

	// --- 4. 返回分析结果 ---
	log.Printf("Analysis successful for type '%s'. Result length: %d", profileType, len(analysisResult))
	// TODO: 未来可以根据 outputFormat 设置合适的 MIME 类型 (例如 text/markdown, application/json)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: analysisResult,
			},
		},
	}, nil
}

// handleGenerateFlamegraph 处理生成火焰图的请求。
func handleGenerateFlamegraph(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	// --- 1. 获取并验证参数 ---
	profileURIStr, ok := args["profile_uri"].(string)
	if !ok || profileURIStr == "" {
		return nil, fmt.Errorf("missing or invalid required argument: profile_uri (string)")
	}
	profileType, ok := args["profile_type"].(string)
	if !ok || profileType == "" {
		return nil, fmt.Errorf("missing or invalid required argument: profile_type (string)")
	}
	outputSvgPath, ok := args["output_svg_path"].(string)
	if !ok || outputSvgPath == "" {
		return nil, fmt.Errorf("missing or invalid required argument: output_svg_path (string)")
	}

	log.Printf("Handling generate_flamegraph: URI=%s, Type=%s, Output=%s", profileURIStr, profileType, outputSvgPath)

	// --- 2. 获取 profile 文件（本地或下载）---
	inputFilePath, cleanup, err := getProfileAsFile(profileURIStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile file for flamegraph: %w", err)
	}
	defer cleanup() // 确保临时文件（如果创建了）被清理

	// 确保输出路径是绝对路径或相对于工作区的有效路径
	// 如果不是绝对路径，则假定它是相对于当前工作目录的
	if !filepath.IsAbs(outputSvgPath) {
		cwd, err := os.Getwd() // 获取当前工作目录 (服务器运行的目录)
		if err != nil {
			log.Printf("无法获取当前工作目录: %v", err)
			// 尝试继续，但路径可能是错误的
		} else {
			outputSvgPath = filepath.Join(cwd, outputSvgPath)
			log.Printf("将相对输出路径转换为绝对路径: %s", outputSvgPath)
		}
	}

	// --- 3. 构建 go tool pprof 命令 ---
	// 基本参数
	cmdArgs := []string{"tool", "pprof"}

	// 根据 profile 类型添加特定标志
	switch profileType {
	case "heap":
		cmdArgs = append(cmdArgs, "-inuse_space") // 常用火焰图选项
	case "allocs":
		cmdArgs = append(cmdArgs, "-alloc_space")
	// cpu, goroutine, mutex, block 通常不需要额外标志即可生成 SVG
	case "cpu", "goroutine", "mutex", "block":
		// 不需要额外标志
	default:
		return nil, fmt.Errorf("unsupported profile type for flamegraph: '%s'", profileType)
	}

	// 添加 SVG 输出和输入文件参数
	cmdArgs = append(cmdArgs, "-svg", "-output", outputSvgPath, inputFilePath)

	log.Printf("Executing command: go %s", strings.Join(cmdArgs, " "))

	// --- 4. 检查 Graphviz (dot) 是否安装 ---
	_, err = exec.LookPath("dot")
	if err != nil {
		errMsg := "Graphviz (dot 命令) 未找到或不在 PATH 中。生成 SVG 火焰图需要 Graphviz。\n" +
			"请先安装 Graphviz。常见安装方式：\n" +
			"- macOS (Homebrew): brew install graphviz\n" +
			"- Debian/Ubuntu: sudo apt-get update && sudo apt-get install graphviz\n" +
			"- CentOS/Fedora: sudo yum install graphviz 或 sudo dnf install graphviz\n" +
			"- Windows (Chocolatey): choco install graphviz"
		log.Println(errMsg)
		return nil, fmt.Errorf(errMsg)
	}
	log.Println("Graphviz (dot) found.")

	// --- 5. 执行命令 ---
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmdOutput, err := cmd.CombinedOutput() // 获取 stdout 和 stderr

	if err != nil {
		log.Printf("Error executing 'go tool pprof': %v\nOutput:\n%s", err, string(cmdOutput))
		return nil, fmt.Errorf("failed to generate flamegraph: %w. Output: %s", err, string(cmdOutput))
	}

	log.Printf("Successfully generated flamegraph: %s", outputSvgPath)
	log.Printf("pprof output:\n%s", string(cmdOutput)) // 记录 pprof 的输出

	// --- 6. 读取 SVG 文件内容并返回 ---
	resultText := fmt.Sprintf("火焰图已成功生成并保存到: %s", outputSvgPath)
	textContent := mcp.TextContent{
		Type: "text",
		Text: resultText,
	}

	// 尝试读取生成的 SVG 文件内容
	svgBytes, readErr := os.ReadFile(outputSvgPath)
	if readErr != nil {
		log.Printf("成功生成 SVG 文件 '%s' 但读取失败: %v", outputSvgPath, readErr)
		// 即使读取失败，仍然返回成功生成的消息
		return &mcp.CallToolResult{
			Content: []mcp.Content{textContent},
		}, nil
	}

	svgContentStr := string(svgBytes)
	svgContent := mcp.TextContent{
		Type: "text", // 使用 text 类型，客户端可以根据内容判断是 SVG
		Text: svgContentStr,
	}

	// 返回包含文本消息和 SVG 内容的结果
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			textContent, // 成功消息和路径
			svgContent,  // SVG 文件内容
		},
	}, nil
}
