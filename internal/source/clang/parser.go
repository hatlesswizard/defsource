package clang

import (
	"context"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// fileAnalysis holds all AST-extracted entities from one C source file.
type fileAnalysis struct {
	Functions []functionDef
	Structs   []structDef
	Enums     []enumDef
	Unions    []unionDef
	Typedefs  []typedefDef
}

// functionDef holds data extracted from a function declaration or definition.
type functionDef struct {
	Name       string
	ReturnType string
	Params     []paramDef
	Signature  string
	StartPos   int
	EndPos     int
	DocComment string
	IsInline   bool
	IsStatic   bool
	IsExtern   bool
}

// paramDef holds a function parameter.
type paramDef struct {
	Name string
	Type string
}

// structDef holds a struct definition extracted from the AST.
type structDef struct {
	Name       string
	Fields     []fieldDef
	StartPos   int
	EndPos     int
	DocComment string
	IsForward  bool // forward declaration only (no body)
}

// fieldDef holds a single struct/union field.
type fieldDef struct {
	Name    string
	Type    string
	BitSize string // non-empty for bit fields
}

// enumDef holds an enum definition.
type enumDef struct {
	Name       string
	Constants  []enumConstant
	StartPos   int
	EndPos     int
	DocComment string
}

// enumConstant holds a single enum constant.
type enumConstant struct {
	Name  string
	Value string
}

// unionDef holds a union definition.
type unionDef struct {
	Name       string
	Fields     []fieldDef
	StartPos   int
	EndPos     int
	DocComment string
}

// typedefDef holds a typedef declaration.
type typedefDef struct {
	Name       string
	Underlying string
	StartPos   int
	EndPos     int
	DocComment string
	IsFuncPtr  bool // whether this is a function pointer typedef
}

// macroDef holds a function-like macro extracted via regex.
type macroDef struct {
	Name       string
	Params     []string
	Body       string
	DocComment string
}

// parseFileC parses a C source file using tree-sitter and returns the analysis.
func parseFileC(src []byte) *fileAnalysis {
	parser, err := treesitter.Get(treesitter.C)
	if err != nil {
		return &fileAnalysis{}
	}
	defer treesitter.Put(treesitter.C, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return &fileAnalysis{}
	}
	defer tree.Close()

	result := &fileAnalysis{}
	root := tree.RootNode()
	walkCNode(root, src, result)
	return result
}

// walkCNode recursively walks the C AST and dispatches on node type.
func walkCNode(node *sitter.Node, src []byte, result *fileAnalysis) {
	if node == nil {
		return
	}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "function_definition":
			fn := extractFunctionDef(child, src)
			if fn != nil {
				result.Functions = append(result.Functions, *fn)
			}

		case "declaration":
			// Declarations can be function prototypes, struct/enum/union/typedef, or variable declarations.
			extractDeclaration(child, src, result)

		case "type_definition":
			td := extractTypedef(child, src)
			if td != nil {
				result.Typedefs = append(result.Typedefs, *td)
			}

		case "struct_specifier":
			st := extractStruct(child, src)
			if st != nil {
				result.Structs = append(result.Structs, *st)
			}

		case "enum_specifier":
			en := extractEnum(child, src)
			if en != nil {
				result.Enums = append(result.Enums, *en)
			}

		case "union_specifier":
			un := extractUnion(child, src)
			if un != nil {
				result.Unions = append(result.Unions, *un)
			}

		case "preproc_ifdef", "preproc_if", "preproc_else", "preproc_elif":
			// Parse all branches of conditional compilation.
			walkCNode(child, src, result)

		default:
			// Recurse into other nodes.
			if child.NamedChildCount() > 0 {
				walkCNode(child, src, result)
			}
		}
	}
}

// extractFunctionDef extracts a function definition from an AST node.
func extractFunctionDef(node *sitter.Node, src []byte) *functionDef {
	fn := &functionDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	// Extract storage class specifiers.
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "storage_class_specifier" {
			spec := nodeText(child, src)
			switch spec {
			case "static":
				fn.IsStatic = true
			case "extern":
				fn.IsExtern = true
			case "inline":
				fn.IsInline = true
			}
		}
	}

	// Get return type from the type node.
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		fn.ReturnType = nodeText(typeNode, src)
	}

	// Get the declarator (contains name and params).
	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		extractFunctionDeclarator(declarator, src, fn)
	}

	// Build signature.
	fn.Signature = buildFunctionSignature(fn)

	// Extract preceding doc comment.
	fn.DocComment = findPrecedingDocComment(src, int(node.StartByte()))

	return fn
}

