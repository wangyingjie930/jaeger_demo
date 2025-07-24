// promotion-service/internal/domain/rule.go
package domain

// RuleEngine 代表一个规则评估引擎的接口。
// 它的职责是：根据给定的规则定义（LHS）和事实（Fact），判断条件是否满足。
type RuleEngine interface {
	// Evaluate 执行规则评估
	// ruleDefinition: 规则的文本表示，例如一个JSON字符串
	// fact: 包含所有上下文信息的事实对象
	// 返回值: bool 代表是否匹配，error 代表评估过程中是否出错
	Evaluate(ruleDefinition string, fact Fact) (bool, error)
}

// PromotionRule 代表一个完整的促销规则。
// 它封装了规则的定义，并利用RuleEngine来执行评估。
type PromotionRule struct {
	// 规则的JSON定义，描述了优惠适用的所有条件 (LHS)。
	// 例如：'{"all": [{"fact": "user.isVip", "operator": "equal", "value": true}]}'
	Definition string
}

// IsSatisfied by a given fact.
func (r *PromotionRule) IsSatisfied(engine RuleEngine, fact Fact) (bool, error) {
	// 如果规则定义为空，我们认为它无条件满足。
	// 这对于那些没有复杂前置条件的通用优惠（如“无门槛5元券”）非常有用。
	if r.Definition == "" {
		return true, nil
	}
	return engine.Evaluate(r.Definition, fact)
}
