// promotion-service/internal/infrastructure/rule/cel_engine.go
package rule

import (
	"fmt"
	"nexus/internal/service/promotion/domain"
	"sync"

	"github.com/google/cel-go/cel"
)

// CelRuleEngine 是 domain.RuleEngine 接口基于 cel-go 的实现
type CelRuleEngine struct {
	env          *cel.Env
	programCache *sync.Map // 用于缓存已编译的规则程序，提高性能
}

// NewCelRuleEngine 创建并初始化一个新的 CEL 规则引擎
// 这是大厂实践中的标准做法：预先定义好环境和类型，确保类型安全和性能。
func NewCelRuleEngine() (domain.RuleEngine, error) {
	// 1. 创建 CEL 环境，并声明 fact 变量的类型。
	// 这一步是关键，它让 CEL 具备了静态类型检查的能力，能识别 domain.Fact 的所有字段。
	env, err := cel.NewEnv(
		// 必须告诉 CEL `domain.Fact` 这个结构体的存在
		cel.Types(&domain.Fact{}, &domain.UserContext{}, &domain.CartItem{}, &domain.EnvironmentContext{}),
		// 声明一个名为 `fact` 的变量，其类型为我们刚注册的 `domain.Fact`
		cel.Variable("fact", cel.ObjectType("domain.Fact")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cel-go environment: %w", err)
	}

	return &CelRuleEngine{
		env:          env,
		programCache: &sync.Map{},
	}, nil
}

// Evaluate 实现了 domain.RuleEngine 接口
func (e *CelRuleEngine) Evaluate(ruleDefinition string, fact domain.Fact) (bool, error) {
	// 如果规则为空，则直接认为是满足条件，这对于无门槛券等场景很实用。
	if ruleDefinition == "" {
		return true, nil
	}

	var prg cel.Program
	// 2. 检查缓存中是否已有编译好的程序
	if cachedPrg, found := e.programCache.Load(ruleDefinition); found {
		prg = cachedPrg.(cel.Program)
	} else {
		// 3. 如果缓存未命中，则编译规则并存入缓存
		ast, issues := e.env.Compile(ruleDefinition)
		if issues != nil && issues.Err() != nil {
			// 编译时错误，说明规则本身有语法问题
			return false, fmt.Errorf("rule compilation failed: %w", issues.Err())
		}

		// 检查编译后的表达式输出类型是否为 bool
		if !ast.OutputType().IsExactType(cel.BoolType) {
			return false, fmt.Errorf("rule must return a boolean value, but got %s", ast.OutputType())
		}

		var err error
		prg, err = e.env.Program(ast)
		if err != nil {
			return false, fmt.Errorf("program creation failed: %w", err)
		}
		// 存入缓存
		e.programCache.Store(ruleDefinition, prg)
	}

	// 4. 执行评估
	out, _, err := prg.Eval(map[string]interface{}{
		"fact": &fact, // 将 fact 数据传入
	})
	if err != nil {
		// 运行时错误，例如除以零
		return false, fmt.Errorf("rule evaluation failed: %w", err)
	}

	// 5. 返回结果
	// CEL 的布尔值需要类型断言
	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("evaluation result is not a boolean")
	}

	return result, nil
}
