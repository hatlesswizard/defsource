package golang

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// fileAnalysis holds all entities extracted from a single Go source file.
type fileAnalysis struct {
	Package    string
	Structs    []structDef
	Interfaces []interfaceDef
	Functions  []functionDef
	Methods    []methodDef
	TypeAliases []typeAliasDef
	Constants  []constantDef
	Calls      []string // function/method names called in this file
}

// structDef holds data from a type_declaration with struct type.
type structDef struct {
	Name       string
	StartPos   int
	EndPos     int
	DocComment string
	Fields     []fieldDef
	Embedded   []string // embedded type names
	TypeParams string   // generic type parameters, e.g. "[T any, U comparable]"
}

// fieldDef holds a single struct field.
type fieldDef struct {
	Name     string
	Type     string
	Tag      string
	Embedded bool
}

// interfaceDef holds data from a type_declaration with interface type.
type interfaceDef struct {
	Name       string
	StartPos   int
	EndPos     int
	DocComment string
	Methods    []interfaceMethodDef
	Embedded   []string // embedded interface names
	TypeParams string
}

// interfaceMethodDef holds a method signature in an interface.
type interfaceMethodDef struct {
	Name      string
	Signature string
}

// functionDef holds a package-level function definition.
type functionDef struct {
	Name       string
	StartPos   int
	EndPos     int
	DocComment string
	Params     []paramDef
	ReturnType string
	TypeParams string
}

// methodDef holds a method (function with receiver) definition.
type methodDef struct {
	Name            string
	ReceiverName    string
	ReceiverType    string
	PointerReceiver bool
	StartPos        int
	EndPos          int
	DocComment      string
	Params          []paramDef
	ReturnType      string
	TypeParams      string
}

// paramDef holds a single function/method parameter.
type paramDef struct {
	Name     string
	Type     string
	Variadic bool
}

// typeAliasDef holds a type alias or named type.
type typeAliasDef struct {
	Name        string
	UnderlyingType string
	StartPos    int
	EndPos      int
	DocComment  string
	TypeParams  string
}

// constantDef holds a constant declaration.
type constantDef struct {
	Name       string
	Type       string
	Value      string
	StartPos   int
	EndPos     int
	DocComment string
}

// parseFile parses a Go source file using tree-sitter and extracts all entities.
func parseFile(content []byte) *fileAnalysis {
	parser, err := treesitter.Get(treesitter.Go)
	if err != nil {
		return nil
	}
	defer treesitter.Put(treesitter.Go, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil
	}
	defer tree.Close()

	root := tree.RootNode()
	analysis := &fileAnalysis{}

	walkNode(root, content, analysis)

	return analysis
}

// walkNode recursively walks the AST tree extracting entities.
func walkNode(node *sitter.Node, content []byte, analysis *fileAnalysis) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "package_clause":
		analysis.Package = extractPackageName(node, content)

	case "type_declaration":
		extractTypeDeclaration(node, content, analysis)

	case "function_declaration":
		fn := extractFunction(node, content)
		if fn != nil {
			analysis.Functions = append(analysis.Functions, *fn)
		}

	case "method_declaration":
		m := extractMethod(node, content)
		if m != nil {
			analysis.Methods = append(analysis.Methods, *m)
		}

	case "const_declaration":
		consts := extractConstants(node, content)
		analysis.Constants = append(analysis.Constants, consts...)

	case "call_expression":
		if name := extractCallName(node, content); name != "" {
			analysis.Calls = append(analysis.Calls, name)
		}
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		walkNode(child, content, analysis)
	}
}

// extractPackageName gets the package name from a package_clause node.
func extractPackageName(node *sitter.Node, content []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "package_identifier" {
			return nodeText(child, content)
		}
	}
	return ""
}

// extractTypeDeclaration extracts struct, interface, or type alias definitions.
func extractTypeDeclaration(node *sitter.Node, content []byte, analysis *fileAnalysis) {
	docComment := findPrecedingComment(node, content)

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_spec" {
			extractTypeSpec(child, content, docComment, node, analysis)
		}
	}
}

