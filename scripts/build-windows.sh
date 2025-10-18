#!/bin/bash
# Windows交叉编译脚本
# 用于在WSL/Linux环境下编译Windows可执行文件

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}==> 开始编译 Windows 版本...${NC}"

# 检查操作系统
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
    echo -e "${YELLOW}检测到Windows环境,使用原生编译${NC}"
    # Windows原生编译
    go build -ldflags "-H windowsgui -s -w" -o interview.exe cmd/gui/main.go
    echo -e "${GREEN}✓ 编译成功 (Windows原生): interview.exe${NC}"
    exit 0
fi

# WSL/Linux环境 - 交叉编译
echo -e "${YELLOW}检测到WSL/Linux环境,使用交叉编译${NC}"

# 方案1: 尝试使用Windows MSYS64中的MinGW (WSL专用)
if [ -f "/mnt/c/msys64/mingw64/bin/x86_64-w64-mingw32-gcc.exe" ]; then
    echo -e "${GREEN}找到Windows MinGW,使用方案1${NC}"
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
        CC="/mnt/c/msys64/mingw64/bin/x86_64-w64-mingw32-gcc.exe" \
        go build -ldflags "-s -w" -o interview.exe cmd/gui/main.go
    echo -e "${GREEN}✓ 编译成功 (方案1 - Windows MinGW): interview.exe${NC}"
    exit 0
fi

# 方案2: 尝试使用Linux系统的MinGW
if command -v x86_64-w64-mingw32-gcc &> /dev/null; then
    echo -e "${GREEN}找到系统MinGW,使用方案2${NC}"
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
        CC=x86_64-w64-mingw32-gcc \
        go build -ldflags "-s -w" -o interview.exe cmd/gui/main.go
    echo -e "${GREEN}✓ 编译成功 (方案2 - 系统MinGW): interview.exe${NC}"
    exit 0
fi

# 方案3: 没有MinGW,提示用户安装
echo -e "${RED}错误: 未找到MinGW交叉编译器${NC}"
echo ""
echo -e "${YELLOW}请选择以下解决方案之一:${NC}"
echo ""
echo "方案A - 安装MinGW (推荐):"
echo "  sudo apt-get update"
echo "  sudo apt-get install -y mingw-w64"
echo ""
echo "方案B - 在Windows环境中编译:"
echo "  1. 打开Windows PowerShell"
echo "  2. cd 到项目目录"
echo "  3. 运行: go build -ldflags \"-H windowsgui -s -w\" -o interview.exe cmd\\gui\\main.go"
echo ""
echo "方案C - 使用GitHub Actions自动构建:"
echo "  参考: .github/workflows/build.yml"
echo ""

exit 1
