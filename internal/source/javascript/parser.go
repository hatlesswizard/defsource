package javascript

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// fileAnalysis is the top-level container returned by parseFile, holding
// all AST-extracted entities from one JavaScript source file.
type fileAnalysis struct {
	Classes   []classDef
	Functions []funcDef
	Modules   []moduleDef
	Constants []constDef
	Calls     []callRef
}

// callRef is a lightweight call-site reference collected during AST walking.
type callRef struct {
	Name string
	Kind string // "function", "method"
}

// classDef holds data extracted from a class_declaration AST node.
type classDef struct {
	Name       string
	StartPos   int
	EndPos     int
	DocComment string
	Exported   bool
	Methods    []methodDef
	Properties []propertyDef
}

// funcDef holds data extracted from a function declaration or exported
// function expression.
type funcDef struct {
	Name       string
	Signature  string
	StartPos   int
	EndPos     int
	DocComment string
	Exported   bool
	IsArrow    bool
}

// moduleDef holds data for an exported object literal with methods.
type moduleDef struct {
	Name       string
	StartPos   int
	EndPos     int
	DocComment string
	Exported   bool
	Methods    []methodDef
}

// constDef holds data for an exported constant.
type constDef struct {
	Name       string
	StartPos   int
	EndPos     int
	DocComment string
	Exported   bool
}

// methodDef holds data for a class method or object method.
type methodDef struct {
	Name       string
	Signature  string
	StartPos   int
	EndPos     int
	DocComment string
	Visibility string
	Static     bool
	IsGetter   bool
	IsSetter   bool
	IsAsync    bool
}

// propertyDef holds data for a class property / field.
type propertyDef struct {
	Name       string
	Type       string
	Visibility string
	DocComment string
}

// parseFile parses a JavaScript source file using tree-sitter and returns
// the collected analysis.
func parseFile(src []byte) *fileAnalysis {
	parser, err := treesitter.Get(treesitter.JavaScript)
	if err != nil {
		return &fileAnalysis{}
	}
	defer treesitter.Put(treesitter.JavaScript, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return &fileAnalysis{}
	}
	defer tree.Close()

	result := &fileAnalysis{}
	root := tree.RootNode()
	walkProgram(root, src, result)
	return result
}

// walkProgram walks the top-level program node and dispatches on statement types.
func walkProgram(node *sitter.Node, src []byte, result *fileAnalysis) {
	if node == nil {
		return
	}

	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}
		processTopLevel(child, src, result)
	}
}

// processTopLevel handles a single top-level statement node.
func processTopLevel(node *sitter.Node, src []byte, result *fileAnalysis) {
	switch node.Type() {
	case "export_statement":
		processExportStatement(node, src, result)

	case "class_declaration":
		cls := extractClass(node, src)
		result.Classes = append(result.Classes, cls)

	case "function_declaration":
		fn := extractFunction(node, src)
		result.Functions = append(result.Functions, fn)

	case "lexical_declaration":
		processLexicalDeclaration(node, src, result, false)

	case "variable_declaration":
		processVariableDeclaration(node, src, result, false)

	case "expression_statement":
		processExpressionStatement(node, src, result)

	default:
		// Recurse to find nested call expressions for ref counting.
		collectCalls(node, src, result)
	}
}

// processExportStatement handles `export` and `export default` statements.
func processExportStatement(node *sitter.Node, src []byte, result *fileAnalysis) {
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}

		switch child.Type() {
		case "class_declaration", "class":
			cls := extractClass(child, src)
			cls.Exported = true
			// Use the export statement span for the entity position and doc.
			cls.StartPos = int(node.StartByte())
			cls.EndPos = int(node.EndByte())
			if cls.DocComment == "" {
				cls.DocComment = findPrecedingDoc(src, int(node.StartByte()))
			}
			if cls.Name == "" {
				cls.Name = "default"
			}
			result.Classes = append(result.Classes, cls)

		case "function_declaration", "function":
			fn := extractFunction(child, src)
			fn.Exported = true
			fn.StartPos = int(node.StartByte())
			fn.EndPos = int(node.EndByte())
			if fn.DocComment == "" {
				fn.DocComment = findPrecedingDoc(src, int(node.StartByte()))
			}
			if fn.Name == "" {
				fn.Name = "default"
			}
			result.Functions = append(result.Functions, fn)

		case "lexical_declaration":
			processLexicalDeclaration(child, src, result, true)

		case "variable_declaration":
			processVariableDeclaration(child, src, result, true)

		case "identifier":
			// export { name } — mark existing entities as exported.
			name := nodeText(child, src)
			markExported(name, result)

		case "export_clause":
			// export { a, b, c }
			cc := child.NamedChildCount()
			for j := range cc {
				spec := child.NamedChild(int(j))
				if spec != nil && spec.Type() == "export_specifier" {
					nameNode := spec.ChildByFieldName("name")
					if nameNode != nil {
						markExported(nodeText(nameNode, src), result)
					}
				}
			}

		case "arrow_function":
			fn := funcDef{
				Name:     "default",
				StartPos: int(node.StartByte()),
				EndPos:   int(node.EndByte()),
				Exported: true,
				IsArrow:  true,
			}
			fn.Signature = extractArrowSignature(child, src)
			fn.DocComment = findPrecedingDoc(src, int(node.StartByte()))
			result.Functions = append(result.Functions, fn)

		case "object":
			// export default { ... } — treat as a module.
			mod := extractObjectModule("default", child, src)
			mod.Exported = true
			mod.StartPos = int(node.StartByte())
			mod.EndPos = int(node.EndByte())
			mod.DocComment = findPrecedingDoc(src, int(node.StartByte()))
			result.Modules = append(result.Modules, mod)
		}
	}
}