// extractTypeSpec processes a single type_spec within a type_declaration.
func extractTypeSpec(node *sitter.Node, content []byte, docComment string, parent *sitter.Node, analysis *fileAnalysis) {
	var name string
	var typeParams string
	var typeNode *sitter.Node

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "type_identifier":
			name = nodeText(child, content)
		case "type_parameter_list":
			typeParams = nodeText(child, content)
		case "struct_type":
			typeNode = child
		case "interface_type":
			typeNode = child
		}
	}

	if name == "" {
		return
	}

	// Use the type_spec's own doc comment if no parent doc is found
	specDoc := findPrecedingComment(node, content)
	if specDoc != "" {
		docComment = specDoc
	}

	if typeNode == nil {
		// Type alias or named type
		underlying := extractUnderlyingType(node, content, name, typeParams)
		analysis.TypeAliases = append(analysis.TypeAliases, typeAliasDef{
			Name:           name,
			UnderlyingType: underlying,
			StartPos:       int(parent.StartByte()),
			EndPos:         int(parent.EndByte()),
			DocComment:     docComment,
			TypeParams:     typeParams,
		})
		return
	}

	switch typeNode.Type() {
	case "struct_type":
		st := extractStruct(name, typeNode, content, docComment, parent, typeParams)
		analysis.Structs = append(analysis.Structs, st)
	case "interface_type":
		iface := extractInterface(name, typeNode, content, docComment, parent, typeParams)
		analysis.Interfaces = append(analysis.Interfaces, iface)
	}
}

// extractStruct builds a structDef from a struct_type AST node.
func extractStruct(name string, typeNode *sitter.Node, content []byte, docComment string, parent *sitter.Node, typeParams string) structDef {
	st := structDef{
		Name:       name,
		StartPos:   int(parent.StartByte()),
		EndPos:     int(parent.EndByte()),
		DocComment: docComment,
		TypeParams: typeParams,
	}

	// Walk field_declaration_list
	for i := 0; i < int(typeNode.ChildCount()); i++ {
		child := typeNode.Child(i)
		if child.Type() == "field_declaration_list" {
			extractStructFields(child, content, &st)
			break
		}
	}

	return st
}

// extractStructFields populates struct fields from a field_declaration_list.
func extractStructFields(listNode *sitter.Node, content []byte, st *structDef) {
	for i := 0; i < int(listNode.ChildCount()); i++ {
		child := listNode.Child(i)
		if child.Type() != "field_declaration" {
			continue
		}
		fields := extractFieldDeclaration(child, content)
		for _, f := range fields {
			if f.Embedded {
				st.Embedded = append(st.Embedded, f.Type)
			}
			st.Fields = append(st.Fields, f)
		}
	}
}

// extractFieldDeclaration extracts one or more fields from a field_declaration node.
func extractFieldDeclaration(node *sitter.Node, content []byte) []fieldDef {
	var names []string
	var fieldType string
	var tag string
	isEmbedded := true

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "field_identifier":
			names = append(names, nodeText(child, content))
			isEmbedded = false
		case "tag":
			tag = nodeText(child, content)
		default:
			// Remaining node types are the field type
			if fieldType == "" && child.Type() != "," && child.Type() != "comment" {
				fieldType = nodeText(child, content)
			}
		}
	}

	if isEmbedded && fieldType != "" {
		// Embedded field: type name only, strip pointer
		typeName := strings.TrimPrefix(fieldType, "*")
		// Strip package prefix for embedded type
		if idx := strings.LastIndex(typeName, "."); idx >= 0 {
			typeName = typeName[idx+1:]
		}
		return []fieldDef{{
			Name:     "",
			Type:     fieldType,
			Embedded: true,
		}}
	}

	var fields []fieldDef
	for _, name := range names {
		fields = append(fields, fieldDef{
			Name: name,
			Type: fieldType,
			Tag:  tag,
		})
	}
	return fields
}

