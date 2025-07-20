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

# 设置环境变量 (从configmap中提取，保持原始配置)
export JAEGER_ENDPOINT="http://jaeger.infra:14268/api/traces"
export KAFKA_BROKERS="kafka-service.infra:9092"
export NACOS_SERVER_ADDRS="nacos.infra:8848"
export ORDER_SERVICE_BASE_URL="http://localhost:8081"
export FRAUD_DETECTION_SERVICE_URL="http://localhost:8085/check"
export INVENTORY_SERVICE_URL="http://localhost:8082"
export INVENTORY_RELEASE_URL="http://localhost:8082/release_stock"
export INVENTORY_RESERVE_URL="http://localhost:8082/reserve_stock"
export NOTIFICATION_SERVICE_URL="http://localhost:8083"
export PRICING_SERVICE_URL="http://localhost:8084/calculate_price"
export PROMOTION_SERVICE_URL="http://localhost:8087/get_promo_price"
export SHIPPING_SERVICE_URL="http://localhost:8086/get_quote"
export DB_SOURCE="root:root@tcp(mysql.infra:3306)/test"
export REDIS_ADDR="redis.infra:6379"
export ZK_SERVERS="zookeeper-headless.infra:2181"
export REDIS_ADDRS="redis-cluster-0.redis-cluster-headless.infra:6379,redis-cluster-1.redis-cluster-headless.infra:6379,redis-cluster-2.redis-cluster-headless.infra:6379"
export NACOS_NAMESPACE="d586122c-170f-40e9-9d17-5cede728cd7e" # 假设这是开发环境的Namespace ID
export NACOS_GROUP="nexus-group"   # 为项目所有服务定义一个统一的分组

# <<<<<<< 改造点: 增加新服务 >>>>>>>>>
SERVICES=(
#    "api-gateway:8080"
    "order-service:8081"
    "inventory-service:8082"
    "notification-service:8083" # 端口改为消费Kafka，脚本中保留便于管理
    "pricing-service:8084"
    "fraud-detection-service:8085"
    "shipping-service:8086"
    "promotion-service:8087"  # 新增
    "delay-scheduler"
)
# <<<<<<< 改造点结束 >>>>>>>>>

# 日志文件目录 (使用绝对路径)
LOG_DIR="$SCRIPT_DIR/logs"
PID_FILE="$SCRIPT_DIR/services.pid"

# 创建日志目录
mkdir -p "$LOG_DIR"
# 创建部署目录
mkdir -p "$SCRIPT_DIR/deploy"

echo -e "${BLUE}🚀 开始启动 Jaeger Demo 微服务...${NC}"

# 清理旧的PID文件
rm -f "$PID_FILE"

# 检查并杀死可能残留的旧进程
echo -e "${YELLOW}🔧 正在清理可能残留的旧服务进程...${NC}"
for service_config in "${SERVICES[@]}"; do
    service_name="${service_config%%:*}"
    binary_path="$SCRIPT_DIR/deploy/${service_name}"
    if pgrep -f "$binary_path" > /dev/null; then
        pkill -f "$binary_path"
        echo -e "${GREEN}✅ 已停止残留的 $service_name 服务${NC}"
        sleep 1
    fi
done


# 启动 Jaeger (如果安装了)
echo -e "${BLUE}📊 检查 Jaeger 状态...${NC}"
if ! curl -s http://localhost:16686 > /dev/null; then
    echo -e "${YELLOW}⚠️  Jaeger 未运行，尝试启动...${NC}"
    if command -v jaeger-all-in-one &> /dev/null; then
        jaeger-all-in-one --collector.http-port=14268 --query.port=16686 > "$LOG_DIR/jaeger.log" 2>&1 &
        JAEGER_PID=$!
        echo $JAEGER_PID >> "$PID_FILE"
        echo -e "${GREEN}✅ Jaeger 已启动 (PID: $JAEGER_PID)${NC}"
        sleep 3 # 等待Jaeger启动
    else
        echo -e "${YELLOW}⚠️  Jaeger 命令不存在。请手动启动或使用 Docker: \ndocker run -d --name jaeger -p 16686:16686 -p 14268:14268 jaegertracing/all-in-one:latest${NC}"
    fi
else
    echo -e "${GREEN}✅ Jaeger 已在运行中${NC}"
fi

# 编译和启动所有微服务
for service_config in "${SERVICES[@]}"; do
    service_name="${service_config%%:*}"
    port="${service_config##*:}"

    echo -e "${BLUE}🔧 编译并启动 $service_name (端口: $port)...${NC}"

    service_path="$SCRIPT_DIR/cmd/$service_name"
    binary_path="$SCRIPT_DIR/deploy/${service_name}"

    if [ ! -d "$service_path" ]; then
        # notification-service 没有 main.go 了，但目录可能存在
        if [ "$service_name" == "notification-service" ]; then
             echo -e "${YELLOW}🔍 $service_name 是一个Kafka消费者，后台运行，跳过HTTP端口检查。${NC}"
        else
            echo -e "${RED}❌ 服务目录不存在: $service_path${NC}"
            continue
        fi
    fi

    # 编译
    (cd "$service_path" && go build -o "$binary_path")
    if [ $? -ne 0 ]; then
        echo -e "${RED}❌ 编译失败: $service_name${NC}"
        exit 1
    fi

    # 启动
    "$binary_path" > "$LOG_DIR/$service_name.log" 2>&1 &
    SERVICE_PID=$!
    echo $SERVICE_PID >> "$PID_FILE"

    echo -e "${GREEN}✅ $service_name 已启动 (PID: $SERVICE_PID)${NC}"
    sleep 1
done

# curl 'http://localhost:9081/create_complex_order?userId=user-normal-4567&is_vip=false&items=item-a,item-b' -H 'Host: nexus.local'

echo -e "${GREEN}🎉 所有服务启动完成！${NC}"
echo -e "${BLUE}📋 服务状态和访问点:${NC}"
echo -e "  - Jaeger UI: http://localhost:16686"
echo ""
echo -e "${YELLOW}💡 测试指令示例:${NC}"
echo -e "  ${GREEN}正常普通用户订单:${NC}"
echo -e "  curl 'http://localhost:8081/create_complex_order?userId=user-normal-4567&is_vip=false&items=item-a,item-b'"
echo -e "  curl 'http://localhost:8081/create_complex_order?userId=user-normal-4567&is_vip=false&items=item-a,item-b&seckill_product_id=product_123'"
echo -e "  ${GREEN}VIP用户大促订单:${NC}"
echo -e "  curl 'http://localhost:8081/create_complex_order?userId=user-vip-789&is_vip=true&items=item-a,item-b'"
echo -e "  ${RED}价格服务故障的VIP订单 (触发SAGA补偿):${NC}"
echo -e "  curl 'http://localhost:8081/create_complex_order?userId=user-vip-789&is_vip=true&items=item-a,item-faulty-123&quantity=11'"
echo -e "  curl 'http://localhost:8081/create_complex_order?userId=user-normal-456&is_vip=false&items=item-a,item-b'"
echo ""
echo -e "${BLUE}📁 日志文件位置: $LOG_DIR${NC}"
echo -e "${BLUE}🛑 停止服务请运行: ./stop-services.sh${NC}"