// processLexicalDeclaration handles const/let/var declarations at top level.
func processLexicalDeclaration(node *sitter.Node, src []byte, result *fileAnalysis, exported bool) {
	count := node.NamedChildCount()
	for i := range count {
		decl := node.NamedChild(int(i))
		if decl == nil || decl.Type() != "variable_declarator" {
			continue
		}

		nameNode := decl.ChildByFieldName("name")
		valueNode := decl.ChildByFieldName("value")
		if nameNode == nil {
			continue
		}

		name := nodeText(nameNode, src)
		if name == "" {
			continue
		}

		docComment := findPrecedingDoc(src, int(node.StartByte()))

		if valueNode == nil {
			// Constant with no value — treat as simple constant.
			c := constDef{
				Name:       name,
				StartPos:   int(node.StartByte()),
				EndPos:     int(node.EndByte()),
				DocComment: docComment,
				Exported:   exported,
			}
			result.Constants = append(result.Constants, c)
			continue
		}

		switch valueNode.Type() {
		case "class", "class_expression":
			cls := extractClass(valueNode, src)
			if cls.Name == "" {
				cls.Name = name
			}
			cls.Exported = exported
			cls.StartPos = int(node.StartByte())
			cls.EndPos = int(node.EndByte())
			cls.DocComment = docComment
			result.Classes = append(result.Classes, cls)

		case "function", "function_expression", "arrow_function":
			fn := funcDef{
				Name:       name,
				StartPos:   int(node.StartByte()),
				EndPos:     int(node.EndByte()),
				DocComment: docComment,
				Exported:   exported,
				IsArrow:    valueNode.Type() == "arrow_function",
			}
			if valueNode.Type() == "arrow_function" {
				fn.Signature = extractArrowSignature(valueNode, src)
			} else {
				fn.Signature = extractFunctionSignature(valueNode, src, name)
			}
			result.Functions = append(result.Functions, fn)

		case "object":
			mod := extractObjectModule(name, valueNode, src)
			mod.Exported = exported
			mod.StartPos = int(node.StartByte())
			mod.EndPos = int(node.EndByte())
			mod.DocComment = docComment
			result.Modules = append(result.Modules, mod)

		case "call_expression":
			// Factory pattern: const app = express()
			c := constDef{
				Name:       name,
				StartPos:   int(node.StartByte()),
				EndPos:     int(node.EndByte()),
				DocComment: docComment,
				Exported:   exported,
			}
			result.Constants = append(result.Constants, c)
			collectCalls(valueNode, src, result)

		default:
			c := constDef{
				Name:       name,
				StartPos:   int(node.StartByte()),
				EndPos:     int(node.EndByte()),
				DocComment: docComment,
				Exported:   exported,
			}
			result.Constants = append(result.Constants, c)
		}
	}
}

// processVariableDeclaration handles var declarations (same logic as lexical).
func processVariableDeclaration(node *sitter.Node, src []byte, result *fileAnalysis, exported bool) {
	processLexicalDeclaration(node, src, result, exported)
}

// processExpressionStatement handles expression statements that might be
// module.exports assignments or prototype method definitions.
func processExpressionStatement(node *sitter.Node, src []byte, result *fileAnalysis) {
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}

		if child.Type() == "assignment_expression" {
			processAssignment(child, src, result)
		} else {
			collectCalls(child, src, result)
		}
	}
}

