package typescript

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// fileAnalysis holds all entities extracted from one TypeScript source file.
type fileAnalysis struct {
	Classes    []classDef
	Interfaces []interfaceDef
	Functions  []functionDef
	TypeAliases []typeAliasDef
	Enums      []enumDef
	Namespaces []namespaceDef
	ReExports  []reExportDef
	References []string // names referenced (for refcount)
}

// classDef holds data extracted from a class declaration.
type classDef struct {
	Name        string
	Exported    bool
	Abstract    bool
	Extends     string
	Implements  []string
	TypeParams  string
	Decorators  []decoratorDef
	Methods     []methodDef
	Properties  []propertyDef
	DocComment  string
	StartPos    int
	EndPos      int
}

// interfaceDef holds data extracted from an interface declaration.
type interfaceDef struct {
	Name        string
	Exported    bool
	Extends     []string
	TypeParams  string
	Methods     []methodDef
	Properties  []propertyDef
	DocComment  string
	StartPos    int
	EndPos      int
}

// functionDef holds data extracted from a function declaration.
type functionDef struct {
	Name        string
	Exported    bool
	Async       bool
	TypeParams  string
	Params      []paramDef
	ReturnType  string
	Overloads   []string // overload signatures
	DocComment  string
	StartPos    int
	EndPos      int
}

// typeAliasDef holds data extracted from a type alias declaration.
type typeAliasDef struct {
	Name        string
	Exported    bool
	TypeParams  string
	Definition  string
	DocComment  string
	StartPos    int
	EndPos      int
}

// enumDef holds data extracted from an enum declaration.
type enumDef struct {
	Name       string
	Exported   bool
	Const      bool
	Members    []enumMemberDef
	DocComment string
	StartPos   int
	EndPos     int
}

// enumMemberDef holds a single enum member.
type enumMemberDef struct {
	Name  string
	Value string
}

// namespaceDef holds data extracted from a namespace/module declaration.
type namespaceDef struct {
	Name       string
	Exported   bool
	DocComment string
	StartPos   int
	EndPos     int
}

// methodDef holds data extracted from a method/function signature within a
// class or interface.
type methodDef struct {
	Name       string
	Static     bool
	Async      bool
	Abstract   bool
	Visibility string // "public", "private", "protected"
	Readonly   bool
	Optional   bool
	TypeParams string
	Params     []paramDef
	ReturnType string
	Decorators []decoratorDef
	DocComment string
	StartPos   int
	EndPos     int
}

// propertyDef holds data extracted from a property declaration.
type propertyDef struct {
	Name       string
	Type       string
	Visibility string
	Static     bool
	Readonly   bool
	Optional   bool
	Decorators []decoratorDef
	DocComment string
}

// paramDef holds data for a single function/method parameter.
type paramDef struct {
	Name     string
	Type     string
	Optional bool
	Rest     bool
	Default  string
}

// decoratorDef holds data for a decorator.
type decoratorDef struct {
	Name      string
	Arguments string
}

// reExportDef represents a re-export statement.
type reExportDef struct {
	Name       string
	ModulePath string
}

// parseFile parses a TypeScript source file using tree-sitter and returns
// the collected analysis. Thread-safe via the shared parser pool.
func parseFile(src []byte, filePath string) *fileAnalysis {
	// Determine if we need TSX parser based on file extension
	lang := treesitter.TypeScript

	parser, err := treesitter.Get(lang)
	if err != nil {
		return &fileAnalysis{}
	}
	defer treesitter.Put(lang, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return &fileAnalysis{}
	}
	defer tree.Close()

	result := &fileAnalysis{}
	root := tree.RootNode()
	walkNode(root, src, result, false)
	return result
}

