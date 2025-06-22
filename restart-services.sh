#!/bin/bash

# Jaeger Demo 微服务重启脚本
# 作者: AI Assistant
# 描述: 重启所有微服务

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔄 重启 Jaeger Demo 微服务...${NC}"

# 停止服务
echo -e "${BLUE}🛑 停止现有服务...${NC}"
./stop-services.sh

# 等待一下确保服务完全停止
sleep 3

# 启动服务
echo -e "${BLUE}🚀 启动服务...${NC}"
./start-services.sh

echo -e "${GREEN}🎉 服务重启完成！${NC}" 