// processAssignment handles assignments that define exports or prototype methods.
func processAssignment(node *sitter.Node, src []byte, result *fileAnalysis) {
	left := node.ChildByFieldName("left")
	right := node.ChildByFieldName("right")
	if left == nil {
		return
	}
	// Fallback: if right is nil, scan named children for the value.
	if right == nil {
		count := node.NamedChildCount()
		for i := range count {
			child := node.NamedChild(int(i))
			if child != nil && child != left {
				right = child
				break
			}
		}
	}
	if right == nil {
		return
	}

	leftText := nodeText(left, src)

	// module.exports = ... or exports.name = ...
	if leftText == "module.exports" {
		processModuleExports(right, src, result, int(node.StartByte()), int(node.EndByte()))
		return
	}

	if strings.HasPrefix(leftText, "exports.") {
		name := strings.TrimPrefix(leftText, "exports.")
		docComment := findPrecedingDoc(src, int(node.StartByte()))
		switch right.Type() {
		case "function", "function_expression", "arrow_function":
			fn := funcDef{
				Name:       name,
				StartPos:   int(node.StartByte()),
				EndPos:     int(node.EndByte()),
				DocComment: docComment,
				Exported:   true,
				IsArrow:    right.Type() == "arrow_function",
			}
			if right.Type() == "arrow_function" {
				fn.Signature = extractArrowSignature(right, src)
			} else {
				fn.Signature = extractFunctionSignature(right, src, name)
			}
			result.Functions = append(result.Functions, fn)
		case "class", "class_expression":
			cls := extractClass(right, src)
			if cls.Name == "" {
				cls.Name = name
			}
			cls.Exported = true
			cls.StartPos = int(node.StartByte())
			cls.EndPos = int(node.EndByte())
			cls.DocComment = docComment
			result.Classes = append(result.Classes, cls)
		default:
			c := constDef{
				Name:       name,
				StartPos:   int(node.StartByte()),
				EndPos:     int(node.EndByte()),
				DocComment: docComment,
				Exported:   true,
			}
			result.Constants = append(result.Constants, c)
		}
		return
	}

	// Prototype method: ClassName.prototype.methodName = function() {}
	if strings.Contains(leftText, ".prototype.") {
		processPrototypeMethod(left, right, src, result, int(node.StartByte()), int(node.EndByte()))
		return
	}

	collectCalls(node, src, result)
}

// processModuleExports handles `module.exports = expr`.
func processModuleExports(value *sitter.Node, src []byte, result *fileAnalysis, startPos, endPos int) {
	docComment := findPrecedingDoc(src, startPos)

	switch value.Type() {
	case "object":
		mod := extractObjectModule("exports", value, src)
		mod.Exported = true
		mod.StartPos = startPos
		mod.EndPos = endPos
		mod.DocComment = docComment
		result.Modules = append(result.Modules, mod)

	case "class", "class_expression":
		cls := extractClass(value, src)
		if cls.Name == "" {
			cls.Name = "exports"
		}
		cls.Exported = true
		cls.StartPos = startPos
		cls.EndPos = endPos
		cls.DocComment = docComment
		result.Classes = append(result.Classes, cls)

	case "function", "function_expression", "arrow_function":
		fn := funcDef{
			Name:       "exports",
			StartPos:   startPos,
			EndPos:     endPos,
			DocComment: docComment,
			Exported:   true,
			IsArrow:    value.Type() == "arrow_function",
		}
		result.Functions = append(result.Functions, fn)

	case "identifier":
		// module.exports = SomeClass — mark as exported.
		name := nodeText(value, src)
		markExported(name, result)
	}
}

// processPrototypeMethod handles Class.prototype.method = function() {}
func processPrototypeMethod(left, right *sitter.Node, src []byte, result *fileAnalysis, startPos, endPos int) {
	leftText := nodeText(left, src)
	parts := strings.SplitN(leftText, ".prototype.", 2)
	if len(parts) != 2 {
		return
	}
	className := parts[0]
	methodName := parts[1]

	docComment := findPrecedingDoc(src, startPos)

	m := methodDef{
		Name:       methodName,
		StartPos:   startPos,
		EndPos:     endPos,
		DocComment: docComment,
		Visibility: "public",
	}

	if right.Type() == "arrow_function" {
		m.Signature = extractArrowSignature(right, src)
	} else if right.Type() == "function" || right.Type() == "function_expression" {
		m.Signature = extractFunctionSignature(right, src, methodName)
	}

	// Find or create the class definition.
	for i := range result.Classes {
		if result.Classes[i].Name == className {
			result.Classes[i].Methods = append(result.Classes[i].Methods, m)
			return
		}
	}

	// No class found, create a placeholder.
	cls := classDef{
		Name:     className,
		StartPos: startPos,
		EndPos:   endPos,
		Exported: true,
		Methods:  []methodDef{m},
	}
	result.Classes = append(result.Classes, cls)
}

