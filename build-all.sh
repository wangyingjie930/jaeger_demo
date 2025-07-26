#!/bin/bash

# build-all.sh
# 自动为 nexus 项目中的微服务构建Docker镜像。
# 用法: ./build-all.sh [服务名]
# 如果不传参数，则构建所有服务；如果传入服务名，则只构建该服务

# -e: 如果任何命令失败（返回非零退出状态），脚本将立即退出。
# -o pipefail: 如果管道中的任何命令失败，则整个管道的退出状态为失败。
set -eo pipefail

# 颜色定义，用于美化输出
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 参照你的 start-services.sh 文件定义的服务列表
# 我们只需要服务名部分
SERVICES=(
#    "api-gateway"
#    "order-service"
#    "promotion-service"
    "inventory-service"
    "notification-service"
    "pricing-service"
    "fraud-detection-service"
    "shipping-service"
    "order-service-v2"
    "delay-scheduler"
)

# 检查是否传入了服务名参数
if [ $# -eq 1 ]; then
    TARGET_SERVICE="$1"
    
    # 验证传入的服务名是否在服务列表中
    if [[ " ${SERVICES[@]} " =~ " ${TARGET_SERVICE} " ]]; then
        echo -e "${BLUE}🚀 开始构建指定服务: ${YELLOW}${TARGET_SERVICE}${NC}"
        echo "--------------------------------------------------"
        
        SERVICES_TO_BUILD=("$TARGET_SERVICE")
    else
        echo -e "${RED}❌ 错误: 服务 '${TARGET_SERVICE}' 不在支持的服务列表中${NC}"
        echo -e "${YELLOW}支持的服务列表:${NC}"
        printf '%s\n' "${SERVICES[@]}"
        exit 1
    fi
else
    echo -e "${BLUE}🚀 开始为所有微服务构建Docker镜像...${NC}"
    echo "--------------------------------------------------"
    
    SERVICES_TO_BUILD=("${SERVICES[@]}")
fi

ACR_REGISTRY="crpi-4dj6hqy7jwojfw8v.cn-shanghai.personal.cr.aliyuncs.com/yingjiewang"

# 循环遍历服务列表
for service in "${SERVICES_TO_BUILD[@]}"; do
    IMAGE_TAG="nexus/${service}:latest"
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

docker pull yingjiewang/nexus-order:latest
docker pull yingjiewang/nexus-promotion:latest


echo -e "${GREEN}🎉 所有目标服务的镜像均已成功构建！${NC}"
echo
echo -e "${BLUE}以下是本次构建的镜像列表:${NC}"
docker images | grep "nexus"