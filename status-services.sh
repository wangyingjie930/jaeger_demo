#!/bin/bash

# Jaeger Demo 微服务状态检查脚本
# 作者: AI Assistant
# 描述: 检查所有微服务的运行状态

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 服务配置
SERVICES=(
    "api-gateway:8080"
    "order-service:8081"
    "shipping-service:8082"
    "fraud-detection-service:8083"
    "pricing-service:8084"
    "notification-service:8085"
    "inventory-service:8086"
)

# PID文件 (使用绝对路径)
PID_FILE="$SCRIPT_DIR/services.pid"

echo -e "${BLUE}📊 Jaeger Demo 微服务状态检查${NC}"
echo "=================================="

# 检查PID文件
if [ -f "$PID_FILE" ]; then
    echo -e "${GREEN}✅ 找到运行中的服务${NC}"
    echo -e "${BLUE}📋 运行中的进程:${NC}"
    
    while IFS= read -r pid; do
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            # 获取进程信息
            process_info=$(ps -p "$pid" -o pid,ppid,cmd --no-headers 2>/dev/null)
            echo -e "  ${GREEN}● PID: $pid${NC}"
            echo -e "    $process_info"
        else
            echo -e "  ${RED}● PID: $pid (已停止)${NC}"
        fi
    done < "$PID_FILE"
else
    echo -e "${YELLOW}⚠️  没有找到运行中的服务${NC}"
fi

echo ""
echo -e "${BLUE}🌐 服务端口检查:${NC}"

# 检查Jaeger
if curl -s http://localhost:16686 > /dev/null 2>&1; then
    echo -e "  ${GREEN}● Jaeger UI: http://localhost:16686 (运行中)${NC}"
else
    echo -e "  ${RED}● Jaeger UI: http://localhost:16686 (未运行)${NC}"
fi

# 检查各个微服务
for service in "${SERVICES[@]}"; do
    IFS=':' read -r service_name port <<< "$service"
    
    if curl -s http://localhost:$port > /dev/null 2>&1; then
        echo -e "  ${GREEN}● $service_name: http://localhost:$port (运行中)${NC}"
    else
        echo -e "  ${RED}● $service_name: http://localhost:$port (未运行)${NC}"
    fi
done

echo ""
echo -e "${BLUE}📁 日志文件:${NC}"
if [ -d "$SCRIPT_DIR/logs" ]; then
    for log_file in "$SCRIPT_DIR/logs"/*.log; do
        if [ -f "$log_file" ]; then
            service_name=$(basename "$log_file" .log)
            file_size=$(du -h "$log_file" | cut -f1)
            echo -e "  ${BLUE}● $service_name.log (${file_size})${NC}"
        fi
    done
else
    echo -e "  ${YELLOW}⚠️  日志目录不存在${NC}"
fi

echo ""
echo -e "${BLUE}💡 使用说明:${NC}"
echo -e "  - 启动服务: ./start-services.sh"
echo -e "  - 停止服务: ./stop-services.sh"
echo -e "  - 重启服务: ./restart-services.sh" 