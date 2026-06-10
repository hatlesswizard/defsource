package cpp

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// fileAnalysis holds all AST-extracted entities from one C++ source file.
type fileAnalysis struct {
	Classes     []classDef
	Structs     []structDef
	Functions   []functionDef
	Enums       []enumDef
	TypeAliases []typeAliasDef
	Namespaces  []namespaceDef
	Concepts    []conceptDef
	Calls       []callRef
}

// callRef is a lightweight call-site reference collected during AST walking.
type callRef struct {
	Name string
	Kind string // "function", "method", "static"
}

// classDef holds data extracted from a class_specifier AST node.
type classDef struct {
	Name          string
	QualifiedName string
	Namespace     string
	Bases         []baseDef
	Methods       []methodDef
	Fields        []fieldDef
	Friends       []string
	TemplateParams string
	StartPos      int
	EndPos        int
	DocComment    string
	Visibility    string
}

// structDef holds data extracted from a struct_specifier AST node.
type structDef struct {
	Name          string
	QualifiedName string
	Namespace     string
	Bases         []baseDef
	Methods       []methodDef
	Fields        []fieldDef
	TemplateParams string
	StartPos      int
	EndPos        int
	DocComment    string
}

// baseDef holds a base class/struct reference.
type baseDef struct {
	Name       string
	Visibility string // "public", "protected", "private"
	Virtual    bool
}

// functionDef holds data from a function_definition or declaration.
type functionDef struct {
	Name          string
	QualifiedName string
	Namespace     string
	Signature     string
	ReturnType    string
	TemplateParams string
	Constexpr     bool
	Consteval     bool
	StartPos      int
	EndPos        int
	DocComment    string
}

// methodDef holds data from a function_definition inside a class/struct body.
type methodDef struct {
	Name          string
	Signature     string
	ReturnType    string
	Visibility    string
	Virtual       bool
	Override      bool
	Const         bool
	Noexcept      bool
	Static        bool
	Constexpr     bool
	Consteval     bool
	Pure          bool   // = 0
	Defaulted     bool   // = default
	Deleted       bool   // = delete
	TemplateParams string
	StartPos      int
	EndPos        int
	DocComment    string
}

// fieldDef holds a class/struct member field.
type fieldDef struct {
	Name       string
	Type       string
	Visibility string
	Static     bool
	Constexpr  bool
	DocComment string
}

// enumDef holds data from an enum/enum class.
type enumDef struct {
	Name          string
	QualifiedName string
	Namespace     string
	Scoped        bool   // true for "enum class"
	UnderlyingType string
	Values        []enumValue
	StartPos      int
	EndPos        int
	DocComment    string
}

// enumValue holds a single enumerator.
type enumValue struct {
	Name       string
	Value      string
	DocComment string
}

// typeAliasDef holds a using/typedef declaration.
type typeAliasDef struct {
	Name          string
	QualifiedName string
	Namespace     string
	AliasedType   string
	TemplateParams string
	StartPos      int
	EndPos        int
	DocComment    string
}

// namespaceDef holds a namespace declaration.
type namespaceDef struct {
	Name          string
	QualifiedName string
	StartPos      int
	EndPos        int
}

// conceptDef holds a C++20 concept definition.
type conceptDef struct {
	Name          string
	QualifiedName string
	Namespace     string
	Constraint    string
	TemplateParams string
	StartPos      int
	EndPos        int
	DocComment    string
}

// parseFile parses a C++ source file using tree-sitter and returns the analysis.
func parseFile(src []byte) *fileAnalysis {
	parser, err := treesitter.Get(treesitter.Cpp)
	if err != nil {
		return &fileAnalysis{}
	}
	defer treesitter.Put(treesitter.Cpp, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return &fileAnalysis{}
	}
	defer tree.Close()

	result := &fileAnalysis{}
	root := tree.RootNode()
	walkNode(root, src, result, "")
	return result
}