// extractDeclaration handles a "declaration" node which can contain function
// prototypes, struct/enum/union definitions with variables, or typedefs.
func extractDeclaration(node *sitter.Node, src []byte, result *fileAnalysis) {
	// Check for storage class specifiers.
	isExtern := false
	isStatic := false
	isInline := false
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "storage_class_specifier" {
			spec := nodeText(child, src)
			switch spec {
			case "extern":
				isExtern = true
			case "static":
				isStatic = true
			case "inline":
				isInline = true
			}
		}
	}

	// Look for type specifiers that define struct/enum/union inline.
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		switch typeNode.Type() {
		case "struct_specifier":
			st := extractStruct(typeNode, src)
			if st != nil && st.Name != "" {
				st.DocComment = findPrecedingDocComment(src, int(node.StartByte()))
				st.StartPos = int(node.StartByte())
				st.EndPos = int(node.EndByte())
				result.Structs = append(result.Structs, *st)
			}
		case "enum_specifier":
			en := extractEnum(typeNode, src)
			if en != nil && en.Name != "" {
				en.DocComment = findPrecedingDocComment(src, int(node.StartByte()))
				en.StartPos = int(node.StartByte())
				en.EndPos = int(node.EndByte())
				result.Enums = append(result.Enums, *en)
			}
		case "union_specifier":
			un := extractUnion(typeNode, src)
			if un != nil && un.Name != "" {
				un.DocComment = findPrecedingDocComment(src, int(node.StartByte()))
				un.StartPos = int(node.StartByte())
				un.EndPos = int(node.EndByte())
				result.Unions = append(result.Unions, *un)
			}
		}
	}

	// Look for function declarators (prototypes).
	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		fnDecl := findFunctionDeclaratorInDecl(declarator)
		if fnDecl != nil {
			fn := &functionDef{
				StartPos: int(node.StartByte()),
				EndPos:   int(node.EndByte()),
				IsExtern: isExtern,
				IsStatic: isStatic,
				IsInline: isInline,
			}
			if typeNode != nil {
				fn.ReturnType = nodeText(typeNode, src)
			}
			extractFunctionDeclarator(fnDecl, src, fn)
			fn.Signature = buildFunctionSignature(fn)
			fn.DocComment = findPrecedingDocComment(src, int(node.StartByte()))
			if fn.Name != "" {
				result.Functions = append(result.Functions, *fn)
			}
		}
	}
}

// findFunctionDeclaratorInDecl recursively finds a function_declarator within a
// declaration's declarator subtree.
func findFunctionDeclaratorInDecl(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	if node.Type() == "function_declarator" {
		return node
	}
	// Check children recursively.
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if result := findFunctionDeclaratorInDecl(child); result != nil {
			return result
		}
	}
	return nil
}

// extractFunctionDeclarator extracts name and parameters from a function_declarator node.
func extractFunctionDeclarator(node *sitter.Node, src []byte, fn *functionDef) {
	if node == nil {
		return
	}

	// The function_declarator contains a "declarator" (the name) and "parameters".
	switch node.Type() {
	case "function_declarator":
		nameNode := node.ChildByFieldName("declarator")
		if nameNode != nil {
			// Handle pointer declarators: *func_name
			if nameNode.Type() == "pointer_declarator" {
				fn.ReturnType += " *"
				for i := 0; i < int(nameNode.ChildCount()); i++ {
					c := nameNode.Child(i)
					if c != nil && c.Type() == "identifier" {
						fn.Name = nodeText(c, src)
						break
					}
				}
			} else if nameNode.Type() == "parenthesized_declarator" {
				// Handle pointer-to-function returns like: int (*func)(...)
				fn.Name = extractIdentifierFromNode(nameNode, src)
			} else {
				fn.Name = nodeText(nameNode, src)
			}
		}

		paramsNode := node.ChildByFieldName("parameters")
		if paramsNode != nil {
			fn.Params = extractParams(paramsNode, src)
		}

	case "pointer_declarator":
		fn.ReturnType += " *"
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil {
				extractFunctionDeclarator(child, src, fn)
			}
		}

	case "identifier":
		fn.Name = nodeText(node, src)

	default:
		// Recurse to find function_declarator.
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "function_declarator" {
				extractFunctionDeclarator(child, src, fn)
				return
			}
		}
		// Final fallback: just get the name.
		if node.Type() == "identifier" {
			fn.Name = nodeText(node, src)
		}
	}
}

// extractParams extracts parameters from a parameter_list node.
func extractParams(node *sitter.Node, src []byte) []paramDef {
	if node == nil {
		return nil
	}

	var params []paramDef
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "parameter_declaration":
			p := extractParamDecl(child, src)
			params = append(params, p)
		case "variadic_parameter":
			params = append(params, paramDef{Name: "...", Type: "..."})
		}
	}

	return params
}

