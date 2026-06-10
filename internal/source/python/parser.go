package python

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// fileAnalysis is the top-level container returned by parseFile, holding
// all AST-extracted entities from one Python source file.
type fileAnalysis struct {
	Classes   []classDef
	Functions []functionDef
	Calls     []callRef
}

// callRef is a lightweight call-site reference collected during AST walking.
type callRef struct {
	Name string
	Kind string // "function", "method", "attribute"
}

// classDef holds data extracted from a class_definition AST node.
type classDef struct {
	Name       string
	Bases      []string
	Decorators []string
	Docstring  string
	Methods    []methodDef
	Properties []propertyDef
	StartPos   int
	EndPos     int
	StartLine  int
}

// functionDef holds data extracted from a top-level function_definition node.
type functionDef struct {
	Name       string
	Params     []paramDef
	ReturnType string
	Decorators []string
	Docstring  string
	Async      bool
	StartPos   int
	EndPos     int
	StartLine  int
}

// methodDef holds data extracted from a method definition within a class.
type methodDef struct {
	Name       string
	Params     []paramDef
	ReturnType string
	Decorators []string
	Docstring  string
	Async      bool
	StartPos   int
	EndPos     int
	StartLine  int
}

// paramDef holds a single function/method parameter.
type paramDef struct {
	Name    string
	Type    string
	Default string
	Star    bool // *args
	StarStar bool // **kwargs
}

// propertyDef holds data for a class property (from @property or assignment).
type propertyDef struct {
	Name        string
	Type        string
	Description string
	Visibility  string
}

// parseFile parses a Python source file using tree-sitter and returns the
// collected analysis: classes (with methods/properties), top-level
// functions, and call references.
func parseFile(src []byte) *fileAnalysis {
	parser, err := treesitter.Get(treesitter.Python)
	if err != nil {
		return &fileAnalysis{}
	}
	defer treesitter.Put(treesitter.Python, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return &fileAnalysis{}
	}
	defer tree.Close()

	result := &fileAnalysis{}
	root := tree.RootNode()
	walkTopLevel(root, src, result)
	return result
}

// walkTopLevel walks only the top-level children of the module node,
// extracting classes, functions, and call references.
func walkTopLevel(node *sitter.Node, src []byte, result *fileAnalysis) {
	if node == nil {
		return
	}

	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}

		switch child.Type() {
		case "class_definition":
			cd := extractClass(child, src)
			result.Classes = append(result.Classes, cd)
		case "function_definition":
			fd := extractFunction(child, src, false)
			result.Functions = append(result.Functions, fd)
		case "decorated_definition":
			result.extractDecorated(child, src)
		case "expression_statement":
			// Look for call expressions at module level
			walkForCalls(child, src, result)
		case "assignment":
			walkForCalls(child, src, result)
		default:
			walkForCalls(child, src, result)
		}
	}
}

// extractDecorated handles a decorated_definition node, extracting the
// decorators and the underlying class or function definition.
func (result *fileAnalysis) extractDecorated(node *sitter.Node, src []byte) {
	decorators := extractDecorators(node, src)

	// The definition is the last named child (after all decorator nodes)
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "class_definition":
			cd := extractClass(child, src)
			cd.Decorators = append(decorators, cd.Decorators...)
			// Use the decorated_definition's start position
			cd.StartPos = int(node.StartByte())
			cd.StartLine = int(node.StartPoint().Row) + 1
			result.Classes = append(result.Classes, cd)
		case "function_definition":
			fd := extractFunction(child, src, false)
			fd.Decorators = append(decorators, fd.Decorators...)
			fd.StartPos = int(node.StartByte())
			fd.StartLine = int(node.StartPoint().Row) + 1
			result.Functions = append(result.Functions, fd)
		}
	}
}

// extractDecorators extracts decorator names from a decorated_definition node.
func extractDecorators(node *sitter.Node, src []byte) []string {
	var decorators []string
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil || child.Type() != "decorator" {
			continue
		}
		// The decorator text is everything between @ and end of decorator
		text := strings.TrimSpace(string(src[child.StartByte():child.EndByte()]))
		text = strings.TrimPrefix(text, "@")
		// Remove arguments from decorator for classification
		if idx := strings.Index(text, "("); idx > 0 {
			decorators = append(decorators, strings.TrimSpace(text[:idx]))
		} else {
			decorators = append(decorators, strings.TrimSpace(text))
		}
	}
	return decorators
}

