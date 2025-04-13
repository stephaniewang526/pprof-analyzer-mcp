[简体中文](README_zh-CN.md) | English

# Pprof Analyzer MCP Server

This is a Model Context Protocol (MCP) server implemented in Go, providing a tool to analyze Go pprof performance profiles.

## Features

*   **`analyze_pprof` Tool:**
    *   Analyzes local pprof files (currently only supports `file://` URIs).
    *   Supported Profile Types:
        *   `cpu`: Analyzes CPU time consumption during code execution to find hot spots.
        *   `heap`: Analyzes the current memory usage (heap allocations) to find objects and functions with high memory consumption.
        *   `goroutine`: Displays stack traces of all current goroutines, used for diagnosing deadlocks, leaks, or excessive goroutine usage.
        *   `allocs`: Analyzes memory allocations (including freed ones) during program execution to locate code with frequent allocations. (*Not yet implemented*)
        *   `mutex`: Analyzes contention on mutexes to find locks causing blocking. (*Not yet implemented*)
        *   `block`: Analyzes operations causing goroutine blocking (e.g., channel waits, system calls). (*Not yet implemented*)
    *   Supported Output Formats: `text`, `markdown`, `json`.
        *   `text` and `markdown` are implemented.
        *   `json`: Outputs Top N results in structured JSON format (implemented for `cpu`, `heap`, `goroutine`).
    *   Configurable number of results to return (`top_n`, defaults to 5).
