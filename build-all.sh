#!/bin/bash

# build-all.sh
# 自动为 jaeger-demo 项目中的所有微服务构建Docker镜像。

# -e: 如果任何命令失败（返回非零退出状态），脚本将立即退出。
# -o pipefail: 如果管道中的任何命令失败，则整个管道的退出状态为失败。
set -eo pipefail

# 颜色定义，用于美化输出
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 参照你的 start-services.sh 文件定义的服务列表
# 我们只需要服务名部分
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

echo -e "${BLUE}🚀 开始为所有微服务构建Docker镜像...${NC}"
echo "--------------------------------------------------"

ACR_REGISTRY="crpi-4dj6hqy7jwojfw8v.cn-shanghai.personal.cr.aliyuncs.com/yingjiewang"

# 循环遍历服务列表
for service in "${SERVICES[@]}"; do
    IMAGE_TAG="jaeger-demo/${service}:latest"
    ACR_IMAGE="${ACR_REGISTRY}/${service}:latest"

    echo -e "🔧 正在构建服务: ${BLUE}${service}${NC}，镜像标签为: ${GREEN}${IMAGE_TAG}${NC}"

    # 执行docker build命令
    # 假设Dockerfile在当前项目根目录
    docker build \
        --build-arg SERVICE_NAME="${service}" \
        -t "${IMAGE_TAG}" \
        .

    echo -e "✅ 服务 ${BLUE}${service}${NC} 构建成功！"

    echo "--------------------------------------------------"
done

echo -e "${GREEN}🎉 所有服务的镜像均已成功构建！${NC}"
echo
echo -e "${BLUE}以下是本次构建的镜像列表:${NC}"
docker images | grep "jaeger-demo"