// walkNode recursively walks the AST, collecting entities.
// nsPrefix carries the current namespace context (e.g., "std::").
func walkNode(node *sitter.Node, src []byte, result *fileAnalysis, nsPrefix string) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "namespace_definition":
		walkNamespace(node, src, result, nsPrefix)
		return

	case "class_specifier":
		cls := extractClass(node, src, nsPrefix)
		if cls.Name != "" {
			result.Classes = append(result.Classes, cls)
		}
		return

	case "struct_specifier":
		st := extractStruct(node, src, nsPrefix)
		if st.Name != "" {
			result.Structs = append(result.Structs, st)
		}
		return

	case "function_definition":
		fn := extractFunction(node, src, nsPrefix)
		if fn.Name != "" {
			result.Functions = append(result.Functions, fn)
		}
		// Walk body for calls
		if body := node.ChildByFieldName("body"); body != nil {
			walkForCalls(body, src, result)
		}
		return

	case "declaration":
		// May contain function declarations, enums, type aliases, or variable declarations
		walkDeclaration(node, src, result, nsPrefix)
		return

	case "enum_specifier":
		en := extractEnum(node, src, nsPrefix)
		if en.Name != "" {
			result.Enums = append(result.Enums, en)
		}
		return

	case "alias_declaration":
		ta := extractTypeAlias(node, src, nsPrefix)
		if ta.Name != "" {
			result.TypeAliases = append(result.TypeAliases, ta)
		}
		return

	case "type_definition":
		ta := extractTypedef(node, src, nsPrefix)
		if ta.Name != "" {
			result.TypeAliases = append(result.TypeAliases, ta)
		}
		return

	case "template_declaration":
		walkTemplate(node, src, result, nsPrefix)
		return

	case "concept_definition":
		c := extractConcept(node, src, nsPrefix)
		if c.Name != "" {
			result.Concepts = append(result.Concepts, c)
		}
		return

	case "call_expression":
		fnNode := node.ChildByFieldName("function")
		if fnNode != nil {
			name := nodeText(fnNode, src)
			if name != "" {
				result.Calls = append(result.Calls, callRef{Name: name, Kind: "function"})
			}
		}
	}

	// Default: recurse into all named children.
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		walkNode(node.NamedChild(i), src, result, nsPrefix)
	}
}

// walkNamespace extracts namespace info and recurses into the namespace body.
func walkNamespace(node *sitter.Node, src []byte, result *fileAnalysis, nsPrefix string) {
	name := ""
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		name = nodeText(nameNode, src)
	}

	// Handle nested namespace syntax: namespace a::b::c
	qualifiedName := nsPrefix + name
	if name != "" {
		ns := namespaceDef{
			Name:          name,
			QualifiedName: qualifiedName,
			StartPos:      int(node.StartByte()),
			EndPos:        int(node.EndByte()),
		}
		result.Namespaces = append(result.Namespaces, ns)
	}

	newPrefix := qualifiedName
	if newPrefix != "" {
		newPrefix += "::"
	}

	// Recurse into namespace body
	body := node.ChildByFieldName("body")
	if body != nil {
		count := int(body.NamedChildCount())
		for i := 0; i < count; i++ {
			walkNode(body.NamedChild(i), src, result, newPrefix)
		}
	}
}

// walkDeclaration handles declaration nodes which may contain function
// declarations, variable declarations, or other nested entities.
func walkDeclaration(node *sitter.Node, src []byte, result *fileAnalysis, nsPrefix string) {
	// Check for enum specifiers, class/struct specifiers inside declaration
	count := int(node.NamedChildCount())
	foundNested := false
	for i := 0; i < count; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "enum_specifier":
			en := extractEnum(child, src, nsPrefix)
			if en.Name != "" {
				en.StartPos = int(node.StartByte())
				en.EndPos = int(node.EndByte())
				result.Enums = append(result.Enums, en)
			}
			foundNested = true
		case "class_specifier":
			cls := extractClass(child, src, nsPrefix)
			if cls.Name != "" {
				result.Classes = append(result.Classes, cls)
			}
			foundNested = true
		case "struct_specifier":
			st := extractStruct(child, src, nsPrefix)
			if st.Name != "" {
				result.Structs = append(result.Structs, st)
			}
			foundNested = true
		}
	}
	if foundNested {
		return
	}

	// Check if this is a function declaration (has a function_declarator)
	if isFunctionDeclaration(node, src) {
		fn := extractFunctionFromDecl(node, src, nsPrefix)
		if fn.Name != "" {
			result.Functions = append(result.Functions, fn)
		}
	}
}

// isFunctionDeclaration checks if a declaration node represents a function declaration.
func isFunctionDeclaration(node *sitter.Node, src []byte) bool {
	declarator := node.ChildByFieldName("declarator")
	if declarator == nil {
		return false
	}
	return hasFunctionDeclarator(declarator)
}

// hasFunctionDeclarator recursively checks if a node is or contains a function_declarator.
func hasFunctionDeclarator(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	if node.Type() == "function_declarator" {
		return true
	}
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		if hasFunctionDeclarator(node.NamedChild(i)) {
			return true
		}
	}
	return false
}

