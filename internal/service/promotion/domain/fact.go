// promotion-service/internal/domain/fact.go
package domain

import "time"

// CartItem 代表购物车中的一个商品项
type CartItem struct {
	SKU      string
	Price    int64 // 商品单价（单位：分）
	Quantity int32
	Category string // 商品品类
	Brand    string // 商品品牌
}

// UserContext 代表当前的用户信息
type UserContext struct {
	ID     int64
	IsVip  bool     // 是否为VIP用户
	Labels []string // 用户标签，如 "new_user", "high_value"
}

// EnvironmentContext 代表环境信息
type EnvironmentContext struct {
	Timestamp time.Time // 当前时间
	Channel   string    // 渠道, e.g., "app", "mini_program"
}

// Fact 是规则引擎和优惠计算策略所需的所有上下文信息的集合。
// 它是一个高度结构化的数据对象，作为评估过程的唯一输入。
// 这种设计将计算逻辑与数据来源完全解耦，极大地提高了系统的可测试性和可扩展性。
type Fact struct {
	User        UserContext
	Items       []CartItem
	Environment EnvironmentContext

	// 派生字段，在服务层预先计算，以简化规则逻辑
	TotalAmount int64 // 购物车总金额
}
