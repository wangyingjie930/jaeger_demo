#!/bin/bash

# 初始化 Redis 集群脚本
# 这个脚本将两个 Redis 节点连接成一个集群并分配哈希槽

set -e

echo "🔧 开始初始化 Redis 集群..."

# 获取 Redis 节点的 IP 地址
REDIS_0_IP=$(kubectl get pod redis-cluster-0 -n infra -o jsonpath='{.status.podIP}')
REDIS_1_IP=$(kubectl get pod redis-cluster-1 -n infra -o jsonpath='{.status.podIP}')
REDIS_2_IP=$(kubectl get pod redis-cluster-2 -n infra -o jsonpath='{.status.podIP}')

echo "📡 Redis 节点 IP 地址:"
echo "  - redis-cluster-0: $REDIS_0_IP"
echo "  - redis-cluster-1: $REDIS_1_IP"
echo "  - redis-cluster-2: $REDIS_2_IP"

# 等待节点启动
echo "⏳ 等待 Redis 节点启动..."
sleep 5

# 在第一个节点上执行集群初始化
echo "🔗 在 redis-cluster-0 上初始化集群..."
kubectl exec -n infra redis-cluster-0 -- redis-cli --cluster create \
  $REDIS_0_IP:6379 $REDIS_1_IP:6379 $REDIS_2_IP:6379 \
  --cluster-replicas 0 \
  --cluster-yes

echo "✅ Redis 集群初始化完成！"

# 验证集群状态
echo "🔍 验证集群状态..."
kubectl exec -n infra redis-cluster-0 -- redis-cli cluster info

echo "📋 集群节点信息:"
kubectl exec -n infra redis-cluster-0 -- redis-cli cluster nodes

echo "🎉 Redis 集群初始化成功！" 