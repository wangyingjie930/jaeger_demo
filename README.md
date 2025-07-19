# [WIP] Nexus 微服务架构演示项目

[![Go Version](https://img.shields.io/badge/Go-1.24.0+-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](https://www.docker.com/)

一个基于Go语言构建的完整微服务架构演示项目，展示了现代微服务的最佳实践，包括分布式追踪、消息队列、服务发现、监控等核心特性。

## 🏗️ 项目架构

### 微服务组件

| 服务名称                     | 端口   | 描述 | 主要功能 |
|--------------------------|------|------|----------|
| **Apisix**               | -    | 统一入口网关 | 路由转发、负载均衡、认证授权 |
| **Order Service**        | 8081 | 订单服务 | 订单创建、状态管理、业务流程编排 |
| **Inventory Service**    | 8082 | 库存服务 | 库存管理、库存预留、库存释放 |
| **Pricing Service**      | 8084 | 定价服务 | 价格计算、折扣策略 |
| **Fraud Detection**      | 8085 | 风控服务 | 欺诈检测、风险评估 |
| **Shipping Service**     | 8086 | 物流服务 | 运费计算、物流跟踪 |
| **Promotion Service**    | 8087 | 促销服务 | 优惠券、促销活动 |
| **Notification Service** | -    | 通知服务 | 消息推送、邮件通知 |
| **Message Router**       | -    | 消息路由 | 消息分发、事件处理 |
| **Delay Scheduler**      | -    | 延迟调度 | 定时任务、延迟处理 |

### 基础设施

- **Jaeger**: 分布式追踪系统 (端口: 16686)
- **Kafka**: 消息队列 (端口: 9092)
- **Zookeeper**: 服务协调 (端口: 2181)
- **Prometheus**: 监控指标收集
- **Push Gateway**: 指标推送网关

## 🚀 快速开始

### 前置要求

- **Go 1.24.0+** - [下载安装](https://golang.org/dl/)
- **Kubernetes** - 用于K8s部署
  - apisix
  - istio
- **telepresence** - 本地连接k8s

### 本地开发环境

1. **克隆项目**
```bash
git clone <repository-url>
cd nexus
```

2. **安装依赖**
```bash
go mod tidy
```

3. **启动k8s**
```bash
telepresence connect
```

4. **启动所有服务**
```bash
# 使用脚本启动
./start-services.sh
```


## Kubernetes 部署

1. **创建命名空间**
```bash
kubectl apply -f k8s/00-namespace.yaml
kubectl apply -f k8s/00-infra-namespace.yaml
```

2. **部署基础设施**
```bash
kubectl apply -k k8s/01-infra/
```

3. **部署配置**
```bash
kubectl apply -k k8s/02-configs/
```

4. **部署微服务**
```bash
kubectl apply -k k8s/03-services/
```

5. **部署监控**
```bash
kubectl apply -k k8s/04-monitoring/
```

6. **部署入口**
```bash
kubectl apply -k k8s/05-ingress/
```

## 🧪 测试

### 性能测试

使用 k6 进行性能测试：

```bash
# 安装 k6
# macOS: brew install k6
# Linux: https://k6.io/docs/getting-started/installation/

# 运行性能测试
k6 run k6-script.js

# 指定API地址
k6 run -e API_BASE_URL=http://localhost:8081 k6-script.js
```

### API 测试

```bash
# 创建订单
curl -X POST "http://localhost:8081/create_complex_order?userID=user123&is_vip=true&items=item-a,item-b"

# 查询订单状态
curl "http://localhost:8081/order_status?orderID=order123"

# 获取库存信息
curl "http://localhost:8081/inventory/check?itemID=item-a"
```

## 🔧 开发指南

### 项目结构

```
nexus/
├── cmd/                    # 微服务入口
│   ├── api-gateway/       # API网关
│   ├── order-service/     # 订单服务
│   ├── inventory-service/ # 库存服务
│   └── ...
├── internal/              # 内部包
│   ├── config/           # 配置管理
│   ├── middleware/       # 中间件
│   ├── models/           # 数据模型
│   └── utils/            # 工具函数
├── conf/                 # 配置文件
├── scripts/              # 管理脚本
├── k8s/                  # Kubernetes配置
├── deploy/               # 部署配置
├── db/                   # 数据库脚本
└── logs/                 # 日志文件
```


### 构建和部署

```bash
# 构建所有服务
./build-all.sh
```

## 📈 监控和可观测性

### 分布式追踪

- **Jaeger**: 查看请求链路和性能分析
- **OpenTelemetry**: 标准化的遥测数据收集

### 指标监控

- **Prometheus**: 指标收集和存储
- **Grafana**: 监控仪表板
- **自定义指标**: 业务指标监控

### 日志管理

- **结构化日志**: 使用 zerolog 进行结构化日志记录
- **日志聚合**: 支持 ELK 或类似日志聚合系统

## 🔒 安全特性

- **服务间认证**: mTLS 证书认证
- **API 认证**: JWT Token 认证
- **数据加密**: 敏感数据加密存储
- **访问控制**: RBAC 权限控制

## 🚀 生产部署

### 环境配置

```bash
# 生产环境变量
export ENV=production
export LOG_LEVEL=info
export METRICS_ENABLED=true
export TRACING_ENABLED=true
```

### 高可用配置

- **负载均衡**: 使用 Nginx 或 HAProxy
- **服务发现**: 集成 Consul 或 etcd
- **配置管理**: 使用 Nacos 或 ConfigMap
- **健康检查**: 完善的健康检查机制

## 🤝 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 📞 支持

- **问题反馈**: [GitHub Issues](https://github.com/your-repo/nexus/issues)
- **文档**: [项目 Wiki](https://github.com/your-repo/nexus/wiki)
- **讨论**: [GitHub Discussions](https://github.com/your-repo/nexus/discussions)

## 🙏 致谢

感谢以下开源项目的支持：

- [Go](https://golang.org/) - 编程语言
- [Jaeger](https://www.jaegertracing.io/) - 分布式追踪
- [Kafka](https://kafka.apache.org/) - 消息队列
- [Prometheus](https://prometheus.io/) - 监控系统
- [Kubernetes](https://kubernetes.io/) - 容器编排

---

⭐ 如果这个项目对您有帮助，请给我们一个星标！ 