// walkNode recursively walks the AST and collects TypeScript entities.
func walkNode(node *sitter.Node, src []byte, result *fileAnalysis, exported bool) {
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
		case "export_statement":
			processExportStatement(child, src, result)

		case "class_declaration":
			cls := extractClass(child, src)
			cls.Exported = exported
			result.Classes = append(result.Classes, cls)

		case "abstract_class_declaration":
			cls := extractClass(child, src)
			cls.Exported = exported
			cls.Abstract = true
			result.Classes = append(result.Classes, cls)

		case "interface_declaration":
			iface := extractInterface(child, src)
			iface.Exported = exported
			result.Interfaces = append(result.Interfaces, iface)

		case "function_declaration":
			fn := extractFunction(child, src)
			fn.Exported = exported
			result.Functions = append(result.Functions, fn)

		case "type_alias_declaration":
			ta := extractTypeAlias(child, src)
			ta.Exported = exported
			result.TypeAliases = append(result.TypeAliases, ta)

		case "enum_declaration":
			en := extractEnum(child, src)
			en.Exported = exported
			result.Enums = append(result.Enums, en)

		case "module", "internal_module":
			ns := extractNamespace(child, src)
			ns.Exported = exported
			result.Namespaces = append(result.Namespaces, ns)

		case "lexical_declaration":
			// Handle: export const foo = ...
			// Variable declarations that export arrow functions or class expressions
			if exported {
				extractLexicalDeclaration(child, src, result)
			}

		case "expression_statement":
			// Walk for references
			walkReferences(child, src, result)

		case "import_statement":
			// Extract imported names as references
			extractImportReferences(child, src, result)

		default:
			// Recurse into other statement-level nodes
			if isStatementContainer(child.Type()) {
				walkNode(child, src, result, false)
			}
		}
	}
}

// isStatementContainer returns true if the node type can contain statements.
func isStatementContainer(nodeType string) bool {
	switch nodeType {
	case "program", "statement_block", "if_statement", "else_clause",
		"for_statement", "for_in_statement", "while_statement",
		"do_statement", "switch_body", "switch_case", "switch_default":
		return true
	}
	return false
}

// processExportStatement handles an export_statement node, extracting
// exported declarations and re-exports.
func processExportStatement(node *sitter.Node, src []byte, result *fileAnalysis) {
	count := int(node.ChildCount())

	// Check for re-exports: export { X } from './module'
	// or: export * from './module'
	var hasFrom bool
	var modulePath string
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "string" || child.Type() == "string_fragment" {
			modulePath = unquote(nodeText(child, src))
			hasFrom = true
		}
		// Look for "from" keyword followed by string
		if nodeText(child, src) == "from" && i+1 < count {
			next := node.Child(i + 1)
			if next != nil && (next.Type() == "string" || next.Type() == "string_fragment") {
				modulePath = unquote(nodeText(next, src))
				hasFrom = true
			}
		}
	}

	if hasFrom {
		// This is a re-export
		for i := 0; i < count; i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			if child.Type() == "export_clause" || child.Type() == "named_exports" {
				extractReExportNames(child, src, modulePath, result)
			}
		}
		return
	}

	// Check for exported declarations
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "class_declaration":
			cls := extractClass(child, src)
			cls.Exported = true
			cls.DocComment = findPrecedingDoc(src, int(node.StartByte()))
			if cls.DocComment == "" {
				cls.DocComment = findPrecedingDoc(src, int(child.StartByte()))
			}
			result.Classes = append(result.Classes, cls)

		case "abstract_class_declaration":
			cls := extractClass(child, src)
			cls.Exported = true
			cls.Abstract = true
			cls.DocComment = findPrecedingDoc(src, int(node.StartByte()))
			if cls.DocComment == "" {
				cls.DocComment = findPrecedingDoc(src, int(child.StartByte()))
			}
			result.Classes = append(result.Classes, cls)

		case "interface_declaration":
			iface := extractInterface(child, src)
			iface.Exported = true
			iface.DocComment = findPrecedingDoc(src, int(node.StartByte()))
			if iface.DocComment == "" {
				iface.DocComment = findPrecedingDoc(src, int(child.StartByte()))
			}
			result.Interfaces = append(result.Interfaces, iface)

		case "function_declaration":
			fn := extractFunction(child, src)
			fn.Exported = true
			fn.DocComment = findPrecedingDoc(src, int(node.StartByte()))
			if fn.DocComment == "" {
				fn.DocComment = findPrecedingDoc(src, int(child.StartByte()))
			}
			result.Functions = append(result.Functions, fn)

		case "type_alias_declaration":
			ta := extractTypeAlias(child, src)
			ta.Exported = true
			ta.DocComment = findPrecedingDoc(src, int(node.StartByte()))
			if ta.DocComment == "" {
				ta.DocComment = findPrecedingDoc(src, int(child.StartByte()))
			}
			result.TypeAliases = append(result.TypeAliases, ta)

		case "enum_declaration":
			en := extractEnum(child, src)
			en.Exported = true
			en.DocComment = findPrecedingDoc(src, int(node.StartByte()))
			if en.DocComment == "" {
				en.DocComment = findPrecedingDoc(src, int(child.StartByte()))
			}
			result.Enums = append(result.Enums, en)

		case "module", "internal_module":
			ns := extractNamespace(child, src)
			ns.Exported = true
			result.Namespaces = append(result.Namespaces, ns)

		case "lexical_declaration":
			extractLexicalDeclaration(child, src, result)

		case "export_clause", "named_exports":
			// export { X, Y } -- already in scope
			extractExportClauseNames(child, src, result)
		}
	}
}