// extractFunctionFromDecl extracts a functionDef from a declaration node (prototype).
func extractFunctionFromDecl(node *sitter.Node, src []byte, nsPrefix string) functionDef {
	fn := functionDef{
		StartPos:  int(node.StartByte()),
		EndPos:    int(node.EndByte()),
		Namespace: strings.TrimSuffix(nsPrefix, "::"),
	}

	// Get the declarator for name
	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		fn.Name = extractDeclaratorName(declarator, src)
	}
	fn.QualifiedName = nsPrefix + fn.Name

	// Get return type
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		fn.ReturnType = nodeText(typeNode, src)
	}

	// Signature is the full declaration (minus trailing semicolon)
	fn.Signature = strings.TrimSuffix(strings.TrimSpace(nodeText(node, src)), ";")

	// Check constexpr/consteval
	text := nodeText(node, src)
	fn.Constexpr = strings.Contains(text, "constexpr ")
	fn.Consteval = strings.Contains(text, "consteval ")

	// Doc comment
	fn.DocComment = findPrecedingDoc(src, fn.StartPos)

	return fn
}

// walkTemplate handles template declarations by extracting template params
// and then processing the inner declaration.
func walkTemplate(node *sitter.Node, src []byte, result *fileAnalysis, nsPrefix string) {
	templateParams := ""
	paramNode := node.ChildByFieldName("parameters")
	if paramNode != nil {
		templateParams = nodeText(paramNode, src)
	}

	templateStart := int(node.StartByte())
	templateEnd := int(node.EndByte())
	// Doc comment precedes the template keyword
	templateDoc := findPrecedingDoc(src, templateStart)

	// Find the inner declaration (last named child that is not template_parameters)
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type() == "template_parameter_list" {
			continue
		}

		switch child.Type() {
		case "class_specifier":
			cls := extractClass(child, src, nsPrefix)
			if cls.Name != "" {
				cls.TemplateParams = templateParams
				cls.StartPos = templateStart
				cls.EndPos = templateEnd
				if templateDoc != "" {
					cls.DocComment = templateDoc
				}
				result.Classes = append(result.Classes, cls)
			}
		case "struct_specifier":
			st := extractStruct(child, src, nsPrefix)
			if st.Name != "" {
				st.TemplateParams = templateParams
				st.StartPos = templateStart
				st.EndPos = templateEnd
				if templateDoc != "" {
					st.DocComment = templateDoc
				}
				result.Structs = append(result.Structs, st)
			}
		case "function_definition":
			fn := extractFunction(child, src, nsPrefix)
			if fn.Name != "" {
				fn.TemplateParams = templateParams
				fn.StartPos = templateStart
				fn.EndPos = templateEnd
				if templateDoc != "" {
					fn.DocComment = templateDoc
				}
				result.Functions = append(result.Functions, fn)
			}
		case "declaration":
			// Template declaration with a declaration inside (e.g., template function declaration)
			walkDeclaration(child, src, result, nsPrefix)
		case "alias_declaration":
			ta := extractTypeAlias(child, src, nsPrefix)
			if ta.Name != "" {
				ta.TemplateParams = templateParams
				ta.StartPos = templateStart
				ta.EndPos = templateEnd
				if templateDoc != "" {
					ta.DocComment = templateDoc
				}
				result.TypeAliases = append(result.TypeAliases, ta)
			}
		case "concept_definition":
			c := extractConcept(child, src, nsPrefix)
			if c.Name != "" {
				c.TemplateParams = templateParams
				c.StartPos = templateStart
				c.EndPos = templateEnd
				if templateDoc != "" {
					c.DocComment = templateDoc
				}
				result.Concepts = append(result.Concepts, c)
			}
		}
	}
}

// extractClass extracts a classDef from a class_specifier node.
func extractClass(node *sitter.Node, src []byte, nsPrefix string) classDef {
	cls := classDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		Namespace:  strings.TrimSuffix(nsPrefix, "::"),
		Visibility: "public",
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		cls.Name = nodeText(nameNode, src)
		cls.QualifiedName = nsPrefix + cls.Name
	}

	// Extract base classes
	cls.Bases = extractBases(node, src)

	// Extract doc comment
	cls.DocComment = findPrecedingDoc(src, cls.StartPos)

	// Walk body for members
	body := node.ChildByFieldName("body")
	if body != nil {
		walkClassBody(body, src, &cls)
	}

	return cls
}

