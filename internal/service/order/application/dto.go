// internal/service/order/application/dto.go
package application

import "nexus/internal/service/order/domain"

// CreateOrderRequest 是创建订单用例的输入数据
type CreateOrderRequest struct {
	TraceID          string
	UserID           string
	IsVIP            bool
	Items            []string
	Quantity         int
	PromoID          string
	SeckillProductID string
	EventID          string
}

// CreateOrderResponse 是创建订单用例的输出数据
type CreateOrderResponse struct {
	OrderID string
	Status  domain.State
	Message string
}

// ToCreateOrderRequest 从Kafka事件转换为应用层请求DTO
func ToCreateOrderRequest(event *domain.OrderCreationRequested) *CreateOrderRequest {
	return &CreateOrderRequest{
		TraceID:          event.TraceID,
		UserID:           event.UserID,
		IsVIP:            event.IsVIP,
		Items:            event.Items,
		Quantity:         event.Quantity,
		PromoID:          event.PromoID,
		SeckillProductID: event.SeckillProductID,
		EventID:          event.EventID,
	}
}

// ToOrderCreationEvent 从应用层请求DTO转换为领域事件（如果需要）
func (req *CreateOrderRequest) ToOrderCreationEvent() *domain.OrderCreationRequested {
	return &domain.OrderCreationRequested{
		TraceID:          req.TraceID,
		UserID:           req.UserID,
		IsVIP:            req.IsVIP,
		Items:            req.Items,
		Quantity:         req.Quantity,
		PromoID:          req.PromoID,
		SeckillProductID: req.SeckillProductID,
		EventID:          req.EventID,
	}
}