// extractClass extracts a class definition from an AST node.
func extractClass(node *sitter.Node, src []byte) classDef {
	cls := classDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	// Get doc comment
	cls.DocComment = findPrecedingDoc(src, cls.StartPos)

	// Get decorators
	cls.Decorators = extractDecorators(node, src)

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "type_identifier", "identifier":
			if cls.Name == "" {
				cls.Name = nodeText(child, src)
			}
		case "type_parameters":
			cls.TypeParams = nodeText(child, src)
			// Strip the angle brackets
			cls.TypeParams = strings.TrimPrefix(cls.TypeParams, "<")
			cls.TypeParams = strings.TrimSuffix(cls.TypeParams, ">")
		case "class_heritage":
			extractClassHeritage(child, src, &cls)
		case "class_body":
			extractClassBody(child, src, &cls)
		}
	}

	return cls
}

// extractClassHeritage parses extends and implements clauses.
// The tree-sitter TypeScript grammar structures class_heritage as:
//   class_heritage -> extends_clause -> "extends" identifier
//                  -> implements_clause -> "implements" type_identifier
func extractClassHeritage(node *sitter.Node, src []byte, cls *classDef) {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "extends_clause":
			// extends_clause children: "extends" keyword, then type identifiers
			clauseCount := int(child.ChildCount())
			for j := 0; j < clauseCount; j++ {
				clauseChild := child.Child(j)
				if clauseChild == nil {
					continue
				}
				if clauseChild.Type() == "identifier" || clauseChild.Type() == "type_identifier" ||
					clauseChild.Type() == "generic_type" || clauseChild.Type() == "member_expression" {
					if cls.Extends == "" {
						cls.Extends = nodeText(clauseChild, src)
					}
				}
			}

		case "implements_clause":
			// implements_clause children: "implements" keyword, then type identifiers
			clauseCount := int(child.ChildCount())
			for j := 0; j < clauseCount; j++ {
				clauseChild := child.Child(j)
				if clauseChild == nil {
					continue
				}
				if clauseChild.Type() == "identifier" || clauseChild.Type() == "type_identifier" ||
					clauseChild.Type() == "generic_type" || clauseChild.Type() == "member_expression" {
					cls.Implements = append(cls.Implements, nodeText(clauseChild, src))
				}
			}
		}
	}
}

// extractClassBody extracts methods and properties from a class body.
func extractClassBody(node *sitter.Node, src []byte, cls *classDef) {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "method_definition":
			m := extractMethod(child, src)
			cls.Methods = append(cls.Methods, m)
		case "public_field_definition", "property_declaration":
			p := extractProperty(child, src)
			cls.Properties = append(cls.Properties, p)
		case "method_signature":
			m := extractMethodSignature(child, src)
			cls.Methods = append(cls.Methods, m)
		}
	}
}