// extractParamDecl extracts a single parameter declaration.
func extractParamDecl(node *sitter.Node, src []byte) paramDef {
	p := paramDef{}

	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		p.Type = nodeText(typeNode, src)
	}

	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		p.Name = extractIdentifierFromNode(declarator, src)
		// If declarator is a pointer_declarator, add * to type.
		if declarator.Type() == "pointer_declarator" {
			p.Type += " *"
			p.Name = extractIdentifierFromNode(declarator, src)
		} else if declarator.Type() == "abstract_pointer_declarator" {
			p.Type += " *"
		}
	}

	// Handle case where the entire text is just "void" (no declarator).
	if p.Type == "" && p.Name == "" {
		text := strings.TrimSpace(nodeText(node, src))
		if text == "void" {
			return paramDef{Type: "void"}
		}
		p.Type = text
	}

	return p
}

// extractStruct extracts a struct definition from a struct_specifier node.
func extractStruct(node *sitter.Node, src []byte) *structDef {
	st := &structDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	// Get struct name.
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		st.Name = nodeText(nameNode, src)
	}

	// Get body (field declarations).
	body := node.ChildByFieldName("body")
	if body == nil {
		// Forward declaration.
		st.IsForward = true
		return st
	}

	st.Fields = extractFieldsFromBody(body, src)
	st.DocComment = findPrecedingDocComment(src, int(node.StartByte()))
	return st
}

// extractEnum extracts an enum definition from an enum_specifier node.
func extractEnum(node *sitter.Node, src []byte) *enumDef {
	en := &enumDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		en.Name = nodeText(nameNode, src)
	}

	body := node.ChildByFieldName("body")
	if body == nil {
		return en
	}

	count := int(body.NamedChildCount())
	for i := 0; i < count; i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type() == "enumerator" {
			ec := enumConstant{}
			nameN := child.ChildByFieldName("name")
			if nameN != nil {
				ec.Name = nodeText(nameN, src)
			}
			valueN := child.ChildByFieldName("value")
			if valueN != nil {
				ec.Value = nodeText(valueN, src)
			}
			en.Constants = append(en.Constants, ec)
		}
	}

	en.DocComment = findPrecedingDocComment(src, int(node.StartByte()))
	return en
}

// extractUnion extracts a union definition from a union_specifier node.
func extractUnion(node *sitter.Node, src []byte) *unionDef {
	un := &unionDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		un.Name = nodeText(nameNode, src)
	}

	body := node.ChildByFieldName("body")
	if body == nil {
		return un
	}

	un.Fields = extractFieldsFromBody(body, src)
	un.DocComment = findPrecedingDocComment(src, int(node.StartByte()))
	return un
}

// extractTypedef extracts a typedef from a type_definition node.
func extractTypedef(node *sitter.Node, src []byte) *typedefDef {
	td := &typedefDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	// Get the underlying type.
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		td.Underlying = nodeText(typeNode, src)
	}

	// Get the declared name from the declarator.
	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		td.Name = extractTypedefName(declarator, src)
		// Check if it is a function pointer typedef.
		if isFunctionPointerDeclarator(declarator) {
			td.IsFuncPtr = true
		}
	}

	td.DocComment = findPrecedingDocComment(src, int(node.StartByte()))
	return td
}

// extractTypedefName extracts the name from a typedef declarator.
func extractTypedefName(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "type_identifier":
		return nodeText(node, src)
	case "pointer_declarator":
		// Recurse into pointer_declarator children.
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil {
				name := extractTypedefName(child, src)
				if name != "" {
					return name
				}
			}
		}
	case "function_declarator":
		// typedef return_type (*name)(params)
		decl := node.ChildByFieldName("declarator")
		if decl != nil {
			return extractTypedefName(decl, src)
		}
	case "parenthesized_declarator":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil {
				name := extractTypedefName(child, src)
				if name != "" {
					return name
				}
			}
		}
	case "identifier":
		return nodeText(node, src)
	}
	return ""
}

// isFunctionPointerDeclarator checks if a declarator represents a function pointer.
func isFunctionPointerDeclarator(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	if node.Type() == "function_declarator" {
		return true
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && isFunctionPointerDeclarator(child) {
			return true
		}
	}
	return false
}