// extractClass builds a classDef from a class_declaration or class node.
func extractClass(node *sitter.Node, src []byte) classDef {
	var cls classDef
	cls.StartPos = int(node.StartByte())
	cls.EndPos = int(node.EndByte())

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		cls.Name = nodeText(nameNode, src)
	}

	cls.DocComment = findPrecedingDoc(src, cls.StartPos)

	body := node.ChildByFieldName("body")
	if body == nil {
		return cls
	}

	count := body.NamedChildCount()
	for i := range count {
		child := body.NamedChild(int(i))
		if child == nil {
			continue
		}

		switch child.Type() {
		case "method_definition":
			m := extractMethodDefinition(child, src)
			cls.Methods = append(cls.Methods, m)
		case "field_definition", "public_field_definition":
			p := extractFieldDefinition(child, src)
			if p.Name != "" {
				cls.Properties = append(cls.Properties, p)
			}
		}
	}

	return cls
}

// extractMethodDefinition builds a methodDef from a method_definition node.
func extractMethodDefinition(node *sitter.Node, src []byte) methodDef {
	var m methodDef
	m.StartPos = int(node.StartByte())
	m.EndPos = int(node.EndByte())
	m.Visibility = "public"

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		m.Name = nodeText(nameNode, src)
	}

	// Check for static, get, set, async modifiers.
	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child == nil {
			continue
		}
		text := nodeText(child, src)
		switch text {
		case "static":
			m.Static = true
		case "get":
			m.IsGetter = true
		case "set":
			m.IsSetter = true
		case "async":
			m.IsAsync = true
		}
	}

	// Private methods start with #.
	if strings.HasPrefix(m.Name, "#") {
		m.Visibility = "private"
	}

	// Extract parameters for signature.
	params := node.ChildByFieldName("parameters")
	if params != nil {
		m.Signature = extractParams(params, src)
	}

	m.DocComment = findPrecedingDoc(src, m.StartPos)
	return m
}

// extractFieldDefinition builds a propertyDef from a field node.
func extractFieldDefinition(node *sitter.Node, src []byte) propertyDef {
	var p propertyDef
	p.Visibility = "public"

	propNode := node.ChildByFieldName("property")
	if propNode != nil {
		p.Name = nodeText(propNode, src)
	} else {
		// Fallback: first named child might be the name.
		if node.NamedChildCount() > 0 {
			first := node.NamedChild(0)
			if first != nil {
				p.Name = nodeText(first, src)
			}
		}
	}

	if strings.HasPrefix(p.Name, "#") {
		p.Visibility = "private"
	}

	// Check for static.
	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child != nil && nodeText(child, src) == "static" {
			p.Visibility = "public" // static fields are always public.
			break
		}
	}

	p.DocComment = findPrecedingDoc(src, int(node.StartByte()))
	return p
}

// extractFunction builds a funcDef from a function_declaration or function node.
func extractFunction(node *sitter.Node, src []byte) funcDef {
	var fn funcDef
	fn.StartPos = int(node.StartByte())
	fn.EndPos = int(node.EndByte())

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		fn.Name = nodeText(nameNode, src)
	}

	fn.Signature = extractFunctionSignature(node, src, fn.Name)
	fn.DocComment = findPrecedingDoc(src, fn.StartPos)
	return fn
}

// extractFunctionSignature extracts the parameter list as a signature string.
func extractFunctionSignature(node *sitter.Node, src []byte, name string) string {
	params := node.ChildByFieldName("parameters")
	if params == nil {
		return name + "()"
	}
	return name + "(" + extractParams(params, src) + ")"
}

// extractArrowSignature extracts parameters from an arrow function.
func extractArrowSignature(node *sitter.Node, src []byte) string {
	// Arrow functions can have:
	// 1. (params) => body — formal_parameters
	// 2. singleParam => body — identifier
	params := node.ChildByFieldName("parameters")
	if params != nil {
		if params.Type() == "formal_parameters" {
			return extractParams(params, src)
		}
		// Single identifier parameter.
		return nodeText(params, src)
	}

	// Check first named child for parameter.
	if node.NamedChildCount() > 0 {
		first := node.NamedChild(0)
		if first != nil && first.Type() == "identifier" {
			return nodeText(first, src)
		}
	}
	return ""
}