// extractStruct extracts a structDef from a struct_specifier node.
func extractStruct(node *sitter.Node, src []byte, nsPrefix string) structDef {
	st := structDef{
		StartPos:  int(node.StartByte()),
		EndPos:    int(node.EndByte()),
		Namespace: strings.TrimSuffix(nsPrefix, "::"),
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		st.Name = nodeText(nameNode, src)
		st.QualifiedName = nsPrefix + st.Name
	}

	// Extract base classes
	st.Bases = extractBases(node, src)

	// Extract doc comment
	st.DocComment = findPrecedingDoc(src, st.StartPos)

	// Walk body for members
	body := node.ChildByFieldName("body")
	if body != nil {
		walkStructBody(body, src, &st)
	}

	return st
}

// extractBases extracts base class specifiers from a class/struct node.
func extractBases(node *sitter.Node, src []byte) []baseDef {
	var bases []baseDef
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type() == "base_class_clause" {
			// Walk the base_class_clause children
			bcCount := int(child.NamedChildCount())
			for j := 0; j < bcCount; j++ {
				bc := child.NamedChild(j)
				if bc == nil {
					continue
				}
				// Look for type_identifier or qualified_identifier
				switch bc.Type() {
				case "base_class_specifier":
					// A base_class_specifier may have access specifier children
					base := extractBaseSpecifier(bc, src)
					if base.Name != "" {
						bases = append(bases, base)
					}
				default:
					// Might be a direct type name
					text := nodeText(bc, src)
					if text != "" && bc.Type() != "access_specifier" {
						bases = append(bases, baseDef{Name: text, Visibility: "private"})
					}
				}
			}
		}
	}
	return bases
}

// extractBaseSpecifier extracts a single base class from a base_class_specifier.
func extractBaseSpecifier(node *sitter.Node, src []byte) baseDef {
	base := baseDef{Visibility: "private"} // class default
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "access_specifier":
			base.Visibility = nodeText(child, src)
		case "virtual":
			base.Virtual = true
		case "type_identifier", "qualified_identifier", "template_type":
			base.Name = nodeText(child, src)
		}
	}
	return base
}

// walkClassBody processes the field_declaration_list (class body) extracting
// methods, fields, and friend declarations, tracking access specifiers.
func walkClassBody(body *sitter.Node, src []byte, cls *classDef) {
	currentVisibility := "private" // class default
	count := int(body.NamedChildCount())
	for i := 0; i < count; i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "access_specifier":
			currentVisibility = extractAccessSpecifier(child, src)

		case "function_definition":
			m := extractMethodDef(child, src, currentVisibility)
			cls.Methods = append(cls.Methods, m)

		case "declaration":
			// Could be a method declaration or field declaration
			if isMethodDeclaration(child, src) {
				m := extractMethodFromDecl(child, src, currentVisibility)
				if m.Name != "" {
					cls.Methods = append(cls.Methods, m)
				}
			} else {
				fields := extractFieldsFromDecl(child, src, currentVisibility)
				cls.Fields = append(cls.Fields, fields...)
			}

		case "field_declaration":
			fields := extractFields(child, src, currentVisibility)
			cls.Fields = append(cls.Fields, fields...)

		case "friend_declaration":
			friendText := nodeText(child, src)
			cls.Friends = append(cls.Friends, friendText)

		case "template_declaration":
			// Template method inside class
			walkTemplateInClass(child, src, cls, currentVisibility)
		}
	}
}

// walkStructBody processes a struct body the same as class but with public default.
func walkStructBody(body *sitter.Node, src []byte, st *structDef) {
	currentVisibility := "public" // struct default
	count := int(body.NamedChildCount())
	for i := 0; i < count; i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "access_specifier":
			currentVisibility = extractAccessSpecifier(child, src)

		case "function_definition":
			m := extractMethodDef(child, src, currentVisibility)
			st.Methods = append(st.Methods, m)

		case "declaration":
			if isMethodDeclaration(child, src) {
				m := extractMethodFromDecl(child, src, currentVisibility)
				if m.Name != "" {
					st.Methods = append(st.Methods, m)
				}
			} else {
				fields := extractFieldsFromDecl(child, src, currentVisibility)
				st.Fields = append(st.Fields, fields...)
			}

		case "field_declaration":
			fields := extractFields(child, src, currentVisibility)
			st.Fields = append(st.Fields, fields...)

		case "template_declaration":
			walkTemplateInStruct(child, src, st, currentVisibility)
		}
	}
}