// extractInterface builds an interfaceDef from an interface_type AST node.
func extractInterface(name string, typeNode *sitter.Node, content []byte, docComment string, parent *sitter.Node, typeParams string) interfaceDef {
	iface := interfaceDef{
		Name:       name,
		StartPos:   int(parent.StartByte()),
		EndPos:     int(parent.EndByte()),
		DocComment: docComment,
		TypeParams: typeParams,
	}

	for i := 0; i < int(typeNode.ChildCount()); i++ {
		child := typeNode.Child(i)
		switch child.Type() {
		case "method_spec", "method_elem":
			m := extractInterfaceMethod(child, content)
			if m != nil {
				iface.Methods = append(iface.Methods, *m)
			}
		case "type_identifier", "qualified_type":
			// Embedded interface
			iface.Embedded = append(iface.Embedded, nodeText(child, content))
		case "struct_elem", "constraint_elem", "type_elem":
			// Type constraint element (Go 1.18+), treat as embedded
			iface.Embedded = append(iface.Embedded, nodeText(child, content))
		}
	}

	return iface
}

// extractInterfaceMethod extracts a method signature from a method_spec node.
func extractInterfaceMethod(node *sitter.Node, content []byte) *interfaceMethodDef {
	var name string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "field_identifier" {
			name = nodeText(child, content)
			break
		}
	}
	if name == "" {
		return nil
	}
	return &interfaceMethodDef{
		Name:      name,
		Signature: nodeText(node, content),
	}
}

// extractFunction extracts a package-level function definition.
func extractFunction(node *sitter.Node, content []byte) *functionDef {
	fn := &functionDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		DocComment: findPrecedingComment(node, content),
	}

	paramListSeen := false
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			fn.Name = nodeText(child, content)
		case "type_parameter_list":
			fn.TypeParams = nodeText(child, content)
		case "parameter_list":
			if !paramListSeen {
				// First parameter_list is the function parameters
				fn.Params = extractParams(child, content)
				paramListSeen = true
			} else {
				// Second parameter_list is the result (multiple returns)
				fn.ReturnType = nodeText(child, content)
			}
		case "result":
			fn.ReturnType = extractResult(child, content)
		// Simple return types appear as direct children
		case "type_identifier", "pointer_type", "slice_type", "map_type",
			"interface_type", "struct_type", "array_type", "channel_type",
			"function_type", "qualified_type", "generic_type":
			if paramListSeen && fn.ReturnType == "" {
				fn.ReturnType = nodeText(child, content)
			}
		}
	}

	if fn.Name == "" {
		return nil
	}
	return fn
}

// extractMethod extracts a method (function with receiver) definition.
func extractMethod(node *sitter.Node, content []byte) *methodDef {
	m := &methodDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		DocComment: findPrecedingComment(node, content),
	}

	paramListCount := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "parameter_list":
			paramListCount++
			switch paramListCount {
			case 1:
				// First parameter_list is receiver
				extractReceiver(child, content, m)
			case 2:
				// Second parameter_list is params
				m.Params = extractParams(child, content)
			case 3:
				// Third parameter_list is multiple returns
				m.ReturnType = nodeText(child, content)
			}
		case "field_identifier":
			m.Name = nodeText(child, content)
		case "type_parameter_list":
			m.TypeParams = nodeText(child, content)
		case "result":
			m.ReturnType = extractResult(child, content)
		// Simple return types appear as direct children
		case "type_identifier", "pointer_type", "slice_type", "map_type",
			"interface_type", "struct_type", "array_type", "channel_type",
			"function_type", "qualified_type", "generic_type":
			if paramListCount >= 2 && m.ReturnType == "" {
				m.ReturnType = nodeText(child, content)
			}
		}
	}

	if m.Name == "" || m.ReceiverType == "" {
		return nil
	}
	return m
}

// extractReceiver extracts receiver information from the parameter_list.
func extractReceiver(node *sitter.Node, content []byte, m *methodDef) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "parameter_declaration" {
			extractReceiverParam(child, content, m)
			return
		}
	}
}

// extractReceiverParam extracts the receiver name and type from a parameter_declaration.
func extractReceiverParam(node *sitter.Node, content []byte, m *methodDef) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			m.ReceiverName = nodeText(child, content)
		case "pointer_type":
			m.PointerReceiver = true
			m.ReceiverType = extractTypeIdentifier(child, content)
		case "type_identifier":
			m.ReceiverType = nodeText(child, content)
		case "generic_type":
			m.ReceiverType = extractGenericTypeName(child, content)
		}
	}
}