// extractInterface extracts an interface definition from an AST node.
func extractInterface(node *sitter.Node, src []byte) interfaceDef {
	iface := interfaceDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	iface.DocComment = findPrecedingDoc(src, iface.StartPos)

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "type_identifier", "identifier":
			if iface.Name == "" {
				iface.Name = nodeText(child, src)
			}
		case "type_parameters":
			iface.TypeParams = nodeText(child, src)
			iface.TypeParams = strings.TrimPrefix(iface.TypeParams, "<")
			iface.TypeParams = strings.TrimSuffix(iface.TypeParams, ">")
		case "extends_type_clause":
			extractExtendsClause(child, src, &iface)
		case "interface_body", "object_type":
			extractInterfaceBody(child, src, &iface)
		}
	}

	return iface
}

// extractExtendsClause parses an interface's extends clause.
func extractExtendsClause(node *sitter.Node, src []byte, iface *interfaceDef) {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "type_identifier" || child.Type() == "generic_type" ||
			child.Type() == "member_expression" {
			iface.Extends = append(iface.Extends, nodeText(child, src))
		}
	}
}

// extractInterfaceBody extracts methods and properties from an interface body.
func extractInterfaceBody(node *sitter.Node, src []byte, iface *interfaceDef) {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "method_signature":
			m := extractMethodSignature(child, src)
			iface.Methods = append(iface.Methods, m)
		case "property_signature":
			p := extractPropertySignature(child, src)
			iface.Properties = append(iface.Properties, p)
		case "call_signature":
			// Interface with call signatures -- treat as method
			m := extractCallSignature(child, src)
			iface.Methods = append(iface.Methods, m)
		case "index_signature":
			p := extractIndexSignature(child, src)
			iface.Properties = append(iface.Properties, p)
		}
	}
}

// extractFunction extracts a function definition from an AST node.
func extractFunction(node *sitter.Node, src []byte) functionDef {
	fn := functionDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	fn.DocComment = findPrecedingDoc(src, fn.StartPos)

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			if fn.Name == "" {
				fn.Name = nodeText(child, src)
			}
		case "type_parameters":
			fn.TypeParams = nodeText(child, src)
			fn.TypeParams = strings.TrimPrefix(fn.TypeParams, "<")
			fn.TypeParams = strings.TrimSuffix(fn.TypeParams, ">")
		case "formal_parameters":
			fn.Params = extractParams(child, src)
		case "type_annotation":
			fn.ReturnType = extractTypeAnnotation(child, src)
		}

		text := nodeText(child, src)
		if text == "async" {
			fn.Async = true
		}
	}

	return fn
}

// extractTypeAlias extracts a type alias definition from an AST node.
func extractTypeAlias(node *sitter.Node, src []byte) typeAliasDef {
	ta := typeAliasDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	ta.DocComment = findPrecedingDoc(src, ta.StartPos)

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "type_identifier", "identifier":
			if ta.Name == "" {
				ta.Name = nodeText(child, src)
			}
		case "type_parameters":
			ta.TypeParams = nodeText(child, src)
			ta.TypeParams = strings.TrimPrefix(ta.TypeParams, "<")
			ta.TypeParams = strings.TrimSuffix(ta.TypeParams, ">")
		default:
			// The type value comes after the "=" sign
			if ta.Name != "" && ta.Definition == "" &&
				child.Type() != "=" && nodeText(child, src) != "=" &&
				nodeText(child, src) != "type" {
				if isTypeNode(child.Type()) {
					ta.Definition = nodeText(child, src)
				}
			}
		}
	}

	return ta
}

// extractEnum extracts an enum definition from an AST node.
func extractEnum(node *sitter.Node, src []byte) enumDef {
	en := enumDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	en.DocComment = findPrecedingDoc(src, en.StartPos)

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		text := nodeText(child, src)
		if text == "const" {
			en.Const = true
		}

		switch child.Type() {
		case "identifier":
			if en.Name == "" {
				en.Name = nodeText(child, src)
			}
		case "enum_body":
			extractEnumBody(child, src, &en)
		}
	}

	return en
}

