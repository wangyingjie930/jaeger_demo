# Jaeger Demo 微服务管理脚本

这个项目包含了一套完整的shell脚本来管理Jaeger Demo微服务。

## 📁 脚本文件

- `start-services.sh` - 启动所有微服务
- `stop-services.sh` - 停止所有微服务  
- `restart-services.sh` - 重启所有微服务
- `status-services.sh` - 查看服务状态

## 🚀 快速开始

### 1. 启动所有服务
```bash
./start-services.sh
```

这将启动以下服务：
- **Jaeger** (如果已安装): http://localhost:16686
- **API Gateway**: http://localhost:8080
- **Order Service**: http://localhost:8081
- **Shipping Service**: http://localhost:8082
- **Fraud Detection Service**: http://localhost:8083
- **Pricing Service**: http://localhost:8084
- **Notification Service**: http://localhost:8085
- **Inventory Service**: http://localhost:8086

### 2. 查看服务状态
```bash
./status-services.sh
```

### 3. 停止所有服务
```bash
./stop-services.sh
```

### 4. 重启所有服务
```bash
./restart-services.sh
```

## 📋 功能特性

### 启动脚本 (`start-services.sh`)
- ✅ 自动检测并启动Jaeger (如果已安装)
- ✅ 按顺序启动所有微服务
- ✅ 自动生成日志文件到 `./logs/` 目录
- ✅ 保存进程PID到 `./services.pid` 文件
- ✅ 彩色输出和状态提示
- ✅ 错误处理和目录检查

### 停止脚本 (`stop-services.sh`)
- ✅ 优雅停止所有进程
- ✅ 强制停止顽固进程
- ✅ 清理PID文件
- ✅ 可选的日志文件清理

### 状态脚本 (`status-services.sh`)
- ✅ 显示所有运行中的进程
- ✅ 检查服务端口可用性
- ✅ 显示日志文件信息
- ✅ 提供使用说明

## 📁 目录结构

```
jaeger-demo/
├── start-services.sh      # 启动脚本
├── stop-services.sh       # 停止脚本
├── restart-services.sh    # 重启脚本
├── status-services.sh     # 状态检查脚本
├── services.pid          # 进程PID文件 (运行时生成)
├── logs/                 # 日志目录 (运行时生成)
│   ├── api-gateway.log
│   ├── order-service.log
│   ├── shipping-service.log
│   └── ...
└── cmd/                  # 服务源码目录
    ├── api-gateway/
    ├── order-service/
    ├── shipping-service/
    └── ...
```

## 🔧 前置要求

1. **Go环境**: 确保已安装Go 1.24.0或更高版本
2. **Jaeger** (可选): 用于分布式追踪
   ```bash
   # 使用Docker启动Jaeger
   docker run -d --name jaeger -p 16686:16686 -p 14268:14268 jaegertracing/all-in-one:latest
   
   # 或安装Jaeger二进制文件
   # 参考: https://www.jaegertracing.io/docs/getting-started/
   ```

## 🐛 故障排除

### 服务启动失败
1. 检查Go环境是否正确安装
2. 确保所有依赖已安装: `go mod tidy`
3. 检查端口是否被占用: `lsof -i :8080`

### 服务无法访问
1. 使用 `./status-services.sh` 检查服务状态
2. 查看日志文件: `tail -f logs/service-name.log`
3. 检查防火墙设置

### 停止服务失败
1. 手动查找进程: `ps aux | grep go`
2. 强制杀死进程: `kill -9 <PID>`
3. 删除PID文件: `rm services.pid`

## 📝 日志管理

- 所有服务日志保存在 `./logs/` 目录
- 每个服务有独立的日志文件
- 停止服务时可选择删除日志文件

## 🔄 自动化

可以将这些脚本集成到CI/CD流程中：

```bash
# 在CI中启动服务进行测试
./start-services.sh
sleep 10  # 等待服务启动
# 运行测试
./stop-services.sh
```

## 📞 支持

如果遇到问题，请检查：
1. 脚本执行权限: `chmod +x *.sh`
2. 服务目录结构是否正确
3. 端口是否被其他程序占用
4. Go模块依赖是否完整 