// extractParams extracts the parameter names from a formal_parameters node.
func extractParams(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}

	var params []string
	count := node.NamedChildCount()
	for i := range count {
		param := node.NamedChild(int(i))
		if param == nil {
			continue
		}

		switch param.Type() {
		case "identifier":
			params = append(params, nodeText(param, src))
		case "assignment_pattern":
			// default parameter: name = value
			left := param.ChildByFieldName("left")
			right := param.ChildByFieldName("right")
			if left != nil {
				name := nodeText(left, src)
				if right != nil {
					name += " = " + nodeText(right, src)
				}
				params = append(params, name)
			}
		case "rest_pattern":
			// ...args
			params = append(params, nodeText(param, src))
		case "object_pattern", "array_pattern":
			// Destructured parameter.
			params = append(params, nodeText(param, src))
		default:
			text := nodeText(param, src)
			if text != "" {
				params = append(params, text)
			}
		}
	}

	return strings.Join(params, ", ")
}

// extractObjectModule builds a moduleDef from an object literal node.
func extractObjectModule(name string, node *sitter.Node, src []byte) moduleDef {
	mod := moduleDef{
		Name:     name,
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}

		if child.Type() == "pair" || child.Type() == "method_definition" {
			m := extractObjectMethod(child, src)
			if m.Name != "" {
				mod.Methods = append(mod.Methods, m)
			}
		}
	}

	return mod
}

// extractObjectMethod extracts a method from a pair node or method_definition
// within an object literal.
func extractObjectMethod(node *sitter.Node, src []byte) methodDef {
	var m methodDef
	m.StartPos = int(node.StartByte())
	m.EndPos = int(node.EndByte())
	m.Visibility = "public"

	if node.Type() == "method_definition" {
		return extractMethodDefinition(node, src)
	}

	// pair: key: value
	key := node.ChildByFieldName("key")
	value := node.ChildByFieldName("value")
	if key == nil || value == nil {
		return m
	}

	m.Name = nodeText(key, src)
	m.DocComment = findPrecedingDoc(src, m.StartPos)

	switch value.Type() {
	case "function", "function_expression", "arrow_function":
		if value.Type() == "arrow_function" {
			m.Signature = extractArrowSignature(value, src)
		} else {
			params := value.ChildByFieldName("parameters")
			if params != nil {
				m.Signature = extractParams(params, src)
			}
		}
	default:
		// Non-function property — skip.
		m.Name = ""
	}

	return m
}

// collectCalls recursively collects call references from a node subtree.
func collectCalls(node *sitter.Node, src []byte, result *fileAnalysis) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "call_expression":
		fn := node.ChildByFieldName("function")
		if fn != nil {
			name := nodeText(fn, src)
			if name != "" {
				kind := "function"
				if strings.Contains(name, ".") {
					kind = "method"
				}
				result.Calls = append(result.Calls, callRef{Name: name, Kind: kind})
			}
		}
	}

	count := node.NamedChildCount()
	for i := range count {
		collectCalls(node.NamedChild(int(i)), src, result)
	}
}

// markExported sets the Exported flag on any matching entity in result.
func markExported(name string, result *fileAnalysis) {
	for i := range result.Classes {
		if result.Classes[i].Name == name {
			result.Classes[i].Exported = true
			return
		}
	}
	for i := range result.Functions {
		if result.Functions[i].Name == name {
			result.Functions[i].Exported = true
			return
		}
	}
	for i := range result.Modules {
		if result.Modules[i].Name == name {
			result.Modules[i].Exported = true
			return
		}
	}
	for i := range result.Constants {
		if result.Constants[i].Name == name {
			result.Constants[i].Exported = true
			return
		}
	}
}

// findPrecedingDoc locates a /** ... */ JSDoc block immediately before pos.
func findPrecedingDoc(src []byte, pos int) string {
	i := pos - 1
	if i < 0 {
		return ""
	}

	// Skip whitespace.
	for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i--
	}

	// Must end with */
	if i < 1 || src[i] != '/' || src[i-1] != '*' {
		return ""
	}

	end := i
	i -= 2
	for i >= 1 {
		if src[i] == '/' && src[i+1] == '*' && (i+2 < len(src) && src[i+2] == '*') {
			return string(src[i : end+1])
		}
		i--
	}

	return ""
}

// nodeText returns the trimmed text content of a node.
func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	return strings.TrimSpace(string(src[node.StartByte():node.EndByte()]))
}