// walkTemplateInClass handles template declarations inside a class body.
func walkTemplateInClass(node *sitter.Node, src []byte, cls *classDef, visibility string) {
	templateParams := ""
	paramNode := node.ChildByFieldName("parameters")
	if paramNode != nil {
		templateParams = nodeText(paramNode, src)
	}

	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type() == "template_parameter_list" {
			continue
		}
		if child.Type() == "function_definition" {
			m := extractMethodDef(child, src, visibility)
			m.TemplateParams = templateParams
			cls.Methods = append(cls.Methods, m)
		} else if child.Type() == "declaration" && isMethodDeclaration(child, src) {
			m := extractMethodFromDecl(child, src, visibility)
			if m.Name != "" {
				m.TemplateParams = templateParams
				cls.Methods = append(cls.Methods, m)
			}
		}
	}
}

// walkTemplateInStruct handles template declarations inside a struct body.
func walkTemplateInStruct(node *sitter.Node, src []byte, st *structDef, visibility string) {
	templateParams := ""
	paramNode := node.ChildByFieldName("parameters")
	if paramNode != nil {
		templateParams = nodeText(paramNode, src)
	}

	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type() == "template_parameter_list" {
			continue
		}
		if child.Type() == "function_definition" {
			m := extractMethodDef(child, src, visibility)
			m.TemplateParams = templateParams
			st.Methods = append(st.Methods, m)
		} else if child.Type() == "declaration" && isMethodDeclaration(child, src) {
			m := extractMethodFromDecl(child, src, visibility)
			if m.Name != "" {
				m.TemplateParams = templateParams
				st.Methods = append(st.Methods, m)
			}
		}
	}
}

// extractAccessSpecifier extracts the access level from an access_specifier node.
func extractAccessSpecifier(node *sitter.Node, src []byte) string {
	text := strings.TrimSpace(nodeText(node, src))
	text = strings.TrimSuffix(text, ":")
	text = strings.TrimSpace(text)
	switch text {
	case "public":
		return "public"
	case "protected":
		return "protected"
	case "private":
		return "private"
	}
	return "private"
}

// extractMethodDef builds a methodDef from a function_definition inside a class/struct body.
func extractMethodDef(node *sitter.Node, src []byte, visibility string) methodDef {
	m := methodDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		Visibility: visibility,
	}

	// Get the declarator
	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		m.Name = extractDeclaratorName(declarator, src)
		m.Signature = nodeText(node, src)
		// Trim body from signature for compact display
		if body := node.ChildByFieldName("body"); body != nil {
			sigEnd := int(body.StartByte()) - int(node.StartByte())
			fullText := nodeText(node, src)
			if sigEnd > 0 && sigEnd < len(fullText) {
				m.Signature = strings.TrimSpace(fullText[:sigEnd])
			}
		}
	}

	// Extract return type
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		m.ReturnType = nodeText(typeNode, src)
	}

	// Check qualifiers
	m.Virtual, m.Override, m.Const, m.Noexcept, m.Static, m.Constexpr, m.Consteval = extractQualifiers(node, src)

	// Doc comment
	m.DocComment = findPrecedingDoc(src, m.StartPos)

	return m
}

// extractMethodFromDecl builds a methodDef from a declaration node (method prototype).
func extractMethodFromDecl(node *sitter.Node, src []byte, visibility string) methodDef {
	m := methodDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		Visibility: visibility,
	}

	m.Signature = strings.TrimSuffix(strings.TrimSpace(nodeText(node, src)), ";")

	// Try to extract name from declarator
	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		m.Name = extractDeclaratorName(declarator, src)
	}

	// Extract return type
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		m.ReturnType = nodeText(typeNode, src)
	}

	// Check for special suffixes
	text := nodeText(node, src)
	m.Virtual = strings.Contains(text, "virtual ")
	m.Override = strings.Contains(text, " override")
	m.Const = strings.Contains(text, ") const")
	m.Noexcept = strings.Contains(text, "noexcept")
	m.Static = strings.Contains(text, "static ")
	m.Constexpr = strings.Contains(text, "constexpr ")
	m.Consteval = strings.Contains(text, "consteval ")
	m.Pure = strings.HasSuffix(strings.TrimSpace(text), "= 0;") || strings.HasSuffix(strings.TrimSpace(text), "= 0")
	m.Defaulted = strings.HasSuffix(strings.TrimSpace(text), "= default;") || strings.HasSuffix(strings.TrimSpace(text), "= default")
	m.Deleted = strings.HasSuffix(strings.TrimSpace(text), "= delete;") || strings.HasSuffix(strings.TrimSpace(text), "= delete")

	// Doc comment
	m.DocComment = findPrecedingDoc(src, m.StartPos)

	return m
}

