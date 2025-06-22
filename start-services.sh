#!/bin/bash

# Jaeger Demo 微服务启动脚本
# 作者: AI Assistant
# 描述: 同时启动所有微服务

set -e  # 遇到错误时退出

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

# 日志文件目录 (使用绝对路径)
LOG_DIR="$SCRIPT_DIR/logs"
PID_FILE="$SCRIPT_DIR/services.pid"

# 创建日志目录
mkdir -p "$LOG_DIR"

echo -e "${BLUE}🚀 开始启动 Jaeger Demo 微服务...${NC}"

# 检查是否已经有服务在运行
if [ -f "$PID_FILE" ]; then
    echo -e "${YELLOW}⚠️  检测到已有服务在运行，请先运行 stop-services.sh${NC}"
    exit 1
fi

# 启动 Jaeger (如果安装了)
echo -e "${BLUE}📊 启动 Jaeger...${NC}"
if command -v jaeger-all-in-one &> /dev/null; then
    jaeger-all-in-one --collector.http-port=14268 --collector.grpc-port=14250 --agent.grpc-port=14250 --agent.http-port=14268 --query.port=16686 --query.base-path=/ &
    JAEGER_PID=$!
    echo $JAEGER_PID >> "$PID_FILE"
    echo -e "${GREEN}✅ Jaeger 已启动 (PID: $JAEGER_PID)${NC}"
else
    echo -e "${YELLOW}⚠️  Jaeger 未安装，请手动启动或使用 Docker: docker run -d --name jaeger -p 16686:16686 -p 14268:14268 jaegertracing/all-in-one:latest${NC}"
fi

# 启动所有微服务
for service in "${SERVICES[@]}"; do
    IFS=':' read -r service_name port <<< "$service"
    
    echo -e "${BLUE}🔧 启动 $service_name (端口: $port)...${NC}"
    
    # 构建服务路径
    service_path="$SCRIPT_DIR/cmd/$service_name"
    
    # 检查服务目录是否存在
    if [ ! -d "$service_path" ]; then
        echo -e "${RED}❌ 服务目录不存在: $service_path${NC}"
        continue
    fi
    
    # 启动服务
    cd "$service_path"
    go build -o "$SCRIPT_DIR/deploy/${service_name}"
    "$SCRIPT_DIR/deploy/${service_name}" > "$LOG_DIR/$service_name.log" 2>&1 &
    SERVICE_PID=$!
    cd "$SCRIPT_DIR" > /dev/null
    
    # 保存PID
    echo $SERVICE_PID >> "$PID_FILE"
    
    echo -e "${GREEN}✅ $service_name 已启动 (PID: $SERVICE_PID, 端口: $port)${NC}"
    
    # 等待服务启动
    sleep 2
done

echo -e "${GREEN}🎉 所有服务启动完成！${NC}"
echo -e "${BLUE}📋 服务状态:${NC}"
echo -e "  - Jaeger UI: http://localhost:16686"
echo -e "  - API Gateway: http://localhost:8080"
echo -e "  - Order Service: http://localhost:8081"
echo -e "  - Shipping Service: http://localhost:8082"
echo -e "  - Fraud Detection Service: http://localhost:8083"
echo -e "  - Pricing Service: http://localhost:8084"
echo -e "  - Notification Service: http://localhost:8085"
echo -e "  - Inventory Service: http://localhost:8086"
echo -e "${BLUE}📁 日志文件位置: $LOG_DIR${NC}"
echo -e "${BLUE}🛑 停止服务请运行: ./stop-services.sh${NC}"


echo "正常: curl 'http://localhost:8080/create_complex_order?userID=user-789&is_vip=false&items=item-a,item-b'"
echo "异常: curl 'http://localhost:8080/create_complex_order?userID=user-vip-123&is_vip=true&items=item-a,item-b'"