// extractEnumBody extracts members from an enum body.
func extractEnumBody(node *sitter.Node, src []byte, en *enumDef) {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "enum_assignment" || child.Type() == "property_identifier" ||
			child.Type() == "identifier" {
			member := extractEnumMember(child, src)
			if member.Name != "" {
				en.Members = append(en.Members, member)
			}
		}
	}
}

// extractEnumMember extracts a single enum member.
func extractEnumMember(node *sitter.Node, src []byte) enumMemberDef {
	member := enumMemberDef{}

	if node.Type() == "property_identifier" || node.Type() == "identifier" {
		member.Name = nodeText(node, src)
		return member
	}

	// enum_assignment: name = value
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if (child.Type() == "property_identifier" || child.Type() == "identifier") && member.Name == "" {
			member.Name = nodeText(child, src)
		} else if member.Name != "" && nodeText(child, src) != "=" {
			member.Value = nodeText(child, src)
		}
	}
	return member
}

// extractNamespace extracts a namespace/module definition from an AST node.
func extractNamespace(node *sitter.Node, src []byte) namespaceDef {
	ns := namespaceDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	ns.DocComment = findPrecedingDoc(src, ns.StartPos)

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "identifier" || child.Type() == "string" {
			if ns.Name == "" {
				ns.Name = unquote(nodeText(child, src))
			}
		}
	}

	return ns
}

// extractMethod extracts a method from a method_definition node.
func extractMethod(node *sitter.Node, src []byte) methodDef {
	m := methodDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		Visibility: "public",
	}

	m.DocComment = findPrecedingDoc(src, m.StartPos)
	m.Decorators = extractDecorators(node, src)

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		text := nodeText(child, src)
		switch text {
		case "static":
			m.Static = true
		case "async":
			m.Async = true
		case "abstract":
			m.Abstract = true
		case "public":
			m.Visibility = "public"
		case "private":
			m.Visibility = "private"
		case "protected":
			m.Visibility = "protected"
		case "readonly":
			m.Readonly = true
		}

		switch child.Type() {
		case "property_identifier", "identifier":
			if m.Name == "" {
				m.Name = text
			}
		case "computed_property_name":
			if m.Name == "" {
				m.Name = text
			}
		case "type_parameters":
			m.TypeParams = text
			m.TypeParams = strings.TrimPrefix(m.TypeParams, "<")
			m.TypeParams = strings.TrimSuffix(m.TypeParams, ">")
		case "formal_parameters":
			m.Params = extractParams(child, src)
		case "type_annotation":
			m.ReturnType = extractTypeAnnotation(child, src)
		}
	}

	return m
}

// extractMethodSignature extracts a method from a method_signature node.
func extractMethodSignature(node *sitter.Node, src []byte) methodDef {
	m := methodDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		Visibility: "public",
	}

	m.DocComment = findPrecedingDoc(src, m.StartPos)

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		text := nodeText(child, src)
		switch text {
		case "readonly":
			m.Readonly = true
		case "?":
			m.Optional = true
		}

		switch child.Type() {
		case "property_identifier", "identifier":
			if m.Name == "" {
				m.Name = text
			}
		case "type_parameters":
			m.TypeParams = text
			m.TypeParams = strings.TrimPrefix(m.TypeParams, "<")
			m.TypeParams = strings.TrimSuffix(m.TypeParams, ">")
		case "formal_parameters":
			m.Params = extractParams(child, src)
		case "type_annotation":
			m.ReturnType = extractTypeAnnotation(child, src)
		}
	}

	return m
}

// extractCallSignature extracts a method from a call_signature node.
func extractCallSignature(node *sitter.Node, src []byte) methodDef {
	m := methodDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		Name:       "(call)",
		Visibility: "public",
	}

	m.DocComment = findPrecedingDoc(src, m.StartPos)

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "type_parameters":
			m.TypeParams = nodeText(child, src)
			m.TypeParams = strings.TrimPrefix(m.TypeParams, "<")
			m.TypeParams = strings.TrimSuffix(m.TypeParams, ">")
		case "formal_parameters":
			m.Params = extractParams(child, src)
		case "type_annotation":
			m.ReturnType = extractTypeAnnotation(child, src)
		}
	}

	return m
}