// isMethodDeclaration checks whether a declaration node represents a method/function declaration.
func isMethodDeclaration(node *sitter.Node, src []byte) bool {
	text := nodeText(node, src)
	// A method declaration has parentheses (parameter list) and no '=' assignment
	// but may have = default, = delete, = 0
	if !strings.Contains(text, "(") {
		return false
	}
	// Look for function_declarator in children
	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		switch declarator.Type() {
		case "function_declarator", "reference_declarator":
			return true
		}
		// Check for nested function declarator
		count := int(declarator.NamedChildCount())
		for i := 0; i < count; i++ {
			child := declarator.NamedChild(i)
			if child != nil && child.Type() == "function_declarator" {
				return true
			}
		}
	}
	return false
}

// extractFieldsFromDecl extracts field definitions from a declaration node.
func extractFieldsFromDecl(node *sitter.Node, src []byte, visibility string) []fieldDef {
	var fields []fieldDef
	text := nodeText(node, src)
	docComment := findPrecedingDoc(src, int(node.StartByte()))

	// Extract type
	typeName := ""
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		typeName = nodeText(typeNode, src)
	}

	// Extract declarator names
	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		name := extractDeclaratorName(declarator, src)
		if name != "" {
			f := fieldDef{
				Name:       name,
				Type:       typeName,
				Visibility: visibility,
				Static:     strings.Contains(text, "static "),
				Constexpr:  strings.Contains(text, "constexpr "),
				DocComment: docComment,
			}
			fields = append(fields, f)
		}
	}
	return fields
}

// extractFields extracts field definitions from a field_declaration node.
func extractFields(node *sitter.Node, src []byte, visibility string) []fieldDef {
	var fields []fieldDef
	text := nodeText(node, src)
	docComment := findPrecedingDoc(src, int(node.StartByte()))

	typeName := ""
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		typeName = nodeText(typeNode, src)
	}

	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		name := extractDeclaratorName(declarator, src)
		if name != "" {
			f := fieldDef{
				Name:       name,
				Type:       typeName,
				Visibility: visibility,
				Static:     strings.Contains(text, "static "),
				Constexpr:  strings.Contains(text, "constexpr "),
				DocComment: docComment,
			}
			fields = append(fields, f)
		}
	}
	return fields
}

// extractFunction builds a functionDef from a function_definition node.
func extractFunction(node *sitter.Node, src []byte, nsPrefix string) functionDef {
	fn := functionDef{
		StartPos:  int(node.StartByte()),
		EndPos:    int(node.EndByte()),
		Namespace: strings.TrimSuffix(nsPrefix, "::"),
	}

	// Get the declarator for name
	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		fn.Name = extractDeclaratorName(declarator, src)
	}
	fn.QualifiedName = nsPrefix + fn.Name

	// Get return type
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		fn.ReturnType = nodeText(typeNode, src)
	}

	// Build signature (everything before body)
	fn.Signature = nodeText(node, src)
	if body := node.ChildByFieldName("body"); body != nil {
		sigEnd := int(body.StartByte()) - int(node.StartByte())
		fullText := nodeText(node, src)
		if sigEnd > 0 && sigEnd < len(fullText) {
			fn.Signature = strings.TrimSpace(fullText[:sigEnd])
		}
	}

	// Check constexpr/consteval
	text := nodeText(node, src)
	fn.Constexpr = strings.HasPrefix(text, "constexpr ")
	fn.Consteval = strings.HasPrefix(text, "consteval ")

	// Doc comment
	fn.DocComment = findPrecedingDoc(src, fn.StartPos)

	return fn
}

// extractEnum builds an enumDef from an enum_specifier node.
func extractEnum(node *sitter.Node, src []byte, nsPrefix string) enumDef {
	en := enumDef{
		StartPos:  int(node.StartByte()),
		EndPos:    int(node.EndByte()),
		Namespace: strings.TrimSuffix(nsPrefix, "::"),
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		en.Name = nodeText(nameNode, src)
		en.QualifiedName = nsPrefix + en.Name
	}

	// Detect scoped enum (enum class / enum struct)
	text := nodeText(node, src)
	en.Scoped = strings.HasPrefix(text, "enum class ") || strings.HasPrefix(text, "enum struct ")

	// Extract underlying type
	underlyingType := node.ChildByFieldName("underlying_type")
	if underlyingType != nil {
		en.UnderlyingType = nodeText(underlyingType, src)
	}

	// Extract enumerator values
	body := node.ChildByFieldName("body")
	if body != nil {
		count := int(body.NamedChildCount())
		for i := 0; i < count; i++ {
			child := body.NamedChild(i)
			if child == nil {
				continue
			}
			if child.Type() == "enumerator" {
				ev := extractEnumerator(child, src)
				en.Values = append(en.Values, ev)
			}
		}
	}

	// Doc comment
	en.DocComment = findPrecedingDoc(src, en.StartPos)

	return en
}