*   **`generate_flamegraph` Tool:**
    *   Uses the `go tool pprof` command to generate an SVG flame graph for the specified pprof file.
    *   Supported Profile Types: `cpu`, `heap`, `allocs`, `goroutine`, `mutex`, `block`.
    *   Requires the user to specify the output SVG file path.
    *   **Important:** This feature depends on [Graphviz](#dependencies) being installed.

## Installation (As a Library/Tool)

You can install this package directly using `go install`:

```bash
go install github.com/ZephyrDeng/pprof-analyzer-mcp@latest
```
This will install the `pprof-analyzer-mcp` executable to your `$GOPATH/bin` or `$HOME/go/bin` directory. Ensure this directory is in your system's PATH to run the command directly.

## Building from Source

Ensure you have a Go environment installed (Go 1.18 or higher recommended).

In the project root directory (`pprof-analyzer-mcp`), run:

```bash
go build
```

This will generate an executable file named `pprof-analyzer-mcp` (or `pprof-analyzer-mcp.exe` on Windows) in the current directory.

### Using `go install` (Recommended)

You can also use `go install` to install the executable into your `$GOPATH/bin` or `$HOME/go/bin` directory. This allows you to run `pprof-analyzer-mcp` directly from the command line (if the directory is added to your system's PATH environment variable).

```bash
# Installs the executable using the module path defined in go.mod
go install .
# Or directly using the GitHub path (recommended after publishing)
# go install github.com/ZephyrDeng/pprof-analyzer-mcp@latest
```

## Releasing (Automated via GitHub Actions)

This project uses [GoReleaser](https://goreleaser.com/) and GitHub Actions to automate the release process. Releases are triggered automatically when a Git tag matching the pattern `v*` (e.g., `v0.1.0`, `v1.2.3`) is pushed to the repository.

**Release Steps:**

1.  **Make Changes:** Develop new features or fix bugs.
2.  **Commit Changes:** Commit your changes using [Conventional Commits](https://www.conventionalcommits.org/) format (e.g., `feat: ...`, `fix: ...`). This is important for automatic changelog generation.
    ```bash
    git add .
    git commit -m "feat: Add awesome new feature"
    # or
    git commit -m "fix: Resolve issue #42"
    ```
3.  **Push Changes:** Push your commits to the main branch on GitHub.
    ```bash
    git push origin main
    ```
4.  **Create and Push Tag:** When ready to release, create a new Git tag and push it to GitHub.
    ```bash
    # Example: Create tag v0.1.0
    git tag v0.1.0

    # Push the tag to GitHub
    git push origin v0.1.0
    ```
5.  **Automatic Release:** Pushing the tag will trigger the `GoReleaser` GitHub Action defined in `.github/workflows/release.yml`. This action will:
    *   Build binaries for Linux, macOS, and Windows (amd64 & arm64).
    *   Generate a changelog based on Conventional Commits since the last tag.
    *   Create a new GitHub Release with the changelog and attach the built binaries and checksums as assets.

You can view the release workflow progress in the "Actions" tab of the GitHub repository.

## Configuring the MCP Client

This server uses the `stdio` transport protocol. You need to configure it in your MCP client (e.g., Roo Cline extension for VS Code).

Typically, this involves adding the following configuration to the `.roo/mcp.json` file in your project root:

```json
{
  "mcpServers": {
    "pprof-analyzer": { // You can choose a name for the server
      // Specify the command to start the server.
      // If you used 'go build', provide the **absolute path** or **relative path** (from project root) to the compiled executable.
      // Example: "command": "/path/to/your/pprof-analyzer-mcp/pprof-analyzer-mcp"
      // Or: "command": "./pprof-analyzer-mcp/pprof-analyzer-mcp"
      //
      // If you used 'go install github.com/ZephyrDeng/pprof-analyzer-mcp@latest'
      // and $GOPATH/bin or $HOME/go/bin is in your PATH,
      // you can use the executable name directly:
      "command": "pprof-analyzer-mcp"
      // Alternatively, if built and installed from source (go install .):
      // "command": "pprof-analyzer-mcp" (if in PATH)
      // Or specify the built path:
      // "command": "./pprof-analyzer-mcp/pprof-analyzer-mcp"
    }
    // ... other server configurations
  }
}
```

**Note:** Adjust the `command` value based on your build method (`go build` or `go install`) and the actual location of the executable. Ensure the MCP client can find and execute this command.

After configuration, reload or restart your MCP client, and it should automatically connect to the `PprofAnalyzer` server.

## Dependencies

*   **Graphviz**: The `generate_flamegraph` tool requires Graphviz to generate SVG flame graphs (the `go tool pprof` command calls `dot` when generating SVG). Ensure Graphviz is installed on your system and the `dot` command is available in your system's PATH environment variable.

    **Installing Graphviz:**
    *   **macOS (using Homebrew):**
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
        # or
        sudo dnf install graphviz
        ```
    *   **Windows (using Chocolatey):**
        ```bash
        choco install graphviz
        ```
    *   **Other Systems:** Refer to the [Graphviz official download page](https://graphviz.org/download/).

## Usage Examples (via MCP Client)

Once the server is connected, you can call the `analyze_pprof` and `generate_flamegraph` tools.

**Example: Analyze CPU Profile (Text format, Top 5)**

```json
{
  "tool_name": "analyze_pprof",
  "arguments": {
    "profile_uri": "file:///path/to/your/cpu.pprof",
    "profile_type": "cpu"
  }
}
```

**Example: Analyze Heap Profile (Markdown format, Top 10)**

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

**Example: Analyze Goroutine Profile (Text format, Top 5)**

```json
{
  "tool_name": "analyze_pprof",
  "arguments": {
    "profile_uri": "file:///path/to/your/goroutine.pprof",
    "profile_type": "goroutine"
  }
}
```

**Example: Generate Flame Graph for CPU Profile**

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

**Example: Generate Flame Graph for Heap Profile (inuse_space)**

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

**Example: Analyze CPU Profile (JSON format, Top 3)**

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

## Future Improvements (TODO)

*   Implement full analysis logic for `allocs`, `mutex`, `block` profiles.
*   Implement `json` output format for `allocs`, `mutex`, `block` profile types.
*   Set appropriate MIME types in MCP results based on `output_format`.
*   Add more robust error handling and logging level control.
*   Consider supporting remote pprof file URIs (e.g., `http://`, `https://`).