// extractGenericTypeName extracts the base type name from a generic_type node.
func extractGenericTypeName(node *sitter.Node, content []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			return nodeText(child, content)
		}
	}
	return nodeText(node, content)
}

// extractTypeIdentifier extracts a type name from within a pointer_type.
func extractTypeIdentifier(node *sitter.Node, content []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "type_identifier":
			return nodeText(child, content)
		case "generic_type":
			return extractGenericTypeName(child, content)
		}
	}
	return ""
}

// extractParams extracts parameter definitions from a parameter_list.
func extractParams(node *sitter.Node, content []byte) []paramDef {
	var params []paramDef

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "parameter_declaration" || child.Type() == "variadic_parameter_declaration" {
			ps := extractParamDeclaration(child, content)
			params = append(params, ps...)
		}
	}

	return params
}

// extractParamDeclaration extracts parameters from a single parameter_declaration.
func extractParamDeclaration(node *sitter.Node, content []byte) []paramDef {
	var names []string
	var paramType string
	variadic := node.Type() == "variadic_parameter_declaration"

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			names = append(names, nodeText(child, content))
		case "variadic_argument":
			variadic = true
			paramType = extractVariadicType(child, content)
		default:
			text := nodeText(child, content)
			// Skip punctuation and the "..." token
			if paramType == "" && child.Type() != "," && text != "..." {
				paramType = text
			}
		}
	}

	// Handle case where there's only a type (unnamed parameter)
	if len(names) == 0 && paramType != "" {
		return []paramDef{{Type: paramType, Variadic: variadic}}
	}

	var params []paramDef
	for _, name := range names {
		params = append(params, paramDef{
			Name:     name,
			Type:     paramType,
			Variadic: variadic,
		})
	}
	return params
}

// extractVariadicType extracts the type from a variadic_argument node.
func extractVariadicType(node *sitter.Node, content []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() != "..." {
			return nodeText(child, content)
		}
	}
	return ""
}

// extractResult extracts the return type from a result node.
func extractResult(node *sitter.Node, content []byte) string {
	// Result can be a parameter_list (multiple returns) or a single type
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "parameter_list" {
			// Multiple returns: (Type1, Type2, ...)
			return nodeText(child, content)
		}
		// Single return type
		return nodeText(child, content)
	}
	return nodeText(node, content)
}

// extractConstants extracts constant declarations.
func extractConstants(node *sitter.Node, content []byte) []constantDef {
	var consts []constantDef
	docComment := findPrecedingComment(node, content)

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "const_spec" {
			c := extractConstSpec(child, content, docComment, node)
			if c != nil {
				consts = append(consts, *c)
			}
		}
	}

	return consts
}

// extractConstSpec extracts a single const_spec.
func extractConstSpec(node *sitter.Node, content []byte, parentDoc string, parent *sitter.Node) *constantDef {
	c := &constantDef{
		StartPos:   int(parent.StartByte()),
		EndPos:     int(parent.EndByte()),
		DocComment: parentDoc,
	}

	// Check for individual spec doc comment
	specDoc := findPrecedingComment(node, content)
	if specDoc != "" {
		c.DocComment = specDoc
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			if c.Name == "" {
				c.Name = nodeText(child, content)
			}
		case "type_identifier":
			c.Type = nodeText(child, content)
		case "expression_list":
			c.Value = nodeText(child, content)
		}
	}

	if c.Name == "" {
		return nil
	}
	return c
}

// extractUnderlyingType extracts the underlying type from a type_spec
// that is not a struct or interface.
func extractUnderlyingType(node *sitter.Node, content []byte, name, typeParams string) string {
	text := nodeText(node, content)
	// Remove the name and type params prefix
	prefix := name
	if typeParams != "" {
		prefix += typeParams
	}
	text = strings.TrimPrefix(text, prefix)
	text = strings.TrimSpace(text)
	// Remove "=" for type aliases
	text = strings.TrimPrefix(text, "=")
	text = strings.TrimSpace(text)
	return text
}