// extractProperty extracts a property from a public_field_definition/property_declaration node.
func extractProperty(node *sitter.Node, src []byte) propertyDef {
	p := propertyDef{
		Visibility: "public",
	}

	p.DocComment = findPrecedingDoc(src, int(node.StartByte()))
	p.Decorators = extractDecorators(node, src)

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		text := nodeText(child, src)
		switch text {
		case "static":
			p.Static = true
		case "readonly":
			p.Readonly = true
		case "public":
			p.Visibility = "public"
		case "private":
			p.Visibility = "private"
		case "protected":
			p.Visibility = "protected"
		case "?":
			p.Optional = true
		}

		switch child.Type() {
		case "property_identifier", "identifier":
			if p.Name == "" {
				p.Name = text
			}
		case "type_annotation":
			p.Type = extractTypeAnnotation(child, src)
		}
	}

	return p
}

// extractPropertySignature extracts a property from a property_signature node.
func extractPropertySignature(node *sitter.Node, src []byte) propertyDef {
	p := propertyDef{}

	p.DocComment = findPrecedingDoc(src, int(node.StartByte()))

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		text := nodeText(child, src)
		switch text {
		case "readonly":
			p.Readonly = true
		case "?":
			p.Optional = true
		}

		switch child.Type() {
		case "property_identifier", "identifier":
			if p.Name == "" {
				p.Name = text
			}
		case "type_annotation":
			p.Type = extractTypeAnnotation(child, src)
		}
	}

	return p
}

// extractIndexSignature extracts an index signature as a property.
func extractIndexSignature(node *sitter.Node, src []byte) propertyDef {
	return propertyDef{
		Name:       "[index]",
		Type:       nodeText(node, src),
		DocComment: findPrecedingDoc(src, int(node.StartByte())),
	}
}

// extractParams extracts parameters from a formal_parameters node.
func extractParams(node *sitter.Node, src []byte) []paramDef {
	var params []paramDef

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "required_parameter", "optional_parameter":
			p := extractSingleParam(child, src)
			if child.Type() == "optional_parameter" {
				p.Optional = true
			}
			if p.Name != "" {
				params = append(params, p)
			}
		case "rest_pattern":
			p := extractRestParam(child, src)
			if p.Name != "" {
				params = append(params, p)
			}
		}
	}

	return params
}

// extractSingleParam extracts a single parameter.
func extractSingleParam(node *sitter.Node, src []byte) paramDef {
	p := paramDef{}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		text := nodeText(child, src)
		switch text {
		case "?":
			p.Optional = true
		case "...":
			p.Rest = true
		}

		switch child.Type() {
		case "identifier":
			if p.Name == "" {
				p.Name = text
			}
		case "type_annotation":
			p.Type = extractTypeAnnotation(child, src)
		case "assignment_pattern":
			// Has default value
			p.Optional = true
			extractAssignmentParam(child, src, &p)
		case "rest_pattern":
			p.Rest = true
			rp := extractRestParam(child, src)
			p.Name = rp.Name
			p.Type = rp.Type
		}
	}

	return p
}

// extractRestParam extracts a rest/spread parameter.
func extractRestParam(node *sitter.Node, src []byte) paramDef {
	p := paramDef{Rest: true}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "identifier":
			p.Name = nodeText(child, src)
		case "type_annotation":
			p.Type = extractTypeAnnotation(child, src)
		}
	}

	return p
}

// extractAssignmentParam extracts name and type from an assignment pattern.
func extractAssignmentParam(node *sitter.Node, src []byte, p *paramDef) {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "identifier":
			if p.Name == "" {
				p.Name = nodeText(child, src)
			} else {
				p.Default = nodeText(child, src)
			}
		case "type_annotation":
			p.Type = extractTypeAnnotation(child, src)
		default:
			if p.Name != "" && nodeText(child, src) != "=" {
				if p.Default == "" {
					p.Default = nodeText(child, src)
				}
			}
		}
	}
}