// extractFieldsFromBody extracts fields from a struct/union body node (field_declaration_list).
func extractFieldsFromBody(body *sitter.Node, src []byte) []fieldDef {
	var fields []fieldDef
	count := int(body.NamedChildCount())
	for i := 0; i < count; i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type() == "field_declaration" {
			fs := extractFieldDecl(child, src)
			fields = append(fields, fs...)
		}
	}
	return fields
}

// extractFieldDecl extracts one or more field definitions from a field_declaration node.
func extractFieldDecl(node *sitter.Node, src []byte) []fieldDef {
	var fields []fieldDef

	typeNode := node.ChildByFieldName("type")
	typeName := ""
	if typeNode != nil {
		typeName = nodeText(typeNode, src)
	}

	// Iterate named children to find declarators.
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "field_identifier":
			f := fieldDef{
				Name: nodeText(child, src),
				Type: typeName,
			}
			fields = append(fields, f)
		case "pointer_declarator":
			name := extractIdentifierFromNode(child, src)
			f := fieldDef{
				Name: name,
				Type: typeName + " *",
			}
			fields = append(fields, f)
		case "array_declarator":
			name := extractIdentifierFromNode(child, src)
			f := fieldDef{
				Name: name,
				Type: typeName + "[]",
			}
			fields = append(fields, f)
		case "bitfield_clause":
			// Bit field: handled as part of the preceding field.
			if len(fields) > 0 {
				fields[len(fields)-1].BitSize = nodeText(child, src)
			}
		}
	}

	// If no named fields found, it might be an anonymous struct/union.
	if len(fields) == 0 && typeName != "" {
		// Check for anonymous struct/union inline.
		if typeNode != nil && (typeNode.Type() == "struct_specifier" || typeNode.Type() == "union_specifier") {
			nameN := typeNode.ChildByFieldName("name")
			if nameN == nil {
				// Anonymous - just note it.
				fields = append(fields, fieldDef{
					Name: "(anonymous)",
					Type: typeName,
				})
			}
		}
	}

	return fields
}

// extractIdentifierFromNode recursively finds an identifier in a node subtree.
func extractIdentifierFromNode(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	if node.Type() == "identifier" || node.Type() == "field_identifier" || node.Type() == "type_identifier" {
		return nodeText(node, src)
	}
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child != nil {
			name := extractIdentifierFromNode(child, src)
			if name != "" {
				return name
			}
		}
	}
	return ""
}

// buildFunctionSignature constructs a function signature string.
func buildFunctionSignature(fn *functionDef) string {
	var sb strings.Builder
	if fn.IsStatic {
		sb.WriteString("static ")
	}
	if fn.IsInline {
		sb.WriteString("inline ")
	}
	if fn.IsExtern {
		sb.WriteString("extern ")
	}
	if fn.ReturnType != "" {
		sb.WriteString(fn.ReturnType)
		sb.WriteString(" ")
	}
	sb.WriteString(fn.Name)
	sb.WriteString("(")
	for i, p := range fn.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		if p.Type == "..." {
			sb.WriteString("...")
		} else {
			sb.WriteString(p.Type)
			if p.Name != "" {
				sb.WriteString(" ")
				sb.WriteString(p.Name)
			}
		}
	}
	sb.WriteString(")")
	return sb.String()
}

// nodeText returns the source text of a tree-sitter node.
func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	start := int(node.StartByte())
	end := int(node.EndByte())
	if start < 0 || end > len(src) || start >= end {
		return ""
	}
	return string(src[start:end])
}

// findPrecedingDocComment finds a Doxygen-style comment immediately before
// the declaration at byte position pos. Supports /** ... */ and /// formats.
func findPrecedingDocComment(content []byte, pos int) string {
	if pos <= 0 || pos > len(content) {
		return ""
	}

	i := pos - 1

	// Skip whitespace backwards.
	for i >= 0 && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n' || content[i] == '\r') {
		i--
	}

	if i < 1 {
		return ""
	}

	// Check for block comment ending with */
	if content[i] == '/' && i >= 1 && content[i-1] == '*' {
		end := i
		i -= 2
		for i >= 1 {
			if content[i] == '/' && content[i+1] == '*' {
				// Verify it is a doc comment (/** or /*!)
				if i+2 < len(content) && (content[i+2] == '*' || content[i+2] == '!') {
					return string(content[i : end+1])
				}
				return ""
			}
			i--
		}
		return ""
	}

	// Check for line comments (/// or //!)
	// Walk backwards collecting consecutive /// lines.
	lineEnd := i + 1
	var lines []string
	for {
		// Find start of current line.
		lineStart := i
		for lineStart > 0 && content[lineStart-1] != '\n' {
			lineStart--
		}

		line := strings.TrimSpace(string(content[lineStart:lineEnd]))
		if strings.HasPrefix(line, "///") || strings.HasPrefix(line, "//!") {
			lines = append([]string{line}, lines...)
		} else {
			break
		}

		// Move to previous line.
		i = lineStart - 1
		if i < 0 {
			break
		}
		// Skip \r\n.
		if i >= 0 && content[i] == '\n' {
			i--
		}
		if i >= 0 && content[i] == '\r' {
			i--
		}
		lineEnd = i + 1
	}

	if len(lines) > 0 {
		return strings.Join(lines, "\n")
	}

	return ""
}

