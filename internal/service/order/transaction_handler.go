// internal/busi/order/transaction_handler.go

package order

import (
	"fmt"
	"log"
)

// TransactionHandler 负责管理整个责任链的事务生命周期
type TransactionHandler struct {
	NextHandler
}

func (h *TransactionHandler) Handle(orderCtx *OrderContext) (err error) {
	// 1. 开始事务，使用 defer 来确保无论过程如何，最终都会进行处理
	log.Println("【事务处理器】=> 开启订单处理事务...")

	defer func() {
		if r := recover(); r != nil {
			// 处理 panic，以防万一
			err = fmt.Errorf("panic recovered: %v", r)
		}

		if err != nil {
			// 2. 如果链中任何地方返回了错误，执行回滚
			log.Println("【事务处理器】=> 检测到错误，开始执行回滚...")
			for _, comp := range orderCtx.compensations {
				// 执行所有已注册的补偿函数
				comp()
			}
			log.Println("【事务处理器】=> 回滚完成。")
		} else {
			// 3. 如果一切顺利，事务“提交”（在我们的场景里，可以只是简单记录日志）
			log.Println("【事务处理器】=> 流程成功，事务提交。")
		}
	}()

	// 4. 执行责任链的下一个环节
	return h.executeNext(orderCtx)
}
