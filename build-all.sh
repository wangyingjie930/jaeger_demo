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

docker pull yingjiewang/nexus-order:latest
docker pull yingjiewang/nexus-promotion:latest
docker pull yingjiewang/nexus-inventory:latest
docker pull yingjiewang/nexus-notification:latest
docker pull yingjiewang/nexus-pricing:latest
docker pull yingjiewang/nexus-fraud-detection:latest
docker pull yingjiewang/nexus-shipping:latest
docker pull yingjiewang/nexus-order-v2:latest
docker pull yingjiewang/nexus-delay-scheduler:latest


echo -e "${GREEN}🎉 所有目标服务的镜像均已成功构建！${NC}"
echo
echo -e "${BLUE}以下是本次构建的镜像列表:${NC}"
docker images | grep "nexus"