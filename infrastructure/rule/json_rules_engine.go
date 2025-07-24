// promotion-service/internal/infrastructure/rule/json_rules_engine.go
package rule

import (
	"encoding/json"
	"fmt"
	"nexus/internal/service/promotion/domain"
	"reflect"
	"strings"
)

// Condition represents a single rule condition (e.g., "user.isVip", "equal", true).
type Condition struct {
	Fact     string      `json:"fact"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// RuleGroup represents a group of conditions combined with "all" or "any".
type RuleGroup struct {
	All []json.RawMessage `json:"all"`
	Any []json.RawMessage `json:"any"`
}

// JSONRuleEngineAdapter 是 domain.RuleEngine 接口的一个新的、自包含的实现。
// 它不依赖任何外部库，直接解析和评估规则。
type JSONRuleEngineAdapter struct{}

// NewJSONRuleEngineAdapter 创建一个新的规则引擎适配器实例。
func NewJSONRuleEngineAdapter() *JSONRuleEngineAdapter {
	return &JSONRuleEngineAdapter{}
}

// Evaluate 实现了 domain.RuleEngine 接口，用于执行规则评估。
func (a *JSONRuleEngineAdapter) Evaluate(ruleDefinition string, fact domain.Fact) (bool, error) {
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(ruleDefinition), &raw); err != nil {
		return false, fmt.Errorf("failed to unmarshal rule definition: %w", err)
	}

	factMap, err := structToMap(fact)
	if err != nil {
		return false, fmt.Errorf("failed to convert fact to map: %w", err)
	}

	return a.evaluateNode(raw, factMap)
}

// evaluateNode 递归地评估一个JSON节点（可以是条件组或单个条件）。
func (a *JSONRuleEngineAdapter) evaluateNode(node json.RawMessage, factMap map[string]interface{}) (bool, error) {
	// 尝试解析为条件组 (all/any)
	var group RuleGroup
	if json.Unmarshal(node, &group) == nil {
		if len(group.All) > 0 {
			for _, subNode := range group.All {
				match, err := a.evaluateNode(subNode, factMap)
				if err != nil {
					return false, err
				}
				if !match {
					return false, nil // "all" 逻辑，一旦有一个不匹配，整个组就不匹配
				}
			}
			return true, nil // 所有都匹配
		}
		if len(group.Any) > 0 {
			for _, subNode := range group.Any {
				match, err := a.evaluateNode(subNode, factMap)
				if err != nil {
					return false, err
				}
				if match {
					return true, nil // "any" 逻辑，一旦有一个匹配，整个组就匹配
				}
			}
			return false, nil // 没有任何一个匹配
		}
	}

	// 尝试解析为单个条件
	var condition Condition
	if json.Unmarshal(node, &condition) == nil {
		return a.evaluateCondition(&condition, factMap)
	}

	return false, fmt.Errorf("invalid rule structure: %s", string(node))
}

// evaluateCondition 评估单个条件。
func (a *JSONRuleEngineAdapter) evaluateCondition(c *Condition, factMap map[string]interface{}) (bool, error) {
	factValue, err := getFactValue(c.Fact, factMap)
	if err != nil {
		return false, err
	}

	// 为了简化比较，我们将所有数字类型转换为 float64
	factValueFloat, factIsNumber := toFloat64(factValue)
	condValueFloat, condIsNumber := toFloat64(c.Value)

	// 核心比较逻辑
	switch c.Operator {
	case "equal":
		if factIsNumber && condIsNumber {
			return factValueFloat == condValueFloat, nil
		}
		return reflect.DeepEqual(factValue, c.Value), nil
	case "notEqual":
		if factIsNumber && condIsNumber {
			return factValueFloat != condValueFloat, nil
		}
		return !reflect.DeepEqual(factValue, c.Value), nil
	case "greaterThan":
		if !factIsNumber || !condIsNumber {
			return false, fmt.Errorf("operator '%s' requires numeric values for fact '%s'", c.Operator, c.Fact)
		}
		return factValueFloat > condValueFloat, nil
	case "lessThan":
		if !factIsNumber || !condIsNumber {
			return false, fmt.Errorf("operator '%s' requires numeric values for fact '%s'", c.Operator, c.Fact)
		}
		return factValueFloat < condValueFloat, nil
	case "greaterThanInclusive":
		if !factIsNumber || !condIsNumber {
			return false, fmt.Errorf("operator '%s' requires numeric values for fact '%s'", c.Operator, c.Fact)
		}
		return factValueFloat >= condValueFloat, nil
	case "lessThanInclusive":
		if !factIsNumber || !condIsNumber {
			return false, fmt.Errorf("operator '%s' requires numeric values for fact '%s'", c.Operator, c.Fact)
		}
		return factValueFloat <= condValueFloat, nil
	// 您可以在这里轻松扩展更多操作符，例如 "in", "notIn", "contains" 等
	default:
		return false, fmt.Errorf("unsupported operator: %s", c.Operator)
	}
}

// getFactValue 通过点分路径 (e.g., "User.IsVip") 从 map 中获取值。
func getFactValue(path string, data map[string]interface{}) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := interface{}(data)

	for _, part := range parts {
		// Go 的 JSON unmarshal 会将 struct 字段名转为大写开头
		// 我们需要在这里做一个适配
		formattedPart := strings.ToUpper(part[:1]) + part[1:]

		val, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid path at '%s'", part)
		}
		current, ok = val[formattedPart]
		if !ok {
			return nil, fmt.Errorf("fact not found: %s", path)
		}
	}
	return current, nil
}

// structToMap 将一个 struct 转换为 map[string]interface{} 以便进行动态访问。
func structToMap(s interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}

// toFloat64 是一个辅助函数，尝试将 interface{} 转换为 float64。
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int8:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case float32:
		return float64(val), true
	case float64:
		return val, true
	case json.Number:
		f, err := val.Float64()
		if err == nil {
			return f, true
		}
	}
	return 0, false
}