// extractClass builds a classDef from a class_definition AST node.
func extractClass(node *sitter.Node, src []byte) classDef {
	cd := classDef{
		StartPos:  int(node.StartByte()),
		EndPos:    int(node.EndByte()),
		StartLine: int(node.StartPoint().Row) + 1,
	}

	// Extract class name
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		cd.Name = string(src[nameNode.StartByte():nameNode.EndByte()])
	}

	// Extract base classes from superclasses (argument_list)
	if argList := node.ChildByFieldName("superclasses"); argList != nil {
		cd.Bases = extractBaseClasses(argList, src)
	}

	// Extract body
	body := node.ChildByFieldName("body"); if body != nil {
		cd.Docstring = extractDocstring(body, src)
		extractClassBody(body, src, &cd)
	}

	return cd
}

// extractBaseClasses extracts base class names from an argument_list node.
func extractBaseClasses(node *sitter.Node, src []byte) []string {
	var bases []string
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}
		// Skip keyword arguments (like metaclass=...)
		if child.Type() == "keyword_argument" {
			continue
		}
		text := strings.TrimSpace(string(src[child.StartByte():child.EndByte()]))
		if text != "" {
			bases = append(bases, text)
		}
	}
	return bases
}

// extractClassBody walks the class body and extracts methods and properties.
func extractClassBody(body *sitter.Node, src []byte, cd *classDef) {
	count := body.NamedChildCount()
	for i := range count {
		child := body.NamedChild(int(i))
		if child == nil {
			continue
		}

		switch child.Type() {
		case "function_definition":
			md := extractMethodFromFunc(child, src, nil)
			cd.Methods = append(cd.Methods, md)
		case "decorated_definition":
			decorators := extractDecorators(child, src)
			// Find the function_definition inside
			innerCount := child.NamedChildCount()
			for j := range innerCount {
				inner := child.NamedChild(int(j))
				if inner == nil {
					continue
				}
				if inner.Type() == "function_definition" {
					md := extractMethodFromFunc(inner, src, decorators)
					md.StartPos = int(child.StartByte())
					md.StartLine = int(child.StartPoint().Row) + 1
					cd.Methods = append(cd.Methods, md)

					// If it's a @property, add as a property too
					for _, dec := range md.Decorators {
						if dec == "property" {
							cd.Properties = append(cd.Properties, propertyDef{
								Name:       md.Name,
								Type:       md.ReturnType,
								Visibility: "public",
							})
							break
						}
					}
				}
			}
		case "expression_statement":
			// Check for class-level assignments (type annotations)
			extractClassLevelAssignment(child, src, cd)
		}
	}
}

// extractClassLevelAssignment extracts class-level variable assignments
// and type annotations as properties.
func extractClassLevelAssignment(node *sitter.Node, src []byte, cd *classDef) {
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "assignment":
			// name = value or name: type = value
			if left := child.ChildByFieldName("left"); left != nil {
				name := strings.TrimSpace(string(src[left.StartByte():left.EndByte()]))
				if name != "" && !strings.Contains(name, ".") {
					visibility := "public"
					if strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__") {
						visibility = "private"
					} else if strings.HasPrefix(name, "_") {
						visibility = "protected"
					}
					if typeNode := child.ChildByFieldName("type"); typeNode != nil {
						typeName := strings.TrimSpace(string(src[typeNode.StartByte():typeNode.EndByte()]))
						cd.Properties = append(cd.Properties, propertyDef{
							Name:       name,
							Type:       typeName,
							Visibility: visibility,
						})
					} else {
						cd.Properties = append(cd.Properties, propertyDef{
							Name:       name,
							Visibility: visibility,
						})
					}
				}
			}
		case "type":
			// Type alias or annotated assignment
			// e.g., name: type
			handleTypeAnnotation(child, src, cd)
		}
	}
}

