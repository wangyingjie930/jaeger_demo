#!/bin/bash

# Jaeger Demo 微服务停止脚本
# 作者: AI Assistant
# 描述: 同时停止所有微服务

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# PID文件 (使用绝对路径)
PID_FILE="$SCRIPT_DIR/services.pid"

echo -e "${BLUE}🛑 开始停止 Jaeger Demo 微服务...${NC}"

./../nexus-order/stop.sh
./../nexus-promotion/stop.sh

# 检查PID文件是否存在
if [ ! -f "$PID_FILE" ]; then
    echo -e "${YELLOW}⚠️  没有找到运行中的服务${NC}"
    exit 0
fi

# 读取所有PID并停止进程
while IFS= read -r pid; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        echo -e "${BLUE}🔧 停止进程 (PID: $pid)...${NC}"
        kill -9 "$pid"
        
        # 等待进程结束
        for i in {1..10}; do
            if ! kill -0 "$pid" 2>/dev/null; then
                echo -e "${GREEN}✅ 进程已停止 (PID: $pid)${NC}"
                break
            fi
            sleep 1
        done
        
        # 如果进程仍然存在，强制杀死
        if kill -0 "$pid" 2>/dev/null; then
            echo -e "${YELLOW}⚠️  强制停止进程 (PID: $pid)...${NC}"
            kill -9 "$pid"
            echo -e "${GREEN}✅ 进程已强制停止 (PID: $pid)${NC}"
        fi
    else
        echo -e "${YELLOW}⚠️  进程不存在或已停止 (PID: $pid)${NC}"
    fi
done < "$PID_FILE"

# 删除PID文件
rm -f "$PID_FILE"

echo -e "${GREEN}🎉 所有服务已停止！${NC}"

# 清理日志文件（可选）
read -p "是否删除日志文件？(y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    if [ -d "$SCRIPT_DIR/logs" ]; then
        rm -rf "$SCRIPT_DIR/logs"
        echo -e "${GREEN}✅ 日志文件已删除${NC}"
    fi
fi

# 强制清理残留端口进程
for port in 8080 8081 8082 8083 8084 8085 8086; do
    pid=$(lsof -ti tcp:$port)
    if [ -n "$pid" ]; then
        echo -e "${YELLOW}⚠️  端口 $port 仍被进程 $pid 占用，强制杀死...${NC}"
        kill -9 $pid
        echo -e "${GREEN}✅ 端口 $port 已释放${NC}"
    fi
done