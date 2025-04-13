简体中文 | [English](README.md)

# Pprof Analyzer MCP 服务器

[![smithery badge](https://smithery.ai/badge/@ZephyrDeng/pprof-analyzer-mcp)](https://smithery.ai/server/@ZephyrDeng/pprof-analyzer-mcp)

这是一个基于 Go 语言实现的模型上下文协议 (MCP) 服务器，提供了一个用于分析 Go pprof 性能剖析文件的工具。

## 功能

*   **`analyze_pprof` 工具:**
    *   分析本地 pprof 文件 (目前仅支持 `file://` URI)。
    *   支持的 Profile 类型：
        *   `cpu`: 分析代码执行的 CPU 时间消耗，找出热点函数。
        *   `heap`: 分析程序当前的内存使用情况（堆内存分配），找出内存占用高的对象和函数。
        *   `goroutine`: 显示所有当前 Goroutine 的堆栈信息，用于诊断死锁、泄漏或 Goroutine 过多的问题。
        *   `allocs`: 分析程序运行期间的内存分配情况（包括已释放的），用于定位频繁分配内存的代码。(*暂未实现*)
        *   `mutex`: 分析互斥锁的竞争情况，找出导致阻塞的锁。(*暂未实现*)
        *   `block`: 分析导致 Goroutine 阻塞的操作（如 channel 等待、系统调用等）。(*暂未实现*)
    *   支持的输出格式：`text`, `markdown`, `json`。
        *   `text` 和 `markdown` 已实现。
        *   `json`: 以结构化 JSON 格式输出 Top N 结果 (已为 `cpu`, `heap`, `goroutine` 实现)。
    *   可配置返回结果数量 (`top_n`, 默认为 5)。
*   **`generate_flamegraph` 工具:**
    *   使用 `go tool pprof` 命令为指定的 pprof 文件生成 SVG 格式的火焰图。
    *   支持的 Profile 类型：`cpu`, `heap`, `allocs`, `goroutine`, `mutex`, `block`。
    *   需要用户指定输出 SVG 文件的路径。
    *   **重要：** 此功能依赖于 [Graphviz](#依赖项) 的安装。

## 安装 (作为库/工具)

你可以使用 `go install` 直接安装此包：

```bash
go install github.com/ZephyrDeng/pprof-analyzer-mcp@latest
```
这会将 `pprof-analyzer-mcp` 可执行文件安装到你的 `$GOPATH/bin` 或 `$HOME/go/bin` 目录下。请确保该目录已添加到你的系统 PATH 环境变量中，以便直接运行命令。

## 从源码构建

确保你已经安装了 Go 环境 (推荐 Go 1.18 或更高版本)。

在项目根目录 (`pprof-analyzer-mcp`) 下运行：

```bash
go build
```

这将生成一个名为 `pprof-analyzer-mcp` (或 `pprof-analyzer-mcp.exe` 在 Windows 上) 的可执行文件在当前目录下。

### 使用 `go install` (推荐)

你也可以使用 `go install` 将可执行文件安装到你的 `$GOPATH/bin` 或 `$HOME/go/bin` 目录下，这样可以直接在命令行中运行 `pprof-analyzer-mcp` (如果该目录已添加到你的系统 PATH 环境变量中)。

```bash
# 使用 go.mod 中定义的模块路径安装可执行文件
go install .
# 或者直接使用 GitHub 路径 (发布后推荐)
# go install github.com/ZephyrDeng/pprof-analyzer-mcp@latest
```

## 使用 Docker 运行

使用 Docker 是一种便捷的运行服务器的方式，因为它打包了必需的 Graphviz 依赖。

1.  **构建 Docker 镜像：**
    在项目根目录（包含 `Dockerfile` 文件的目录）下运行：
    ```bash
    docker build -t pprof-analyzer-mcp .
    ```

2.  **运行 Docker 容器：**
    ```bash
    docker run -i --rm pprof-analyzer-mcp
    ```
    *   `-i` 标志保持标准输入 (STDIN) 打开，这是此 MCP 服务器使用的 stdio 传输协议所必需的。
    *   `--rm` 标志表示容器退出时自动删除。

3.  **为 Docker 配置 MCP 客户端：**
    要将你的 MCP 客户端（如 Roo Cline）连接到在 Docker 内部运行的服务器，请更新你的 `.roo/mcp.json`：
    ```json
    {
      "mcpServers": {
        "pprof-analyzer-docker": { // 为 Docker 设置使用一个不同的名称
          // 使用 'docker run' 作为命令
          "command": "docker run -i --rm pprof-analyzer-mcp"
        }
        // ... 其他服务器配置
      }
    }
    ```
    在客户端尝试运行此命令之前，请确保 `pprof-analyzer-mcp` 镜像已在本地构建。


## 发布流程 (通过 GitHub Actions 自动化)

本项目使用 [GoReleaser](https://goreleaser.com/) 和 GitHub Actions 来自动化发布流程。当一个匹配 `v*` 模式（例如 `v0.1.0`, `v1.2.3`）的 Git 标签被推送到仓库时，会自动触发发布。

**发布步骤：**

1.  **进行更改：** 开发新功能或修复 Bug。
2.  **提交更改：** 使用 [Conventional Commits](https://www.conventionalcommits.org/) 格式提交你的更改 (例如 `feat: ...`, `fix: ...`)。这对自动生成 Changelog 很重要。
    ```bash
    git add .
    git commit -m "feat: 添加了很棒的新功能"
    # 或者
    git commit -m "fix: 解决了问题 #42"
    ```
3.  **推送更改：** 将你的提交推送到 GitHub 的主分支。
    ```bash
    git push origin main
    ```
4.  **创建并推送标签：** 准备好发布时，创建一个新的 Git 标签并将其推送到 GitHub。
    ```bash
    # 示例：创建标签 v0.1.0
    git tag v0.1.0

    # 推送标签到 GitHub
    git push origin v0.1.0
    ```
5.  **自动发布：** 推送标签将触发 `.github/workflows/release.yml` 中定义的 `GoReleaser` GitHub Action。此 Action 将会：
    *   为 Linux、macOS 和 Windows (amd64 & arm64) 构建二进制文件。
    *   基于自上一个标签以来的 Conventional Commits 生成 Changelog。
    *   在 GitHub 上创建一个新的 Release，包含 Changelog，并将构建好的二进制文件和校验和文件作为附件上传。

你可以在 GitHub 仓库的 "Actions" 标签页查看发布工作流的进度。

## 配置 MCP 客户端

本服务器使用 `stdio` 传输协议。你需要在你的 MCP 客户端 (例如 VS Code 的 Roo Cline 扩展) 中配置它。

通常，这需要在项目根目录的 `.roo/mcp.json` 文件中添加如下配置：

```json
{
  "mcpServers": {
    "pprof-analyzer": { // 你可以为服务器选择一个名称
      // 指定启动服务器的命令。
      // 如果你使用了 'go build'，这里需要填写编译出的可执行文件的 **绝对路径** 或 **相对路径** (相对于项目根目录)。
      // 例如："command": "/path/to/your/pprof-analyzer-mcp/pprof-analyzer-mcp"
      // 或者："command": "./pprof-analyzer-mcp/pprof-analyzer-mcp"
      //
      // 如果你使用了 'go install github.com/ZephyrDeng/pprof-analyzer-mcp@latest'
      // 并且 $GOPATH/bin 或 $HOME/go/bin 在你的 PATH 中，
      // 你可以直接使用可执行文件名：
      "command": "pprof-analyzer-mcp"
      // 或者，如果从源码构建并安装 (go install .):
      // "command": "pprof-analyzer-mcp" (if in PATH)
      // 或者指定构建后的路径：
      // "command": "./pprof-analyzer-mcp/pprof-analyzer-mcp"
    }
    // ... 其他服务器配置
  }
}
```

**注意：** `command` 的值需要根据你的构建方式（`go build` 或 `go install`）和可执行文件的实际位置进行调整。确保 MCP 客户端能够找到并执行这个命令。

配置完成后，重新加载或重启你的 MCP 客户端，它应该会自动连接到 `PprofAnalyzer` 服务器。

## 依赖项

*   **Graphviz**: `generate_flamegraph` 工具需要 Graphviz 来生成 SVG 火焰图 (`go tool pprof` 在生成 SVG 时会调用 `dot` 命令)。请确保你的系统已经安装了 Graphviz 并且 `dot` 命令在系统的 PATH 环境变量中。

    **安装 Graphviz:**
    *   **macOS (使用 Homebrew):**
        ```bash
        brew install graphviz
        ```
    *   **Debian/Ubuntu:**
        ```bash
        sudo apt-get update && sudo apt-get install graphviz
        ```
    *   **CentOS/Fedora:**
        ```bash
        sudo yum install graphviz
        # 或者
        sudo dnf install graphviz
        ```
    *   **Windows (使用 Chocolatey):**
        ```bash
        choco install graphviz
        ```
    *   **其他系统：** 请参考 [Graphviz 官方下载页面](https://graphviz.org/download/)。

## 使用示例 (通过 MCP 客户端)

一旦服务器连接成功，你就可以调用 `analyze_pprof` 和 `generate_flamegraph` 工具了。

**示例：分析 CPU Profile (文本格式，Top 5)**

```json
{
  "tool_name": "analyze_pprof",
  "arguments": {
    "profile_uri": "file:///path/to/your/cpu.pprof",
    "profile_type": "cpu"
  }
}
```

**示例：分析 Heap Profile (Markdown 格式，Top 10)**

```json
{
  "tool_name": "analyze_pprof",
  "arguments": {
    "profile_uri": "file:///path/to/your/heap.pprof",
    "profile_type": "heap",
    "top_n": 10,
    "output_format": "markdown"
  }
}
```

**示例：分析 Goroutine Profile (文本格式，Top 5)**

```json
{
  "tool_name": "analyze_pprof",
  "arguments": {
    "profile_uri": "file:///path/to/your/goroutine.pprof",
    "profile_type": "goroutine"
  }
}
```

**示例：生成 CPU Profile 的火焰图**

```json
{
  "tool_name": "generate_flamegraph",
  "arguments": {
    "profile_uri": "file:///path/to/your/cpu.pprof",
    "profile_type": "cpu",
    "output_svg_path": "/path/to/save/cpu_flamegraph.svg"
  }
}
```

**示例：生成 Heap Profile (inuse_space) 的火焰图**

```json
{
  "tool_name": "generate_flamegraph",
  "arguments": {
    "profile_uri": "file:///path/to/your/heap.pprof",
    "profile_type": "heap",
    "output_svg_path": "/path/to/save/heap_flamegraph.svg"
  }
}
```

**示例：分析 CPU Profile (JSON 格式，Top 3)**

```json
{
  "tool_name": "analyze_pprof",
  "arguments": {
    "profile_uri": "file:///path/to/your/cpu.pprof",
    "profile_type": "cpu",
    "top_n": 3,
    "output_format": "json"
  }
}
```

## 未来改进 (TODO)

*   实现 `allocs`, `mutex`, `block` profile 的完整分析逻辑。
*   为 `allocs`, `mutex`, `block` profile 类型实现 `json` 输出格式。
*   在 MCP 结果中根据 `output_format` 设置合适的 MIME 类型。
*   增加更健壮的错误处理和日志级别控制。
*   考虑支持远程 pprof 文件 URI (例如 `http://`, `https://`)。