// extractTypeAnnotation extracts the type from a type_annotation node.
func extractTypeAnnotation(node *sitter.Node, src []byte) string {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		// Skip the colon
		if nodeText(child, src) == ":" {
			continue
		}
		return strings.TrimSpace(nodeText(child, src))
	}
	return ""
}

// extractDecorators extracts decorator nodes preceding a declaration.
func extractDecorators(node *sitter.Node, src []byte) []decoratorDef {
	var decorators []decoratorDef

	// Look at previous siblings for decorators
	parent := node.Parent()
	if parent == nil {
		return nil
	}

	nodeIdx := -1
	parentCount := int(parent.ChildCount())
	for i := 0; i < parentCount; i++ {
		if parent.Child(i) == node {
			nodeIdx = i
			break
		}
	}

	if nodeIdx <= 0 {
		return nil
	}

	// Walk backward from the node to find decorator nodes
	for i := nodeIdx - 1; i >= 0; i-- {
		child := parent.Child(i)
		if child == nil {
			break
		}
		if child.Type() == "decorator" {
			d := extractSingleDecorator(child, src)
			decorators = append([]decoratorDef{d}, decorators...)
		} else {
			break
		}
	}

	return decorators
}

// extractSingleDecorator extracts a single decorator.
func extractSingleDecorator(node *sitter.Node, src []byte) decoratorDef {
	d := decoratorDef{}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "identifier":
			d.Name = nodeText(child, src)
		case "call_expression":
			// @Decorator(args)
			callCount := int(child.ChildCount())
			for j := 0; j < callCount; j++ {
				callChild := child.Child(j)
				if callChild == nil {
					continue
				}
				switch callChild.Type() {
				case "identifier", "member_expression":
					d.Name = nodeText(callChild, src)
				case "arguments":
					d.Arguments = nodeText(callChild, src)
				}
			}
		}
	}

	return d
}

// extractLexicalDeclaration handles const/let/var declarations that may
// export arrow functions or class expressions.
func extractLexicalDeclaration(node *sitter.Node, src []byte, result *fileAnalysis) {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "variable_declarator" {
			extractVariableDeclarator(child, src, result)
		}
	}
}

// extractVariableDeclarator handles a single variable declarator that might
// be an exported arrow function: export const foo = (...) => { ... }
func extractVariableDeclarator(node *sitter.Node, src []byte, result *fileAnalysis) {
	var name string
	var valueNode *sitter.Node

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "identifier":
			if name == "" {
				name = nodeText(child, src)
			}
		case "arrow_function", "function":
			valueNode = child
		}
	}

	if name == "" || valueNode == nil {
		return
	}

	// This is an exported arrow function
	fn := functionDef{
		Name:     name,
		Exported: true,
		StartPos: int(node.Parent().StartByte()),
		EndPos:   int(node.Parent().EndByte()),
	}
	fn.DocComment = findPrecedingDoc(src, fn.StartPos)

	// Extract params and return type from the arrow function
	arrowCount := int(valueNode.ChildCount())
	for i := 0; i < arrowCount; i++ {
		child := valueNode.Child(i)
		if child == nil {
			continue
		}
		text := nodeText(child, src)
		if text == "async" {
			fn.Async = true
		}
		switch child.Type() {
		case "formal_parameters":
			fn.Params = extractParams(child, src)
		case "type_parameters":
			fn.TypeParams = text
			fn.TypeParams = strings.TrimPrefix(fn.TypeParams, "<")
			fn.TypeParams = strings.TrimSuffix(fn.TypeParams, ">")
		case "type_annotation":
			fn.ReturnType = extractTypeAnnotation(child, src)
		}
	}

	result.Functions = append(result.Functions, fn)
}

