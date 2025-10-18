# Windows 编译指南

## 快速解决方案

### 问题症状
```
go build -ldflags "-H windowsgui -s -w" -o interview.exe cmd/gui/main.go
panic: runtime error: invalid memory address or nil pointer dereference
```

### 立即修复

**在WSL/Linux环境** (移除 `-H windowsgui` 标志):
```bash
# 使用提供的脚本 (推荐)
./scripts/build-windows.sh

# 或手动编译
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC="/mnt/c/msys64/mingw64/bin/x86_64-w64-mingw32-gcc.exe" \
  go build -ldflags "-s -w" -o interview.exe cmd/gui/main.go
```

**在Windows PowerShell** (可使用所有标志):
```powershell
go build -ldflags "-H windowsgui -s -w" -o interview.exe cmd\gui\main.go
```

## 为什么会失败?

1. **`-H windowsgui` 标志问题**: 此标志仅在Windows原生编译时稳定,在交叉编译时可能触发Go链接器Bug
2. **Go 1.24.x 已知问题**: 特定版本的Go在PE文件生成时存在空指针Bug
3. **交叉编译复杂性**: CGO + Fyne + 交叉编译组合增加了失败概率

## 三种编译方案

### 方案1: WSL/Linux 交叉编译 (开发使用)

**特点**: 不显示 `-H windowsgui`,程序启动时会有控制台窗口

```bash
# 自动化脚本 (推荐)
./scripts/build-windows.sh

# 手动编译
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC="/mnt/c/msys64/mingw64/bin/x86_64-w64-mingw32-gcc.exe" \
  go build -ldflags "-s -w" -o interview.exe cmd/gui/main.go
```

**优点**:
- 在Linux开发环境直接构建
- 避免链接器崩溃
- 编译速度快

**缺点**:
- 程序启动时会显示控制台窗口 (可忽略或后期修复)

### 方案2: Windows 原生编译 (生产发布)

**特点**: 完全支持 `-H windowsgui`,无控制台窗口

```powershell
# Windows PowerShell中执行
cd path\to\interview-ai-go
go build -ldflags "-H windowsgui -s -w" -o interview.exe cmd\gui\main.go
```

**优点**:
- 稳定可靠
- 支持所有Windows特定优化
- 生成纯GUI程序

**缺点**:
- 需要Windows环境

### 方案3: GitHub Actions 自动化 (CI/CD)

**特点**: 自动化多平台构建

创建 `.github/workflows/build.yml`:
```yaml
name: Build Releases

on:
  push:
    branches: [ main ]
    tags: [ 'v*' ]

jobs:
  build-windows:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Build Windows
        run: |
          go build -ldflags "-H windowsgui -s -w" -o interview.exe cmd\gui\main.go

      - name: Upload artifact
        uses: actions/upload-artifact@v3
        with:
          name: interview-windows
          path: interview.exe
```

## 隐藏控制台窗口的替代方案

如果使用方案1编译,有以下方法隐藏控制台:

### 方法A: 编译后修改PE头 (需要Windows)

```powershell
# 使用Visual Studio的editbin工具
editbin /SUBSYSTEM:WINDOWS interview.exe
```

### 方法B: 在代码中隐藏 (不推荐,增加复杂度)

创建 `internal/ui/console_windows.go`:
```go
//go:build windows
// +build windows

package ui

import "syscall"

func init() {
    hideConsole()
}

func hideConsole() {
    kernel32 := syscall.NewLazyDLL("kernel32.dll")
    console := kernel32.NewProc("GetConsoleWindow")

    if console.Find() == nil {
        hwnd, _, _ := console.Call()
        if hwnd != 0 {
            user32 := syscall.NewLazyDLL("user32.dll")
            showWindow := user32.NewProc("ShowWindow")
            showWindow.Call(hwnd, 0) // SW_HIDE = 0
        }
    }
}
```

然后在 `cmd/gui/main.go` 中导入:
```go
import _ "interviewai/internal/ui"  // 自动执行init()
```

### 方法C: 使用第三方构建工具

```bash
# 使用goversioninfo (需要安装)
go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo

# 创建versioninfo.json配置文件
# 运行goversioninfo生成.syso资源文件
goversioninfo -64

# 正常编译 (会自动嵌入资源文件)
go build -o interview.exe cmd/gui/main.go
```

## 验证编译结果

### 检查编译产物

