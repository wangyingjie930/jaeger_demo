package application

// UseCouponRequest 是核销优惠券的请求体
type UseCouponRequest struct {
	UserID      string   `json:"user_id"`
	CouponCode  string   `json:"coupon_code"`
	OrderID     string   `json:"order_id"`
	OrderAmount float64  `json:"order_amount"`
	ItemIDs     []string `json:"item_ids"`
}

// UseCouponResponse 是核销优惠券的响应体
type UseCouponResponse struct {
	Success        bool    `json:"success"`
	DiscountAmount float64 `json:"discount_amount"`
	FinalAmount    float64 `json:"final_amount"`
	Message        string  `json:"message"`
}

// GetPromoPriceResponse 是获取促销价的响应体
type GetPromoPriceResponse struct {
	Price float64 `json:"price"`
	Promo string  `json:"promo"`
}