// extractReExportNames extracts names from a re-export clause.
func extractReExportNames(node *sitter.Node, src []byte, modulePath string, result *fileAnalysis) {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "export_specifier" || child.Type() == "import_specifier" {
			name := extractSpecifierName(child, src)
			if name != "" {
				result.ReExports = append(result.ReExports, reExportDef{
					Name:       name,
					ModulePath: modulePath,
				})
			}
		}
	}
}

// extractExportClauseNames extracts names from an export { X, Y } clause.
func extractExportClauseNames(node *sitter.Node, src []byte, result *fileAnalysis) {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "export_specifier" {
			name := extractSpecifierName(child, src)
			if name != "" {
				result.References = append(result.References, name)
			}
		}
	}
}

// extractSpecifierName extracts the exported name from an export/import specifier.
func extractSpecifierName(node *sitter.Node, src []byte) string {
	// Check for alias: export { X as Y }
	var name, alias string
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "identifier" {
			if name == "" {
				name = nodeText(child, src)
			} else {
				alias = nodeText(child, src)
			}
		}
	}
	if alias != "" {
		return alias
	}
	return name
}

// extractImportReferences collects imported names as references.
func extractImportReferences(node *sitter.Node, src []byte, result *fileAnalysis) {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "import_clause" {
			importCount := int(child.ChildCount())
			for j := 0; j < importCount; j++ {
				importChild := child.Child(j)
				if importChild == nil {
					continue
				}
				switch importChild.Type() {
				case "identifier":
					result.References = append(result.References, nodeText(importChild, src))
				case "named_imports":
					namedCount := int(importChild.ChildCount())
					for k := 0; k < namedCount; k++ {
						namedChild := importChild.Child(k)
						if namedChild != nil && namedChild.Type() == "import_specifier" {
							name := extractSpecifierName(namedChild, src)
							if name != "" {
								result.References = append(result.References, name)
							}
						}
					}
				}
			}
		}
	}
}

// walkReferences walks an expression node to collect identifier references.
func walkReferences(node *sitter.Node, src []byte, result *fileAnalysis) {
	if node == nil {
		return
	}
	if node.Type() == "identifier" || node.Type() == "type_identifier" {
		name := nodeText(node, src)
		if name != "" && isUpperFirst(name) {
			result.References = append(result.References, name)
		}
	}
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		walkReferences(node.Child(i), src, result)
	}
}

// findPrecedingDoc finds the JSDoc comment (/** ... */) preceding a declaration.
func findPrecedingDoc(src []byte, pos int) string {
	i := pos - 1
	if i < 0 {
		return ""
	}

	// Skip whitespace
	for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i--
	}

	// Skip decorators: @...
	for i >= 0 && src[i] == ')' {
		depth := 1
		i--
		for i >= 0 && depth > 0 {
			if src[i] == ')' {
				depth++
			} else if src[i] == '(' {
				depth--
			}
			if depth > 0 {
				i--
			}
		}
		// Back up past the decorator name and @
		if i >= 0 && src[i] == '(' {
			i--
		}
		for i >= 0 && src[i] != '@' && src[i] != '\n' {
			i--
		}
		if i >= 0 && src[i] == '@' {
			i--
		}
		// Skip whitespace after backing up past decorator
		for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
			i--
		}
	}

	// Check for end of a block comment: */
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

// nodeText returns the source text for a tree-sitter node.
func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	return string(src[node.StartByte():node.EndByte()])
}

// unquote removes surrounding quotes from a string.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') ||
			(s[0] == '`' && s[len(s)-1] == '`') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// isUpperFirst returns true if the string starts with an uppercase letter.
func isUpperFirst(s string) bool {
	if s == "" {
		return false
	}
	return s[0] >= 'A' && s[0] <= 'Z'
}

// isTypeNode returns true if the node type represents a type expression.
func isTypeNode(nodeType string) bool {
	switch nodeType {
	case "type_identifier", "generic_type", "union_type", "intersection_type",
		"array_type", "tuple_type", "function_type", "conditional_type",
		"mapped_type", "indexed_access_type", "literal_type", "object_type",
		"parenthesized_type", "predicate_type", "template_literal_type",
		"type_query", "lookup_type", "infer_type":
		return true
	}
	return false
}