// extractEnumerator extracts a single enum value.
func extractEnumerator(node *sitter.Node, src []byte) enumValue {
	ev := enumValue{}
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		ev.Name = nodeText(nameNode, src)
	}
	valueNode := node.ChildByFieldName("value")
	if valueNode != nil {
		ev.Value = nodeText(valueNode, src)
	}
	ev.DocComment = findPrecedingDoc(src, int(node.StartByte()))
	return ev
}

// extractTypeAlias builds a typeAliasDef from an alias_declaration (using X = T).
func extractTypeAlias(node *sitter.Node, src []byte, nsPrefix string) typeAliasDef {
	ta := typeAliasDef{
		StartPos:  int(node.StartByte()),
		EndPos:    int(node.EndByte()),
		Namespace: strings.TrimSuffix(nsPrefix, "::"),
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		ta.Name = nodeText(nameNode, src)
		ta.QualifiedName = nsPrefix + ta.Name
	}

	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		ta.AliasedType = nodeText(typeNode, src)
	}

	ta.DocComment = findPrecedingDoc(src, ta.StartPos)
	return ta
}

// extractTypedef builds a typeAliasDef from a type_definition (typedef T X).
func extractTypedef(node *sitter.Node, src []byte, nsPrefix string) typeAliasDef {
	ta := typeAliasDef{
		StartPos:  int(node.StartByte()),
		EndPos:    int(node.EndByte()),
		Namespace: strings.TrimSuffix(nsPrefix, "::"),
	}

	// Extract the type and declarator
	typeNode := node.ChildByFieldName("type")
	if typeNode != nil {
		ta.AliasedType = nodeText(typeNode, src)
	}

	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		ta.Name = extractDeclaratorName(declarator, src)
		ta.QualifiedName = nsPrefix + ta.Name
	}

	ta.DocComment = findPrecedingDoc(src, ta.StartPos)
	return ta
}

// extractConcept builds a conceptDef from a concept_definition node.
func extractConcept(node *sitter.Node, src []byte, nsPrefix string) conceptDef {
	c := conceptDef{
		StartPos:  int(node.StartByte()),
		EndPos:    int(node.EndByte()),
		Namespace: strings.TrimSuffix(nsPrefix, "::"),
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		c.Name = nodeText(nameNode, src)
		c.QualifiedName = nsPrefix + c.Name
	}

	// Constraint (everything after = )
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type() != "identifier" && child.Type() != "template_parameter_list" {
			c.Constraint = nodeText(child, src)
		}
	}

	c.DocComment = findPrecedingDoc(src, c.StartPos)
	return c
}

// extractDeclaratorName extracts the function/variable name from various
// declarator node types.
func extractDeclaratorName(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}

	switch node.Type() {
	case "identifier", "field_identifier", "type_identifier":
		return nodeText(node, src)

	case "qualified_identifier":
		// Return the last segment as the name, but we keep full qualified name
		return nodeText(node, src)

	case "operator_name":
		return nodeText(node, src)

	case "destructor_name":
		return nodeText(node, src)

	case "function_declarator":
		// Name is the first child (the declarator before the parameter list)
		declarator := node.ChildByFieldName("declarator")
		if declarator != nil {
			return extractDeclaratorName(declarator, src)
		}
		// Fallback: first named child
		if node.NamedChildCount() > 0 {
			return extractDeclaratorName(node.NamedChild(0), src)
		}

	case "reference_declarator", "pointer_declarator":
		// Unwrap: the actual declarator is inside
		count := int(node.NamedChildCount())
		for i := 0; i < count; i++ {
			child := node.NamedChild(i)
			if child != nil {
				name := extractDeclaratorName(child, src)
				if name != "" {
					return name
				}
			}
		}

	case "parenthesized_declarator":
		if node.NamedChildCount() > 0 {
			return extractDeclaratorName(node.NamedChild(0), src)
		}

	case "init_declarator":
		declarator := node.ChildByFieldName("declarator")
		if declarator != nil {
			return extractDeclaratorName(declarator, src)
		}

	case "array_declarator":
		declarator := node.ChildByFieldName("declarator")
		if declarator != nil {
			return extractDeclaratorName(declarator, src)
		}

	default:
		// Try ChildByFieldName("declarator") as generic fallback
		declarator := node.ChildByFieldName("declarator")
		if declarator != nil {
			return extractDeclaratorName(declarator, src)
		}
		// Or if it's a terminal, just return its text
		if node.NamedChildCount() == 0 {
			return nodeText(node, src)
		}
	}

	return ""
}