// Regex patterns for macro extraction.
var (
	// Match function-like macros: #define NAME(params) body
	// Handles multi-line macros with backslash continuation.
	reFunctionMacro = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*define[ \t]+([A-Za-z_]\w*)\(([^)]*)\)[ \t]*(.*)`)
	// Match the doc comment preceding a #define line (/** ... */ or /// lines).
	reDocBeforeDefine = regexp.MustCompile(`(?s)(/\*\*.*?\*/|(?:[ \t]*///[^\n]*\n)+)\s*#[ \t]*define`)
)

// extractMacros extracts function-like macros from C source using regex.
// Tree-sitter does not parse preprocessor directives in a useful way.
func extractMacros(content []byte) []macroDef {
	var macros []macroDef
	src := string(content)

	matches := reFunctionMacro.FindAllStringSubmatchIndex(src, -1)
	for _, loc := range matches {
		name := src[loc[2]:loc[3]]
		params := src[loc[4]:loc[5]]
		bodyStart := loc[6]
		bodyEnd := loc[7]

		// Handle multi-line macros (backslash continuation).
		body := expandMultilineMacro(src, bodyStart, bodyEnd)

		// Parse params.
		var paramList []string
		if params != "" {
			for _, p := range strings.Split(params, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					paramList = append(paramList, p)
				}
			}
		}

		// Try to find preceding doc comment.
		docComment := findMacroDocComment(src, loc[0])

		macros = append(macros, macroDef{
			Name:       name,
			Params:     paramList,
			Body:       body,
			DocComment: docComment,
		})
	}

	return macros
}

// expandMultilineMacro expands a macro body that may span multiple lines
// via backslash continuation.
func expandMultilineMacro(src string, start, end int) string {
	if start >= len(src) || end > len(src) {
		return ""
	}

	body := src[start:end]

	// If the body ends with a backslash, collect continuation lines.
	for strings.HasSuffix(strings.TrimSpace(body), "\\") {
		// Find the next line.
		nextLineStart := end
		for nextLineStart < len(src) && src[nextLineStart] != '\n' {
			nextLineStart++
		}
		if nextLineStart >= len(src) {
			break
		}
		nextLineStart++ // skip the \n

		// Find end of next line.
		nextLineEnd := nextLineStart
		for nextLineEnd < len(src) && src[nextLineEnd] != '\n' {
			nextLineEnd++
		}

		nextLine := src[nextLineStart:nextLineEnd]
		body = strings.TrimSuffix(strings.TrimSpace(body), "\\") + " " + strings.TrimSpace(nextLine)
		end = nextLineEnd
	}

	return strings.TrimSpace(body)
}

// findMacroDocComment looks for a doc comment immediately before the #define.
func findMacroDocComment(src string, definePos int) string {
	if definePos <= 0 {
		return ""
	}

	// Look backwards from the #define position.
	i := definePos - 1
	for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i--
	}

	if i < 1 {
		return ""
	}

	// Check for block comment.
	if src[i] == '/' && i >= 1 && src[i-1] == '*' {
		end := i
		j := i - 2
		for j >= 1 {
			if src[j] == '/' && src[j+1] == '*' {
				if j+2 < len(src) && (src[j+2] == '*' || src[j+2] == '!') {
					return src[j : end+1]
				}
				return ""
			}
			j--
		}
	}

	// Check for /// line comments.
	lineEnd := i + 1
	var lines []string
	for {
		lineStart := i
		for lineStart > 0 && src[lineStart-1] != '\n' {
			lineStart--
		}
		line := strings.TrimSpace(src[lineStart:lineEnd])
		if strings.HasPrefix(line, "///") || strings.HasPrefix(line, "//!") {
			lines = append([]string{line}, lines...)
		} else {
			break
		}
		i = lineStart - 1
		if i < 0 {
			break
		}
		if i >= 0 && src[i] == '\n' {
			i--
		}
		if i >= 0 && src[i] == '\r' {
			i--
		}
		lineEnd = i + 1
	}
	if len(lines) > 0 {
		return strings.Join(lines, "\n")
	}

	return ""
}
