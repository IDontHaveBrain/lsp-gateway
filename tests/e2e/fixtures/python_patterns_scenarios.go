package fixtures

// PythonPatternScenario represents a test scenario for Python design patterns
type PythonPatternScenario struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	PatternType     string                 `json:"pattern_type"` // creational, structural, behavioral
	FilePath        string                 `json:"file_path"`
	TestPositions   []PythonTestPosition   `json:"test_positions"`
	ExpectedSymbols []PythonPatternSymbol  `json:"expected_symbols"`
	LSPMethods      []string               `json:"lsp_methods"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// PythonTestPosition represents a position in code for LSP testing
type PythonTestPosition struct {
	Line      int    `json:"line"`
	Character int    `json:"character"`
	Symbol    string `json:"symbol"`
	Context   string `json:"context"`
}

// PythonPatternSymbol represents expected symbols with LSP metadata
type PythonPatternSymbol struct {
	Name        string `json:"name"`
	Kind        int    `json:"kind"` // LSP SymbolKind values: Class=5, Method=6, Function=12, Variable=13
	Type        string `json:"type"`
	Description string `json:"description"`
}

// GetCreationalPatternScenarios returns test scenarios for creational design patterns
func GetCreationalPatternScenarios() []PythonPatternScenario {
	return []PythonPatternScenario{
		{
			Name:        "FactoryMethodDefinition",
			Description: "Test go-to-definition on Factory Method pattern classes and methods",
			PatternType: "creational",
			FilePath:    "patterns/creational/factory_method.py",
			TestPositions: []PythonTestPosition{
				{Line: 15, Character: 10, Symbol: "ConcreteCreator", Context: "class definition"},
				{Line: 25, Character: 15, Symbol: "create_product", Context: "method call"},
				{Line: 35, Character: 8, Symbol: "Product", Context: "base class reference"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "Creator", Kind: 5, Type: "class", Description: "Abstract creator base class"},
				{Name: "ConcreteCreator", Kind: 5, Type: "class", Description: "Concrete factory class"},
				{Name: "Product", Kind: 5, Type: "class", Description: "Abstract product interface"},
				{Name: "ConcreteProduct", Kind: 5, Type: "class", Description: "Concrete product implementation"},
				{Name: "create_product", Kind: 6, Type: "method", Description: "Factory method"},
			},
			LSPMethods: []string{"textDocument/definition", "textDocument/references", "textDocument/hover"},
			Metadata: map[string]interface{}{
				"complexity": "medium",
				"inheritance_levels": 2,
				"pattern_category": "object_creation",
			},
		},
		{
			Name:        "BuilderCompletion",
			Description: "Test code completion on Builder pattern method chaining",
			PatternType: "creational",
			FilePath:    "patterns/creational/builder.py",
			TestPositions: []PythonTestPosition{
				{Line: 20, Character: 12, Symbol: "set_name", Context: "method chaining"},
				{Line: 25, Character: 15, Symbol: "set_price", Context: "builder method"},
				{Line: 30, Character: 10, Symbol: "build", Context: "final build call"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "Builder", Kind: 5, Type: "class", Description: "Abstract builder interface"},
				{Name: "ConcreteBuilder", Kind: 5, Type: "class", Description: "Concrete builder implementation"},
				{Name: "Director", Kind: 5, Type: "class", Description: "Director class orchestrating build"},
				{Name: "Product", Kind: 5, Type: "class", Description: "Complex product being built"},
				{Name: "build", Kind: 6, Type: "method", Description: "Builds and returns final product"},
			},
			LSPMethods: []string{"textDocument/completion", "textDocument/hover", "textDocument/documentSymbol"},
			Metadata: map[string]interface{}{
				"complexity": "high",
				"method_chaining": true,
				"fluent_interface": true,
			},
		},
		{
			Name:        "SingletonReferences",
			Description: "Test find references on Singleton pattern instance methods",
			PatternType: "creational",
			FilePath:    "patterns/creational/singleton.py",
			TestPositions: []PythonTestPosition{
				{Line: 12, Character: 8, Symbol: "getInstance", Context: "static method"},
				{Line: 18, Character: 10, Symbol: "_instance", Context: "class variable"},
				{Line: 25, Character: 5, Symbol: "Singleton", Context: "class usage"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "Singleton", Kind: 5, Type: "class", Description: "Singleton class with single instance"},
				{Name: "_instance", Kind: 13, Type: "variable", Description: "Class variable holding single instance"},
				{Name: "getInstance", Kind: 6, Type: "method", Description: "Static method returning singleton instance"},
				{Name: "__new__", Kind: 6, Type: "method", Description: "Constructor ensuring single instance"},
			},
			LSPMethods: []string{"textDocument/references", "workspace/symbol", "textDocument/definition"},
			Metadata: map[string]interface{}{
				"complexity": "low",
				"thread_safety": false,
				"instantiation_control": true,
			},
		},
		{
			Name:        "AbstractFactorySymbols",
			Description: "Test document symbols on Abstract Factory pattern hierarchy",
			PatternType: "creational",
			FilePath:    "patterns/creational/abstract_factory.py",
			TestPositions: []PythonTestPosition{
				{Line: 10, Character: 6, Symbol: "AbstractFactory", Context: "abstract base class"},
				{Line: 20, Character: 8, Symbol: "ConcreteFactory1", Context: "concrete factory"},
				{Line: 35, Character: 12, Symbol: "create_product_a", Context: "factory method"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "AbstractFactory", Kind: 5, Type: "class", Description: "Abstract factory interface"},
				{Name: "ConcreteFactory1", Kind: 5, Type: "class", Description: "First concrete factory"},
				{Name: "ConcreteFactory2", Kind: 5, Type: "class", Description: "Second concrete factory"},
				{Name: "AbstractProductA", Kind: 5, Type: "class", Description: "Abstract product A interface"},
				{Name: "AbstractProductB", Kind: 5, Type: "class", Description: "Abstract product B interface"},
				{Name: "create_product_a", Kind: 6, Type: "method", Description: "Creates product A"},
				{Name: "create_product_b", Kind: 6, Type: "method", Description: "Creates product B"},
			},
			LSPMethods: []string{"textDocument/documentSymbol", "workspace/symbol", "textDocument/hover"},
			Metadata: map[string]interface{}{
				"complexity": "high",
				"product_families": 2,
				"factory_variants": 2,
			},
		},
	}
}

// GetStructuralPatternScenarios returns test scenarios for structural design patterns
func GetStructuralPatternScenarios() []PythonPatternScenario {
	return []PythonPatternScenario{
		{
			Name:        "AdapterDefinition",
			Description: "Test go-to-definition on Adapter pattern interface adaptation",
			PatternType: "structural",
			FilePath:    "patterns/structural/adapter.py",
			TestPositions: []PythonTestPosition{
				{Line: 15, Character: 6, Symbol: "Adapter", Context: "adapter class"},
				{Line: 22, Character: 12, Symbol: "adaptee", Context: "wrapped object"},
				{Line: 28, Character: 8, Symbol: "request", Context: "adapted method"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "Target", Kind: 5, Type: "class", Description: "Target interface expected by client"},
				{Name: "Adaptee", Kind: 5, Type: "class", Description: "Existing class with incompatible interface"},
				{Name: "Adapter", Kind: 5, Type: "class", Description: "Adapter making adaptee compatible with target"},
				{Name: "request", Kind: 6, Type: "method", Description: "Target interface method"},
				{Name: "specific_request", Kind: 6, Type: "method", Description: "Adaptee specific method"},
			},
			LSPMethods: []string{"textDocument/definition", "textDocument/references", "textDocument/hover"},
			Metadata: map[string]interface{}{
				"complexity": "medium",
				"interface_adaptation": true,
				"composition_pattern": true,
			},
		},
		{
			Name:        "DecoratorChaining",
			Description: "Test completion on Decorator pattern method chaining",
			PatternType: "structural",
			FilePath:    "patterns/structural/decorator.py",
			TestPositions: []PythonTestPosition{
				{Line: 18, Character: 10, Symbol: "ConcreteDecorator", Context: "decorator class"},
				{Line: 25, Character: 8, Symbol: "operation", Context: "decorated method"},
				{Line: 32, Character: 15, Symbol: "component", Context: "wrapped component"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "Component", Kind: 5, Type: "class", Description: "Abstract component interface"},
				{Name: "ConcreteComponent", Kind: 5, Type: "class", Description: "Concrete component implementation"},
				{Name: "Decorator", Kind: 5, Type: "class", Description: "Base decorator class"},
				{Name: "ConcreteDecorator", Kind: 5, Type: "class", Description: "Concrete decorator adding behavior"},
				{Name: "operation", Kind: 6, Type: "method", Description: "Component operation being decorated"},
			},
			LSPMethods: []string{"textDocument/completion", "textDocument/documentSymbol", "workspace/symbol"},
			Metadata: map[string]interface{}{
				"complexity": "medium",
				"behavior_extension": true,
				"recursive_composition": true,
			},
		},
		{
			Name:        "FacadeSimplification",
			Description: "Test hover information on Facade pattern subsystem interactions",
			PatternType: "structural",
			FilePath:    "patterns/structural/facade.py",
			TestPositions: []PythonTestPosition{
				{Line: 12, Character: 6, Symbol: "Facade", Context: "facade class"},
				{Line: 20, Character: 10, Symbol: "subsystem1", Context: "subsystem reference"},
				{Line: 25, Character: 8, Symbol: "operation", Context: "simplified interface"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "Facade", Kind: 5, Type: "class", Description: "Facade providing simplified interface"},
				{Name: "SubsystemA", Kind: 5, Type: "class", Description: "Complex subsystem A"},
				{Name: "SubsystemB", Kind: 5, Type: "class", Description: "Complex subsystem B"},
				{Name: "SubsystemC", Kind: 5, Type: "class", Description: "Complex subsystem C"},
				{Name: "operation", Kind: 6, Type: "method", Description: "Simplified facade operation"},
			},
			LSPMethods: []string{"textDocument/hover", "textDocument/references", "textDocument/definition"},
			Metadata: map[string]interface{}{
				"complexity": "low",
				"interface_simplification": true,
				"subsystem_count": 3,
			},
		},
		{
			Name:        "ProxyAccess",
			Description: "Test workspace symbols on Proxy pattern access control",
			PatternType: "structural",
			FilePath:    "patterns/structural/proxy.py",
			TestPositions: []PythonTestPosition{
				{Line: 14, Character: 6, Symbol: "Proxy", Context: "proxy class"},
				{Line: 22, Character: 10, Symbol: "real_subject", Context: "real object reference"},
				{Line: 28, Character: 8, Symbol: "request", Context: "proxied method"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "Subject", Kind: 5, Type: "class", Description: "Abstract subject interface"},
				{Name: "RealSubject", Kind: 5, Type: "class", Description: "Real subject implementation"},
				{Name: "Proxy", Kind: 5, Type: "class", Description: "Proxy controlling access to real subject"},
				{Name: "request", Kind: 6, Type: "method", Description: "Subject interface method"},
				{Name: "check_access", Kind: 6, Type: "method", Description: "Access control method"},
			},
			LSPMethods: []string{"workspace/symbol", "textDocument/definition", "textDocument/hover"},
			Metadata: map[string]interface{}{
				"complexity": "medium",
				"access_control": true,
				"lazy_initialization": true,
			},
		},
	}
}

// GetBehavioralPatternScenarios returns test scenarios for behavioral design patterns
func GetBehavioralPatternScenarios() []PythonPatternScenario {
	return []PythonPatternScenario{
		{
			Name:        "ObserverNotification",
			Description: "Test definition and references on Observer pattern notification system",
			PatternType: "behavioral",
			FilePath:    "patterns/behavioral/observer.py",
			TestPositions: []PythonTestPosition{
				{Line: 12, Character: 6, Symbol: "Subject", Context: "subject class"},
				{Line: 20, Character: 8, Symbol: "Observer", Context: "observer interface"},
				{Line: 28, Character: 10, Symbol: "notify", Context: "notification method"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "Subject", Kind: 5, Type: "class", Description: "Subject maintaining observer list"},
				{Name: "Observer", Kind: 5, Type: "class", Description: "Abstract observer interface"},
				{Name: "ConcreteObserver", Kind: 5, Type: "class", Description: "Concrete observer implementation"},
				{Name: "attach", Kind: 6, Type: "method", Description: "Attach observer to subject"},
				{Name: "detach", Kind: 6, Type: "method", Description: "Detach observer from subject"},
				{Name: "notify", Kind: 6, Type: "method", Description: "Notify all observers"},
				{Name: "update", Kind: 6, Type: "method", Description: "Observer update method"},
			},
			LSPMethods: []string{"textDocument/definition", "textDocument/references", "workspace/symbol"},
			Metadata: map[string]interface{}{
				"complexity": "medium",
				"notification_pattern": true,
				"loose_coupling": true,
			},
		},
		{
			Name:        "StrategySelection",
			Description: "Test completion on Strategy pattern algorithm selection",
			PatternType: "behavioral",
			FilePath:    "patterns/behavioral/strategy.py",
			TestPositions: []PythonTestPosition{
				{Line: 15, Character: 6, Symbol: "Context", Context: "context class"},
				{Line: 22, Character: 10, Symbol: "strategy", Context: "strategy reference"},
				{Line: 28, Character: 8, Symbol: "execute_algorithm", Context: "strategy method"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "Strategy", Kind: 5, Type: "class", Description: "Abstract strategy interface"},
				{Name: "ConcreteStrategyA", Kind: 5, Type: "class", Description: "First concrete strategy"},
				{Name: "ConcreteStrategyB", Kind: 5, Type: "class", Description: "Second concrete strategy"},
				{Name: "Context", Kind: 5, Type: "class", Description: "Context using strategy"},
				{Name: "execute_algorithm", Kind: 6, Type: "method", Description: "Strategy algorithm method"},
				{Name: "set_strategy", Kind: 6, Type: "method", Description: "Set current strategy"},
			},
			LSPMethods: []string{"textDocument/completion", "textDocument/hover", "textDocument/documentSymbol"},
			Metadata: map[string]interface{}{
				"complexity": "medium",
				"algorithm_family": true,
				"runtime_selection": true,
			},
		},
		{
			Name:        "CommandExecution",
			Description: "Test hover and symbols on Command pattern execution flow",
			PatternType: "behavioral",
			FilePath:    "patterns/behavioral/command.py",
			TestPositions: []PythonTestPosition{
				{Line: 14, Character: 6, Symbol: "Command", Context: "command interface"},
				{Line: 22, Character: 8, Symbol: "ConcreteCommand", Context: "concrete command"},
				{Line: 30, Character: 10, Symbol: "execute", Context: "execution method"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "Command", Kind: 5, Type: "class", Description: "Abstract command interface"},
				{Name: "ConcreteCommand", Kind: 5, Type: "class", Description: "Concrete command implementation"},
				{Name: "Receiver", Kind: 5, Type: "class", Description: "Command receiver object"},
				{Name: "Invoker", Kind: 5, Type: "class", Description: "Command invoker"},
				{Name: "execute", Kind: 6, Type: "method", Description: "Execute command operation"},
				{Name: "undo", Kind: 6, Type: "method", Description: "Undo command operation"},
			},
			LSPMethods: []string{"textDocument/hover", "textDocument/documentSymbol", "textDocument/references"},
			Metadata: map[string]interface{}{
				"complexity": "medium",
				"encapsulate_request": true,
				"undo_support": true,
			},
		},
		{
			Name:        "StateTransition",
			Description: "Test definition lookup on State pattern state transitions",
			PatternType: "behavioral",
			FilePath:    "patterns/behavioral/state.py",
			TestPositions: []PythonTestPosition{
				{Line: 12, Character: 6, Symbol: "State", Context: "state interface"},
				{Line: 20, Character: 8, Symbol: "ConcreteStateA", Context: "concrete state"},
				{Line: 28, Character: 10, Symbol: "handle", Context: "state handler"},
			},
			ExpectedSymbols: []PythonPatternSymbol{
				{Name: "State", Kind: 5, Type: "class", Description: "Abstract state interface"},
				{Name: "ConcreteStateA", Kind: 5, Type: "class", Description: "First concrete state"},
				{Name: "ConcreteStateB", Kind: 5, Type: "class", Description: "Second concrete state"},
				{Name: "Context", Kind: 5, Type: "class", Description: "Context maintaining state"},
				{Name: "handle", Kind: 6, Type: "method", Description: "State-specific handler method"},
				{Name: "change_state", Kind: 6, Type: "method", Description: "State transition method"},
			},
			LSPMethods: []string{"textDocument/definition", "workspace/symbol", "textDocument/hover"},
			Metadata: map[string]interface{}{
				"complexity": "high",
				"finite_state_machine": true,
				"state_transitions": true,
			},
		},
	}
}

// GetAllPythonPatternScenarios returns all Python design pattern test scenarios
func GetAllPythonPatternScenarios() []PythonPatternScenario {
	var allScenarios []PythonPatternScenario
	allScenarios = append(allScenarios, GetCreationalPatternScenarios()...)
	allScenarios = append(allScenarios, GetStructuralPatternScenarios()...)
	allScenarios = append(allScenarios, GetBehavioralPatternScenarios()...)
	return allScenarios
}

// GetScenarioByName returns a specific scenario by name
func GetScenarioByName(name string) *PythonPatternScenario {
	allScenarios := GetAllPythonPatternScenarios()
	for _, scenario := range allScenarios {
		if scenario.Name == name {
			return &scenario
		}
	}
	return nil
}

// GetScenariosForLSPMethod returns scenarios that test a specific LSP method
func GetScenariosForLSPMethod(method string) []PythonPatternScenario {
	var matchingScenarios []PythonPatternScenario
	allScenarios := GetAllPythonPatternScenarios()
	
	for _, scenario := range allScenarios {
		for _, lspMethod := range scenario.LSPMethods {
			if lspMethod == method {
				matchingScenarios = append(matchingScenarios, scenario)
				break
			}
		}
	}
	return matchingScenarios
}

// GetScenariosForPatternType returns scenarios for a specific pattern type
func GetScenariosForPatternType(patternType string) []PythonPatternScenario {
	switch patternType {
	case "creational":
		return GetCreationalPatternScenarios()
	case "structural":
		return GetStructuralPatternScenarios()
	case "behavioral":
		return GetBehavioralPatternScenarios()
	default:
		return []PythonPatternScenario{}
	}
}

// GetScenariosByComplexity returns scenarios filtered by complexity level
func GetScenariosByComplexity(complexity string) []PythonPatternScenario {
	var matchingScenarios []PythonPatternScenario
	allScenarios := GetAllPythonPatternScenarios()
	
	for _, scenario := range allScenarios {
		if scenarioComplexity, exists := scenario.Metadata["complexity"]; exists {
			if scenarioComplexity == complexity {
				matchingScenarios = append(matchingScenarios, scenario)
			}
		}
	}
	return matchingScenarios
}

// GetScenarioMetadata returns aggregated metadata about all scenarios
func GetScenarioMetadata() map[string]interface{} {
	allScenarios := GetAllPythonPatternScenarios()
	metadata := map[string]interface{}{
		"total_scenarios":       len(allScenarios),
		"creational_count":      len(GetCreationalPatternScenarios()),
		"structural_count":      len(GetStructuralPatternScenarios()),
		"behavioral_count":      len(GetBehavioralPatternScenarios()),
		"supported_lsp_methods": []string{
			"textDocument/definition",
			"textDocument/references", 
			"textDocument/hover",
			"textDocument/documentSymbol",
			"workspace/symbol",
			"textDocument/completion",
		},
		"pattern_types": []string{"creational", "structural", "behavioral"},
		"complexity_levels": []string{"low", "medium", "high"},
	}
	return metadata
}