// extractQualifiers detects method qualifiers from the AST node and source text.
func extractQualifiers(node *sitter.Node, src []byte) (virtual, override, isConst, noexcept, static, constexpr, consteval bool) {
	text := nodeText(node, src)
	virtual = strings.Contains(text, "virtual ")
	override = strings.Contains(text, " override")
	isConst = strings.Contains(text, ") const")
	noexcept = strings.Contains(text, "noexcept")
	static = strings.Contains(text, "static ")
	constexpr = strings.HasPrefix(strings.TrimSpace(text), "constexpr ")
	consteval = strings.HasPrefix(strings.TrimSpace(text), "consteval ")
	return
}

// walkForCalls walks a subtree collecting only call expressions.
func walkForCalls(node *sitter.Node, src []byte, result *fileAnalysis) {
	if node == nil {
		return
	}
	if node.Type() == "call_expression" {
		fnNode := node.ChildByFieldName("function")
		if fnNode != nil {
			name := nodeText(fnNode, src)
			if name != "" {
				kind := "function"
				if strings.Contains(name, "::") {
					kind = "static"
				} else if strings.Contains(name, ".") || strings.Contains(name, "->") {
					kind = "method"
				}
				result.Calls = append(result.Calls, callRef{Name: name, Kind: kind})
			}
		}
	}
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		walkForCalls(node.NamedChild(i), src, result)
	}
}

// findPrecedingDoc finds a Doxygen comment (/** */ or ///) immediately
// preceding the declaration at byte offset pos.
func findPrecedingDoc(src []byte, pos int) string {
	if pos <= 0 || pos > len(src) {
		return ""
	}

	// Scan backwards from pos skipping whitespace
	i := pos - 1
	for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i--
	}

	if i < 0 {
		return ""
	}

	// Check for block comment ending with */ (requires at least "*/")
	if i >= 1 && src[i-1] == '*' && src[i] == '/' {
		end := i // position of closing '/'
		// Scan back to find the opening "/*"
		j := i - 2
		for j >= 0 {
			if src[j] == '/' && j+1 < len(src) && src[j+1] == '*' {
				// Check it's a doc comment (/** or /*!)
				if j+2 < len(src) && (src[j+2] == '*' || src[j+2] == '!') {
					return string(src[j : end+1])
				}
				// Regular comment, not a doc comment
				return ""
			}
			j--
		}
		return ""
	}

	// Check for line comments (/// or //!)
	// Find the end of line that contains the character at position i
	lineEnd := i
	// Go to start of current line
	lineStart := i
	for lineStart > 0 && src[lineStart-1] != '\n' {
		lineStart--
	}
	line := strings.TrimSpace(string(src[lineStart : lineEnd+1]))
	if strings.HasPrefix(line, "///") || strings.HasPrefix(line, "//!") {
		// Collect consecutive /// lines
		var lines []string
		lines = append(lines, line)
		// Look for more lines above
		j := lineStart - 1
		for j >= 0 {
			// Skip CR/LF
			for j >= 0 && (src[j] == '\n' || src[j] == '\r') {
				j--
			}
			if j < 0 {
				break
			}
			// Find start of previous line
			prevEnd := j
			prevStart := j
			for prevStart > 0 && src[prevStart-1] != '\n' {
				prevStart--
			}
			prevLine := strings.TrimSpace(string(src[prevStart : prevEnd+1]))
			if strings.HasPrefix(prevLine, "///") || strings.HasPrefix(prevLine, "//!") {
				lines = append([]string{prevLine}, lines...)
				j = prevStart - 1
			} else {
				break
			}
		}
		return strings.Join(lines, "\n")
	}

	return ""
}

// nodeText returns the source text of a node.
func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	start := node.StartByte()
	end := node.EndByte()
	if int(start) >= len(src) || int(end) > len(src) {
		return ""
	}
	return string(src[start:end])
}
