package adapter

import (
	"context"
	"github.com/wangyingjie930/nexus-pkg/constants"
	"github.com/wangyingjie930/nexus-pkg/httpclient"
	"net/url"
	"nexus/internal/service/order/domain"
	"strings"
)

// FraudHTTPAdapter 是 port.FraudDetectionService 接口的HTTP实现。
type FraudHTTPAdapter struct {
	client *httpclient.Client
}

// NewFraudHTTPAdapter 创建一个新的欺诈检测服务适配器实例。
func NewFraudHTTPAdapter(client *httpclient.Client) *FraudHTTPAdapter {
	return &FraudHTTPAdapter{client: client}
}

// CheckFraud 实现了调用外部欺诈检测服务的逻辑。
func (a *FraudHTTPAdapter) CheckFraud(ctx context.Context, order *domain.Order) error {
	params := url.Values{}
	params.Set("items", strings.Join(order.Items, ","))
	params.Set("userId", order.UserID)

	// 调用通用的 HTTP 客户端，并指定目标服务和路径
	// 所有的服务发现、追踪、错误处理都在 httpclient.Client 中完成
	return a.client.CallService(ctx, constants.FraudDetectionService, constants.FraudCheckPath, params)
}
