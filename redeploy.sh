#!/bin/bash

# redeploy.sh
# 重新构建镜像并部署到 Kubernetes
# 用法: ./redeploy.sh [服务名]
# 如果不传参数，则重新部署所有服务；如果传入服务名，则只重新部署该服务

# -e: 如果任何命令失败（返回非零退出状态），脚本将立即退出。
# -o pipefail: 如果管道中的任何命令失败，则整个管道的退出状态为失败。
set -eo pipefail

# 颜色定义，用于美化输出
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 服务列表
SERVICES=(
    "api-gateway"
    "order-service"
    "inventory-service"
    "notification-service"
    "pricing-service"
    "fraud-detection-service"
    "shipping-service"
    "promotion-service"
)

# 检查是否传入了服务名参数
if [ $# -eq 1 ]; then
    TARGET_SERVICE="$1"
    
    # 验证传入的服务名是否在服务列表中
    if [[ " ${SERVICES[@]} " =~ " ${TARGET_SERVICE} " ]]; then
        echo -e "${BLUE}🚀 开始重新部署指定服务: ${YELLOW}${TARGET_SERVICE}${NC}"
        SERVICES_TO_DEPLOY=("$TARGET_SERVICE")
    else
        echo -e "${RED}❌ 错误: 服务 '${TARGET_SERVICE}' 不在支持的服务列表中${NC}"
        echo -e "${YELLOW}支持的服务列表:${NC}"
        printf '%s\n' "${SERVICES[@]}"
        exit 1
    fi
else
    echo -e "${BLUE}🚀 开始重新部署所有微服务...${NC}"
    SERVICES_TO_DEPLOY=("${SERVICES[@]}")
fi

echo "--------------------------------------------------"

# 第一步：构建 Docker 镜像
echo -e "${YELLOW}📦 第一步：构建 Docker 镜像${NC}"
if [ $# -eq 1 ]; then
    ./build-all.sh "$TARGET_SERVICE"
else
    ./build-all.sh
fi

echo "--------------------------------------------------"

# 第二步：重新部署到 Kubernetes
echo -e "${YELLOW}🚀 第二步：重新部署到 Kubernetes${NC}"

for service in "${SERVICES_TO_DEPLOY[@]}"; do
    echo -e "🔄 正在重新部署服务: ${BLUE}${service}${NC}"
    
    # 使用 rollout restart 重新部署（推荐方式）
    kubectl rollout restart deployment/"${service}" -n jaeger-demo
    
    echo -e "✅ 服务 ${BLUE}${service}${NC} 重新部署命令已发送"
done

echo "--------------------------------------------------"

# 第三步：等待部署完成
echo -e "${YELLOW}⏳ 第三步：等待部署完成${NC}"

for service in "${SERVICES_TO_DEPLOY[@]}"; do
    echo -e "⏳ 等待服务 ${BLUE}${service}${NC} 部署完成..."
    kubectl rollout status deployment/"${service}" -n jaeger-demo
    echo -e "✅ 服务 ${BLUE}${service}${NC} 部署完成！"
done

echo "--------------------------------------------------"

echo -e "${GREEN}🎉 所有目标服务重新部署完成！${NC}"
echo
echo -e "${BLUE}当前 Pod 状态:${NC}"
kubectl get pods -n jaeger-demo

echo
echo -e "${BLUE}服务状态:${NC}"
kubectl get services -n jaeger-demo 