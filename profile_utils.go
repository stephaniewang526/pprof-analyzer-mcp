package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
		// 可以在这里添加 os.Stat 检查文件是否存在且可读
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
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", nil, fmt.Errorf("failed to download profile from '%s': received status code %d", uriStr, resp.StatusCode)
		}

		// 创建临时文件来存储下载的内容
		tempFile, err := os.CreateTemp("", "pprof-*") // 使用通用模式
		if err != nil {
			return "", nil, fmt.Errorf("failed to create temporary file for download: %w", err)
		}
		filePath = tempFile.Name()
		log.Printf("Downloading profile to temporary file: %s", filePath)

		// 定义清理函数，用于删除临时文件
		cleanup = func() {
			log.Printf("Cleaning up temporary file: %s", filePath)
			err := os.Remove(filePath)
			if err != nil && !os.IsNotExist(err) {
				log.Printf("Warning: failed to remove temporary file '%s': %v", filePath, err)
			}
		}

		_, err = io.Copy(tempFile, resp.Body)
		closeErr := tempFile.Close()

		if err != nil {
			cleanup() // 如果复制失败，尝试清理临时文件
			return "", nil, fmt.Errorf("failed to write downloaded content to temporary file '%s': %w", filePath, err)
		}
		if closeErr != nil {
			log.Printf("Warning: failed to close temporary file handle for '%s': %v", filePath, closeErr)
		}

		log.Printf("Successfully downloaded profile to %s", filePath)
		return filePath, cleanup, nil

	default:
		return "", nil, fmt.Errorf("unsupported URI scheme '%s', only 'file://', 'http://', 'https://', or a plain local path are supported", parsedURI.Scheme)
	}
}