// handleTypeAnnotation handles standalone type annotations at class level.
func handleTypeAnnotation(node *sitter.Node, src []byte, cd *classDef) {
	text := strings.TrimSpace(string(src[node.StartByte():node.EndByte()]))
	if colonIdx := strings.Index(text, ":"); colonIdx > 0 {
		name := strings.TrimSpace(text[:colonIdx])
		typeName := strings.TrimSpace(text[colonIdx+1:])
		if name != "" {
			visibility := "public"
			if strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__") {
				visibility = "private"
			} else if strings.HasPrefix(name, "_") {
				visibility = "protected"
			}
			cd.Properties = append(cd.Properties, propertyDef{
				Name:       name,
				Type:       typeName,
				Visibility: visibility,
			})
		}
	}
}

// extractMethodFromFunc builds a methodDef from a function_definition node
// that appears inside a class body.
func extractMethodFromFunc(node *sitter.Node, src []byte, decorators []string) methodDef {
	md := methodDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		StartLine:  int(node.StartPoint().Row) + 1,
		Decorators: decorators,
	}

	// Check if async
	// In tree-sitter Python grammar, async functions inside a class are still
	// function_definition but the parent may have "async" keyword.
	// Actually, async functions should be captured as function_definition with
	// a preceding "async" keyword in the source.
	nodeText := string(src[node.StartByte():node.EndByte()])
	if strings.HasPrefix(strings.TrimSpace(nodeText), "async ") {
		md.Async = true
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		md.Name = string(src[nameNode.StartByte():nameNode.EndByte()])
	}

	if paramsNode := node.ChildByFieldName("parameters"); paramsNode != nil {
		md.Params = extractParameters(paramsNode, src)
	}

	if retNode := node.ChildByFieldName("return_type"); retNode != nil {
		md.ReturnType = strings.TrimSpace(string(src[retNode.StartByte():retNode.EndByte()]))
	}

	if body := node.ChildByFieldName("body"); body != nil {
		md.Docstring = extractDocstring(body, src)
	}

	return md
}

// extractFunction builds a functionDef from a top-level function_definition node.
func extractFunction(node *sitter.Node, src []byte, async bool) functionDef {
	fd := functionDef{
		StartPos:  int(node.StartByte()),
		EndPos:    int(node.EndByte()),
		StartLine: int(node.StartPoint().Row) + 1,
		Async:     async,
	}

	// Check if source starts with "async"
	nodeText := string(src[node.StartByte():node.EndByte()])
	if strings.HasPrefix(strings.TrimSpace(nodeText), "async ") {
		fd.Async = true
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		fd.Name = string(src[nameNode.StartByte():nameNode.EndByte()])
	}

	if paramsNode := node.ChildByFieldName("parameters"); paramsNode != nil {
		fd.Params = extractParameters(paramsNode, src)
	}

	if retNode := node.ChildByFieldName("return_type"); retNode != nil {
		fd.ReturnType = strings.TrimSpace(string(src[retNode.StartByte():retNode.EndByte()]))
	}

	if body := node.ChildByFieldName("body"); body != nil {
		fd.Docstring = extractDocstring(body, src)
	}

	return fd
}

// extractParameters extracts parameter definitions from a parameters node.
func extractParameters(node *sitter.Node, src []byte) []paramDef {
	var params []paramDef
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			// Simple parameter name (like 'self', 'cls', or a plain arg)
			name := string(src[child.StartByte():child.EndByte()])
			params = append(params, paramDef{Name: name})

		case "typed_parameter":
			p := extractTypedParam(child, src)
			params = append(params, p)

		case "default_parameter":
			p := extractDefaultParam(child, src)
			params = append(params, p)

		case "typed_default_parameter":
			p := extractTypedDefaultParam(child, src)
			params = append(params, p)

		case "list_splat_pattern":
			// *args
			name := extractParamName(child, src)
			params = append(params, paramDef{Name: name, Star: true})

		case "dictionary_splat_pattern":
			// **kwargs
			name := extractParamName(child, src)
			params = append(params, paramDef{Name: name, StarStar: true})
		}
	}
	return params
}