// extractCallName extracts the function/method name from a call_expression.
func extractCallName(node *sitter.Node, content []byte) string {
	if node.ChildCount() == 0 {
		return ""
	}
	fn := node.Child(0)
	switch fn.Type() {
	case "identifier":
		return nodeText(fn, content)
	case "selector_expression":
		// pkg.Function or receiver.Method
		for i := 0; i < int(fn.ChildCount()); i++ {
			child := fn.Child(i)
			if child.Type() == "field_identifier" {
				return nodeText(child, content)
			}
		}
	}
	return ""
}

// findPrecedingComment finds the doc comment preceding a node.
func findPrecedingComment(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	startByte := int(node.StartByte())
	if startByte == 0 {
		return ""
	}

	// Look for comment nodes that immediately precede this node.
	// Walk backwards from the node start to find contiguous comment lines.
	prev := node.PrevSibling()
	if prev == nil {
		// Try parent's children
		return extractCommentFromBytes(content, startByte)
	}

	// Check if previous sibling is a comment
	if prev.Type() == "comment" {
		return collectCommentGroup(prev, content)
	}

	return extractCommentFromBytes(content, startByte)
}

// collectCommentGroup collects a contiguous group of // comments ending at node.
func collectCommentGroup(lastComment *sitter.Node, content []byte) string {
	var comments []string
	node := lastComment

	for node != nil && node.Type() == "comment" {
		text := nodeText(node, content)
		comments = append([]string{text}, comments...)
		node = node.PrevSibling()
	}

	if len(comments) == 0 {
		return ""
	}
	return strings.Join(comments, "\n")
}

// extractCommentFromBytes extracts comment text by scanning backwards from pos.
func extractCommentFromBytes(content []byte, pos int) string {
	if pos <= 0 || pos > len(content) {
		return ""
	}

	// Walk backwards over whitespace
	i := pos - 1
	for i >= 0 && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n' || content[i] == '\r') {
		i--
	}

	if i < 1 {
		return ""
	}

	// Check if we're at the end of a line comment or block comment
	if content[i] == '/' && i >= 1 && content[i-1] == '/' {
		// End of a // comment on the same line? Unusual. Look for \n before.
		return ""
	}

	// Look for end of block comment
	if content[i] == '/' && i >= 1 && content[i-1] == '*' {
		// Block comment ending with */
		end := i + 1
		i -= 2
		for i >= 1 {
			if content[i] == '/' && content[i+1] == '*' {
				return string(content[i:end])
			}
			i--
		}
		return ""
	}

	// Check if previous line ends with a line comment
	// Scan back to find the end of the previous line
	lineEnd := i
	for lineEnd >= 0 && content[lineEnd] != '\n' {
		lineEnd--
	}

	if lineEnd < 0 {
		return ""
	}

	// Now check if there are // comments above pos
	var commentLines []string
	scanPos := pos - 1
	for scanPos >= 0 {
		// Skip trailing whitespace
		for scanPos >= 0 && (content[scanPos] == ' ' || content[scanPos] == '\t' || content[scanPos] == '\r') {
			scanPos--
		}
		if scanPos < 0 || content[scanPos] != '\n' {
			break
		}
		scanPos-- // skip the \n

		// Find the start of this line
		lineStart := scanPos
		for lineStart >= 0 && content[lineStart] != '\n' {
			lineStart--
		}
		lineStart++

		line := strings.TrimSpace(string(content[lineStart : scanPos+1]))
		if strings.HasPrefix(line, "//") {
			commentLines = append([]string{line}, commentLines...)
			scanPos = lineStart - 1
		} else {
			break
		}
	}

	if len(commentLines) == 0 {
		return ""
	}
	return strings.Join(commentLines, "\n")
}

// nodeText returns the source text for a tree-sitter node.
func nodeText(node *sitter.Node, content []byte) string {
	start := int(node.StartByte())
	end := int(node.EndByte())
	if start < 0 || end > len(content) || start >= end {
		return ""
	}
	return string(content[start:end])
}