```bash
# 在WSL/Linux
ls -lh interview.exe
file interview.exe
# 应输出: PE32+ executable (console) x86-64 (stripped to external PDB), for MS Windows

# 在Windows PowerShell
Get-Item interview.exe | Select-Object *
```

### 测试运行

```powershell
# 在Windows上测试
.\interview.exe -c config\config.yaml
```

## 完整的构建Makefile

创建 `Makefile`:

```makefile
.PHONY: windows windows-wsl windows-native clean

# WSL/Linux环境交叉编译Windows版本
windows-wsl:
	@echo "==> Building for Windows (WSL cross-compile)..."
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
		CC="/mnt/c/msys64/mingw64/bin/x86_64-w64-mingw32-gcc.exe" \
		go build -ldflags "-s -w" -o interview.exe cmd/gui/main.go
	@echo "✓ Build complete: interview.exe"

# Windows原生环境编译 (PowerShell中使用)
windows-native:
	@echo "==> Building for Windows (native compile)..."
	go build -ldflags "-H windowsgui -s -w" -o interview.exe cmd/gui/main.go
	@echo "✓ Build complete: interview.exe"

# 清理编译产物
clean:
	rm -f interview.exe interview-*.exe

# 自动检测环境
windows:
	@if [ -f "/mnt/c/msys64/mingw64/bin/x86_64-w64-mingw32-gcc.exe" ]; then \
		$(MAKE) windows-wsl; \
	else \
		echo "Error: MinGW not found. Please use 'make windows-native' on Windows"; \
		exit 1; \
	fi
```

**使用方式**:
```bash
# WSL/Linux
make windows-wsl

# Windows (在Git Bash或PowerShell中)
make windows-native

# 清理
make clean
```

## 故障排查

### 问题1: "cgo: C compiler not found"

**原因**: 未安装MinGW交叉编译器

**解决**:
```bash
# WSL/Ubuntu
sudo apt-get update
sudo apt-get install -y mingw-w64

# 或使用Windows MSYS64 (如果已安装)
# 无需额外安装,使用脚本自动检测
```

### 问题2: "undefined reference to `_imp_*`"

**原因**: 链接器找不到Windows库

**解决**:
```bash
# 确保使用正确的MinGW编译器
which x86_64-w64-mingw32-gcc

# 或指定完整路径
CC="/mnt/c/msys64/mingw64/bin/x86_64-w64-mingw32-gcc.exe"
```

### 问题3: 编译成功但Windows上无法运行

**原因**: 缺少运行时DLL

**解决**:
```bash
# 静态链接所有依赖 (增加体积)
CGO_LDFLAGS="-static -static-libgcc -static-libstdc++" \
  GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc \
  go build -ldflags "-s -w" -o interview.exe cmd/gui/main.go
```

### 问题4: 链接器崩溃 (本问题)

**原因**: `-H windowsgui` 与交叉编译冲突

**解决**:
```bash
# 移除 -H windowsgui 标志
# 使用scripts/build-windows.sh脚本
./scripts/build-windows.sh
```

## 推荐工作流

### 日常开发

```bash
# 在WSL/Linux上开发和测试Linux版本
go run cmd/gui/main.go -c config/config.yaml

# 偶尔编译Windows版本测试
./scripts/build-windows.sh
```

### 发布准备

```bash
# 方式1: 在Windows机器上编译 (推荐)
go build -ldflags "-H windowsgui -s -w" -o interview.exe cmd\gui\main.go

# 方式2: 使用GitHub Actions自动构建
git tag v1.0.0
git push --tags
# 等待Actions完成,下载artifacts
```

### 多平台构建

```bash
# Linux
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
  go build -ldflags "-s -w" -o interview-linux cmd/gui/main.go

# macOS
GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 \
  go build -ldflags "-s -w" -o interview-darwin cmd/gui/main.go

# Windows
./scripts/build-windows.sh
```

## 总结

| 场景 | 推荐方案 | 命令 |
|------|---------|------|
| 开发测试 | WSL交叉编译 | `./scripts/build-windows.sh` |
| 生产发布 | Windows原生 | `go build -ldflags "-H windowsgui -s -w"` |
| CI/CD | GitHub Actions | 提交代码自动构建 |

**核心要点**:
- ❌ 不要在WSL/Linux交叉编译时使用 `-H windowsgui`
- ✅ 使用提供的 `scripts/build-windows.sh` 脚本
- ✅ 生产发布时在Windows环境原生编译

---

**文档版本**: 1.0
**最后更新**: 2025-10-18
**相关文档**: `claudedocs/windows-build-fix.md`
