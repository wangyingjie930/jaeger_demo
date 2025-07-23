// promotion-service/internal/infrastructure/rule/json_rules_engine.go
package rule

import (
	"encoding/json"
	"nexus/internal/service/promotion/domain"
)

// JSONRuleEngineAdapter 是 domain.RuleEngine 接口的一个具体实现。
// 它使用了一个第三方的 json-rules-engine 库来执行规则评估。
// 这是一个典型的适配器模式应用，它将第三方库的API适配到我们自己的领域接口。
type JSONRuleEngineAdapter struct{}

// NewJSONRuleEngineAdapter 创建一个新的规则引擎适配器实例。
func NewJSONRuleEngineAdapter() *JSONRuleEngineAdapter {
	return &JSONRuleEngineAdapter{}
}

// Evaluate 实现了 domain.RuleEngine 接口。
func (a *JSONRuleEngineAdapter) Evaluate(ruleDefinition string, fact domain.Fact) (bool, error) {
	// 1. 将我们的领域对象 "Fact" 序列化为 JSON。
	//    因为大多数规则引擎都是基于JSON或map[string]interface{}来工作的。
	factData, err := json.Marshal(fact)
	if err != nil {
		return false, err
	}

	var factMap map[string]interface{}
	err = json.Unmarshal(factData, &factMap)
	if err != nil {
		return false, err
	}

	// 2. 创建第三方引擎的实例，并加载规则定义。
	engine, err := jre.New(ruleDefinition)
	if err != nil {
		return false, err // 规则定义本身可能存在语法错误
	}

	// 3. 执行评估。
	result := engine.Evaluate(factMap)

	// 4. 返回结果。
	return result.IsSuccess(), nil
}
