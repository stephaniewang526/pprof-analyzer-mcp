package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
)

// 全局变量，用于跟踪由本服务器启动的 pprof 进程
var (
	runningPprofs = make(map[int]*os.Process) // 存储 PID 到 Process 指针的映射
	pprofMutex    sync.Mutex                  // 用于保护 runningPprofs 的互斥锁
)

// handleOpenInteractivePprof 处理在 macOS 上尝试打开 pprof 交互式 UI 的请求。
func handleOpenInteractivePprof(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("此功能仅在 macOS 上可用 (当前系统: %s)", runtime.GOOS)
	}

	args := request.Params.Arguments

	profileURIStr, ok := args["profile_uri"].(string)
	if !ok || profileURIStr == "" {
		return nil, fmt.Errorf("missing or invalid required argument: profile_uri (string)")
	}
	httpAddress, ok := args["http_address"].(string)
	if !ok || httpAddress == "" {
		httpAddress = ":8081" // 默认端口
		log.Printf("No http_address provided, using default: %s", httpAddress)
	}

	log.Printf("Handling open_interactive_pprof: URI=%s, Address=%s", profileURIStr, httpAddress)

	inputFilePath, cleanup, err := getProfileAsFile(profileURIStr) // 调用 profile_utils.go 中的函数
	if err != nil {
		return nil, fmt.Errorf("failed to get profile file: %w", err)
	}
	// 注意：不能在这里 defer cleanup()，因为 pprof 进程需要持续访问文件

	cmdArgs := []string{"tool", "pprof"}
	cmdArgs = append(cmdArgs, fmt.Sprintf("-http=%s", httpAddress)) // 总是添加 -http 参数
	cmdArgs = append(cmdArgs, inputFilePath)

	log.Printf("Preparing to execute command in background: go %s", strings.Join(cmdArgs, " "))

	_, err = exec.LookPath("go")
	if err != nil {
		log.Println("Error: 'go' command not found in PATH.")
		if parsedURI, parseErr := url.Parse(profileURIStr); parseErr == nil && (parsedURI.Scheme == "http" || parsedURI.Scheme == "https") {
			cleanup() // 尝试清理临时文件
		}
		return nil, fmt.Errorf("'go' command not found in PATH, cannot start pprof")
	}

	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	err = cmd.Start()

	if err != nil {
		log.Printf("Error starting 'go tool pprof' in background: %v", err)
		if parsedURI, parseErr := url.Parse(profileURIStr); parseErr == nil && (parsedURI.Scheme == "http" || parsedURI.Scheme == "https") {
			cleanup() // 尝试清理临时文件
		}
		return nil, fmt.Errorf("failed to start 'go tool pprof': %w", err)
	}

	pid := cmd.Process.Pid
	pprofMutex.Lock()
	runningPprofs[pid] = cmd.Process
	pprofMutex.Unlock()

	log.Printf("Successfully started 'go tool pprof' in background with PID: %d", pid)

	resultText := fmt.Sprintf("已成功在后台启动 'go tool pprof' (PID: %d) 来分析 '%s'", pid, inputFilePath)
	resultText += fmt.Sprintf("，监听地址约为 %s。", httpAddress)
	resultText += "\n你可以使用 'disconnect_pprof_session' 工具并提供 PID 来尝试终止此进程。"
	resultText += "\n注意：如果是远程 URL，下载的临时 pprof 文件在进程结束前不会被自动删除。"

	log.Println(resultText)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: resultText,
			},
		},
	}, nil
}

// handleDisconnectPprofSession 处理断开指定 pprof 会话的请求。
func handleDisconnectPprofSession(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	pidFloat, ok := args["pid"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid required argument: pid (number)")
	}
	pid := int(pidFloat)
	if pid <= 0 {
		return nil, fmt.Errorf("invalid PID: %d", pid)
	}

	log.Printf("Handling disconnect_pprof_session for PID: %d", pid)

	pprofMutex.Lock()
	process, exists := runningPprofs[pid]
	if !exists {
		pprofMutex.Unlock()
		log.Printf("PID %d not found in running pprof sessions.", pid)
		return nil, fmt.Errorf("未找到 PID 为 %d 的正在运行的 pprof 会话", pid)
	}
	delete(runningPprofs, pid) // 从 map 中移除记录
	pprofMutex.Unlock()

	log.Printf("Attempting to terminate process with PID: %d", pid)
	err := process.Signal(os.Interrupt) // 尝试 Interrupt
	if err != nil {
		log.Printf("Failed to send Interrupt signal to PID %d: %v. Trying Kill signal.", pid, err)
		err = process.Signal(os.Kill) // 尝试 Kill
		if err != nil {
			log.Printf("Failed to send Kill signal to PID %d: %v", pid, err)
			// 即使信号发送失败，也认为尝试过断开，但返回错误
			return nil, fmt.Errorf("尝试终止 PID %d 失败：%w", pid, err)
		}
	}

	// 尝试释放进程资源（虽然信号可能异步处理，但这有助于清理）
	_, err = process.Wait()
	if err != nil && !strings.Contains(err.Error(), "wait: no child processes") && !strings.Contains(err.Error(), "signal:") {
		// 忽略 "no child processes" 和信号相关的错误，因为进程可能已经被信号终止
		log.Printf("Warning: Error waiting for process PID %d after signaling: %v", pid, err)
	}

	resultText := fmt.Sprintf("已成功向 PID %d 发送终止信号。", pid)
	log.Println(resultText)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: resultText,
			},
		},
	}, nil
}

// setupSignalHandler 设置信号处理，用于在服务器退出时清理 pprof 进程。
// 这个函数应该在 main 函数中被调用一次。
func setupSignalHandler() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Printf("Received signal: %s. Cleaning up running pprof processes...", sig)

		pprofMutex.Lock()
		pidsToTerminate := make([]int, 0, len(runningPprofs))
		processesToTerminate := make([]*os.Process, 0, len(runningPprofs))
		for pid, process := range runningPprofs {
			pidsToTerminate = append(pidsToTerminate, pid)
			processesToTerminate = append(processesToTerminate, process)
		}
		runningPprofs = make(map[int]*os.Process) // 清空 map
		pprofMutex.Unlock()

		if len(pidsToTerminate) == 0 {
			log.Println("No running pprof processes to terminate.")
			return
		}

		log.Printf("Terminating %d pprof processes: %v", len(pidsToTerminate), pidsToTerminate)
		var wg sync.WaitGroup
		wg.Add(len(processesToTerminate))

		for i, process := range processesToTerminate {
			go func(p *os.Process, pid int) {
				defer wg.Done()
				log.Printf("Sending Interrupt signal to PID %d...", pid)
				err := p.Signal(os.Interrupt)
				if err != nil {
					log.Printf("Failed to send Interrupt to PID %d: %v. Trying Kill.", pid, err)
					err = p.Signal(os.Kill)
					if err != nil {
						log.Printf("Failed to send Kill to PID %d: %v", pid, err)
					}
				}
			}(process, pidsToTerminate[i])
		}
		wg.Wait() // 等待所有终止 goroutine 完成尝试
		log.Println("Cleanup finished.")
	}()
}