// extractTypedParam handles "name: type" parameters.
func extractTypedParam(node *sitter.Node, src []byte) paramDef {
	p := paramDef{}

	// Check for splat patterns within typed params
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "identifier":
			if p.Name == "" {
				p.Name = string(src[child.StartByte():child.EndByte()])
			}
		case "list_splat_pattern":
			p.Name = extractParamName(child, src)
			p.Star = true
		case "dictionary_splat_pattern":
			p.Name = extractParamName(child, src)
			p.StarStar = true
		case "type":
			p.Type = strings.TrimSpace(string(src[child.StartByte():child.EndByte()]))
		}
	}

	// If name not found from children, try field name
	if p.Name == "" {
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			p.Name = string(src[nameNode.StartByte():nameNode.EndByte()])
		}
	}

	// If type not found, try the "type" field
	if p.Type == "" {
		if typeNode := node.ChildByFieldName("type"); typeNode != nil {
			p.Type = strings.TrimSpace(string(src[typeNode.StartByte():typeNode.EndByte()]))
		}
	}

	return p
}

// extractDefaultParam handles "name = default" parameters.
func extractDefaultParam(node *sitter.Node, src []byte) paramDef {
	p := paramDef{}
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		p.Name = string(src[nameNode.StartByte():nameNode.EndByte()])
	}
	if valNode := node.ChildByFieldName("value"); valNode != nil {
		p.Default = strings.TrimSpace(string(src[valNode.StartByte():valNode.EndByte()]))
	}
	return p
}

// extractTypedDefaultParam handles "name: type = default" parameters.
func extractTypedDefaultParam(node *sitter.Node, src []byte) paramDef {
	p := paramDef{}
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		p.Name = string(src[nameNode.StartByte():nameNode.EndByte()])
	}
	if typeNode := node.ChildByFieldName("type"); typeNode != nil {
		p.Type = strings.TrimSpace(string(src[typeNode.StartByte():typeNode.EndByte()]))
	}
	if valNode := node.ChildByFieldName("value"); valNode != nil {
		p.Default = strings.TrimSpace(string(src[valNode.StartByte():valNode.EndByte()]))
	}
	return p
}

// extractParamName gets the name from a splat pattern node.
func extractParamName(node *sitter.Node, src []byte) string {
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child != nil && child.Type() == "identifier" {
			return string(src[child.StartByte():child.EndByte()])
		}
	}
	// Fallback: extract from text
	text := string(src[node.StartByte():node.EndByte()])
	text = strings.TrimPrefix(text, "**")
	text = strings.TrimPrefix(text, "*")
	return strings.TrimSpace(text)
}

// extractDocstring extracts the docstring from a block/body node.
// In Python, the docstring is the first expression statement in a body
// if that expression is a string literal.
func extractDocstring(body *sitter.Node, src []byte) string {
	if body == nil {
		return ""
	}

	// The first named child of the body (block node) should be an
	// expression_statement containing a string.
	count := body.NamedChildCount()
	if count == 0 {
		return ""
	}

	first := body.NamedChild(0)
	if first == nil {
		return ""
	}

	if first.Type() != "expression_statement" {
		return ""
	}

	// The expression_statement should contain a string node
	exprCount := first.NamedChildCount()
	if exprCount == 0 {
		return ""
	}

	strNode := first.NamedChild(0)
	if strNode == nil {
		return ""
	}

	if strNode.Type() != "string" && strNode.Type() != "concatenated_string" {
		return ""
	}

	text := string(src[strNode.StartByte():strNode.EndByte()])
	return stripQuotes(text)
}

// stripQuotes removes Python string delimiters (single/double, triple-quoted).
func stripQuotes(s string) string {
	// Triple-quoted strings
	for _, q := range []string{`"""`, `'''`} {
		if strings.HasPrefix(s, q) && strings.HasSuffix(s, q) && len(s) >= 6 {
			return strings.TrimSpace(s[3 : len(s)-3])
		}
	}
	// Single-quoted strings
	for _, q := range []string{`"`, `'`} {
		if strings.HasPrefix(s, q) && strings.HasSuffix(s, q) && len(s) >= 2 {
			return strings.TrimSpace(s[1 : len(s)-1])
		}
	}
	return s
}

// walkForCalls recursively walks a node tree and collects call references.
func walkForCalls(node *sitter.Node, src []byte, result *fileAnalysis) {
	if node == nil {
		return
	}

	if node.Type() == "call" {
		if fnNode := node.ChildByFieldName("function"); fnNode != nil {
			name := strings.TrimSpace(string(src[fnNode.StartByte():fnNode.EndByte()]))
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
		walkForCalls(node.NamedChild(int(i)), src, result)
	}
}
