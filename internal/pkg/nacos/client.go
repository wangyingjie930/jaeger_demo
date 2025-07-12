// internal/pkg/nacos/client.go
package nacos

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// Client 封装了 Nacos 命名客户端
type Client struct {
	namingClient naming_client.INamingClient

	namespaceId string // ✨ 新增: 存储命名空间ID
	groupName   string // ✨ 新增: 存储默认分组名
}

// NewNacosClient 创建并返回一个新的 Nacos 客户端
// addrs 格式为 "ip1:port1,ip2:port2"
func NewNacosClient(addrs string, namespaceId, groupName string) (*Client, error) {
	if namespaceId == "" {
		log.Println("⚠️ WARNING: NACOS_NAMESPACE is not set. Using default public namespace.")
	}
	if groupName == "" {
		groupName = "DEFAULT_GROUP" // Nacos 默认分组
		log.Printf("⚠️ WARNING: NACOS_GROUP is not set. Using '%s'.", groupName)
	}

	var serverConfigs []constant.ServerConfig
	for _, addr := range strings.Split(addrs, ",") {
		parts := strings.Split(addr, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid nacos address format: %s", addr)
		}
		port, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid port in nacos address: %s", parts[1])
		}
		serverConfigs = append(serverConfigs, *constant.NewServerConfig(parts[0], port))
	}

	// 客户端配置
	clientConfig := *constant.NewClientConfig(
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogDir("/tmp/nacos/log"),
		constant.WithCacheDir("/tmp/nacos/cache"),
		constant.WithLogLevel("warn"),
		constant.WithNamespaceId(namespaceId), // ✨ 核心: 在客户端配置中指定命名空间
	)

	// 创建命名服务客户端
	namingClient, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create nacos naming client: %w", err)
	}

	log.Println("✅ Successfully connected to Nacos.")
	return &Client{
		namingClient: namingClient,
		namespaceId:  namespaceId,
		groupName:    groupName,
	}, nil
}

// RegisterServiceInstance 注册一个服务实例到 Nacos
func (c *Client) RegisterServiceInstance(serviceName, ip string, port int) error {
	success, err := c.namingClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        uint64(port),
		ServiceName: serviceName,
		Weight:      10,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,        // 设置为临时节点，心跳断开后会自动摘除
		GroupName:   c.groupName, // ✨ 核心: 注册时使用客户端配置的分组
	})
	if err != nil {
		return fmt.Errorf("failed to register service with nacos: %w", err)
	}
	if !success {
		return fmt.Errorf("nacos registration was not successful for service: %s", serviceName)
	}
	log.Printf("✅ Service '%s' registered to Nacos successfully (%s:%d)", serviceName, ip, port)
	return nil
}

// DeregisterServiceInstance 从 Nacos 注销一个服务实例
func (c *Client) DeregisterServiceInstance(serviceName, ip string, port int) error {
	_, err := c.namingClient.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          ip,
		Port:        uint64(port),
		ServiceName: serviceName,
		Ephemeral:   true,
		GroupName:   c.groupName, // ✨ 核心: 注销时使用客户端配置的分组
	})
	if err != nil {
		return fmt.Errorf("failed to deregister service with nacos: %w", err)
	}
	log.Printf("ℹ️ Service '%s' deregistered from Nacos (%s:%d)", serviceName, ip, port)
	return nil
}

// DiscoverServiceInstance 从 Nacos 发现一个健康的服务实例
// 使用 Nacos 内置的负载均衡算法
func (c *Client) DiscoverServiceInstance(serviceName string) (string, int, error) {
	instance, err := c.namingClient.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
		ServiceName: serviceName,
		GroupName:   c.groupName, // ✨ 核心: 服务发现时指定分组
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to discover healthy instance for service '%s': %w", serviceName, err)
	}
	if instance == nil {
		return "", 0, fmt.Errorf("no healthy instance available for service '%s'", serviceName)
	}
	return instance.Ip, int(instance.Port), nil
}

// Close 关闭 Nacos 客户端连接
func (c *Client) Close() {
	if c.namingClient != nil {
		// Nacos Go SDK v2.x.x 没有显式的 Close 方法
		// 临时节点会在心跳停止后自动过期
		log.Println("ℹ️ Nacos client does not require explicit closing. Ephemeral nodes will expire.")
	}
}
