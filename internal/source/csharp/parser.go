package csharp

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/docparser"
	"github.com/hatlesswizard/defsource/internal/docparser/xmldoc"
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// xmlDocParser is the shared XML doc comment parser instance.
var xmlDocParser = xmldoc.New()

// parseXMLDoc parses raw XML doc comment text into a structured DocComment.
func parseXMLDoc(raw string) *docparser.DocComment {
	return xmlDocParser.Parse(raw)
}

// fileAnalysis holds all AST-extracted type definitions from one .cs file.
type fileAnalysis struct {
	Types      []typeDef
	References []string // Type/method names referenced in the file
}

// findType finds a type definition by qualified name (Namespace.TypeName).
func (fa *fileAnalysis) findType(qualifiedName string) *typeDef {
	for i := range fa.Types {
		if fa.Types[i].qualifiedName() == qualifiedName {
			return &fa.Types[i]
		}
	}
	// Fallback: try matching just the type name (for simple cases)
	parts := strings.Split(qualifiedName, ".")
	simpleName := parts[len(parts)-1]
	for i := range fa.Types {
		if fa.Types[i].Name == simpleName {
			return &fa.Types[i]
		}
	}
	return nil
}

// typeDef holds data extracted from a type declaration AST node.
type typeDef struct {
	Name       string
	Namespace  string
	Kind       string // "class", "interface", "struct", "record", "enum", "delegate"
	Visibility string // "public", "protected", "private", "internal"
	StartPos   int
	EndPos     int
	DocComment string
	IsPartial  bool
	IsStatic   bool
	IsAbstract bool
	IsSealed   bool
	Generics   string // e.g., "<T, U>"
	BaseTypes  []string
	Attributes []string
	Methods    []methodDef
	Properties []propertyDef
	Fields     []fieldDef
	Events     []eventDef
	Members    []enumMemberDef // For enums only
	Deprecated bool
}

// qualifiedName returns "Namespace.TypeName".
func (td *typeDef) qualifiedName() string {
	if td.Namespace == "" {
		return td.Name
	}
	return td.Namespace + "." + td.Name
}

// methodDef holds data extracted from a method/constructor declaration.
type methodDef struct {
	Name        string
	Signature   string
	Visibility  string
	ReturnType  string
	StartPos    int
	EndPos      int
	DocComment  string
	IsStatic    bool
	IsAbstract  bool
	IsVirtual   bool
	IsOverride  bool
	IsAsync     bool
	IsExtension bool // Extension method (has 'this' first param)
	Generics    string
	Params      []paramDef
	Attributes  []string
}

// paramDef holds data for a method parameter.
type paramDef struct {
	Name       string
	Type       string
	HasDefault bool
	Default    string
	IsParams   bool // params keyword
	IsRef      bool
	IsOut      bool
	IsIn       bool
	IsThis     bool // Extension method 'this' parameter
}

// propertyDef holds data for a property declaration.
type propertyDef struct {
	Name       string
	Type       string
	Visibility string
	DocComment string
	HasGetter  bool
	HasSetter  bool
	HasInit    bool
	IsAuto     bool
	StartPos   int
	EndPos     int
}

// fieldDef holds data for a field declaration.
type fieldDef struct {
	Name       string
	Type       string
	Visibility string
	DocComment string
	IsReadonly bool
	IsConst    bool
	IsStatic   bool
}

// eventDef holds data for an event declaration.
type eventDef struct {
	Name       string
	Type       string
	Visibility string
	DocComment string
}

// enumMemberDef holds data for an enum member.
type enumMemberDef struct {
	Name       string
	Value      string
	DocComment string
}

// parseFile parses a C# source file using tree-sitter and returns the collected
// analysis: types (with methods/properties/fields/events), and references.
func parseFile(src []byte) *fileAnalysis {
	parser, err := treesitter.Get(treesitter.CSharp)
	if err != nil {
		return &fileAnalysis{}
	}
	defer treesitter.Put(treesitter.CSharp, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return &fileAnalysis{}
	}
	defer tree.Close()

	result := &fileAnalysis{}
	root := tree.RootNode()

	// Detect file-scoped namespace
	fileNamespace := detectFileNamespace(root, src)

	walkCSNode(root, src, fileNamespace, result)
	return result
}

// detectFileNamespace finds a file-scoped namespace declaration (no braces).
func detectFileNamespace(root *sitter.Node, src []byte) string {
	count := root.NamedChildCount()
	for i := range count {
		child := root.NamedChild(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "file_scoped_namespace_declaration" {
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				return nodeText(nameNode, src)
			}
		}
	}
	return ""
}

// walkCSNode recursively walks the C# AST, dispatching on node types.
func walkCSNode(node *sitter.Node, src []byte, namespace string, result *fileAnalysis) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "namespace_declaration":
		ns := ""
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			ns = nodeText(nameNode, src)
		}
		// Walk body with this namespace
		if body := node.ChildByFieldName("body"); body != nil {
			walkCSChildren(body, src, ns, result)
		}
		return

	case "file_scoped_namespace_declaration":
		ns := ""
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			ns = nodeText(nameNode, src)
		}
		// Walk all remaining children with this namespace
		count := node.NamedChildCount()
		for i := range count {
			child := node.NamedChild(int(i))
			if child != nil {
				walkCSNode(child, src, ns, result)
			}
		}
		return

	case "class_declaration":
		td := extractTypeDecl(node, src, namespace, source.KindClass)
		result.Types = append(result.Types, td)
		return

	case "interface_declaration":
		td := extractTypeDecl(node, src, namespace, source.KindInterface)
		result.Types = append(result.Types, td)
		return

	case "struct_declaration":
		td := extractTypeDecl(node, src, namespace, source.KindStruct)
		result.Types = append(result.Types, td)
		return

	case "record_declaration":
		td := extractTypeDecl(node, src, namespace, source.KindRecord)
		result.Types = append(result.Types, td)
		return

	case "enum_declaration":
		td := extractEnumDecl(node, src, namespace)
		result.Types = append(result.Types, td)
		return

	case "delegate_declaration":
		td := extractDelegateDecl(node, src, namespace)
		result.Types = append(result.Types, td)
		return

	case "invocation_expression":
		// Track method/function references
		if fn := node.ChildByFieldName("function"); fn != nil {
			name := nodeText(fn, src)
			if name != "" {
				result.References = append(result.References, name)
			}
		}

	case "object_creation_expression":
		if typeNode := node.ChildByFieldName("type"); typeNode != nil {
			name := nodeText(typeNode, src)
			if name != "" {
				result.References = append(result.References, name)
			}
		}
	}

	// Default: recurse into children
	walkCSChildren(node, src, namespace, result)
}

// walkCSChildren walks all named children of a node.
func walkCSChildren(node *sitter.Node, src []byte, namespace string, result *fileAnalysis) {
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child != nil {
			walkCSNode(child, src, namespace, result)
		}
	}
}

// extractTypeDecl extracts a type declaration (class/interface/struct/record).
func extractTypeDecl(node *sitter.Node, src []byte, namespace, kind string) typeDef {
	td := typeDef{
		Namespace:  namespace,
		Kind:       kind,
		Visibility: "internal", // C# default
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
	}

	// Extract name
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		td.Name = nodeText(nameNode, src)
	}

	// Extract type parameters (generics) - C# grammar uses "type_parameters" or
	// the node may be a type_parameter_list child
	if tpNode := node.ChildByFieldName("type_parameters"); tpNode != nil {
		td.Generics = nodeText(tpNode, src)
	} else {
		// Search for type_parameter_list among children
		count := node.ChildCount()
		for i := range count {
			child := node.Child(int(i))
			if child != nil && child.Type() == "type_parameter_list" {
				td.Generics = nodeText(child, src)
				break
			}
		}
	}

	// Extract base types
	if baseNode := node.ChildByFieldName("bases"); baseNode != nil {
		td.BaseTypes = extractBaseTypes(baseNode, src)
	}

	// Extract modifiers and attributes
	extractModifiers(node, src, &td)

	// Extract doc comment
	td.DocComment = findPrecedingXMLDoc(src, td.StartPos)

	// Check deprecated from attributes
	for _, attr := range td.Attributes {
		if strings.Contains(attr, "Obsolete") {
			td.Deprecated = true
			break
		}
	}

	// Walk body for members
	if body := node.ChildByFieldName("body"); body != nil {
		extractMembers(body, src, &td)
	}

	// For records with parameter list (positional records)
	if kind == source.KindRecord {
		if paramList := node.ChildByFieldName("parameters"); paramList != nil {
			extractRecordParams(paramList, src, &td)
		}
	}

	return td
}

// extractEnumDecl extracts an enum declaration.
func extractEnumDecl(node *sitter.Node, src []byte, namespace string) typeDef {
	td := typeDef{
		Namespace:  namespace,
		Kind:       source.KindEnum,
		Visibility: "internal",
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		td.Name = nodeText(nameNode, src)
	}

	extractModifiers(node, src, &td)
	td.DocComment = findPrecedingXMLDoc(src, td.StartPos)

	// Extract enum members from body
	if body := node.ChildByFieldName("body"); body != nil {
		count := body.NamedChildCount()
		for i := range count {
			child := body.NamedChild(int(i))
			if child == nil {
				continue
			}
			if child.Type() == "enum_member_declaration" {
				member := enumMemberDef{}
				if nameNode := child.ChildByFieldName("name"); nameNode != nil {
					member.Name = nodeText(nameNode, src)
				}
				if valNode := child.ChildByFieldName("value"); valNode != nil {
					member.Value = nodeText(valNode, src)
				}
				member.DocComment = findPrecedingXMLDoc(src, int(child.StartByte()))
				td.Members = append(td.Members, member)
			}
		}
	}

	return td
}

// extractDelegateDecl extracts a delegate declaration.
func extractDelegateDecl(node *sitter.Node, src []byte, namespace string) typeDef {
	td := typeDef{
		Namespace:  namespace,
		Kind:       source.KindDelegate,
		Visibility: "internal",
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		td.Name = nodeText(nameNode, src)
	}

	if tpNode := node.ChildByFieldName("type_parameters"); tpNode != nil {
		td.Generics = nodeText(tpNode, src)
	} else {
		count := node.ChildCount()
		for i := range count {
			child := node.Child(int(i))
			if child != nil && child.Type() == "type_parameter_list" {
				td.Generics = nodeText(child, src)
				break
			}
		}
	}

	extractModifiers(node, src, &td)
	td.DocComment = findPrecedingXMLDoc(src, td.StartPos)

	return td
}

// extractModifiers extracts access modifiers and attributes from a type node.
func extractModifiers(node *sitter.Node, src []byte, td *typeDef) {
	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "modifier":
			mod := nodeText(child, src)
			switch mod {
			case "public":
				td.Visibility = "public"
			case "protected":
				td.Visibility = "protected"
			case "private":
				td.Visibility = "private"
			case "internal":
				td.Visibility = "internal"
			case "static":
				td.IsStatic = true
			case "abstract":
				td.IsAbstract = true
			case "sealed":
				td.IsSealed = true
			case "partial":
				td.IsPartial = true
			}
		case "attribute_list":
			attr := nodeText(child, src)
			td.Attributes = append(td.Attributes, attr)
		}
	}
}

// extractMembers extracts methods, properties, fields, and events from a type body.
func extractMembers(body *sitter.Node, src []byte, td *typeDef) {
	count := body.NamedChildCount()
	for i := range count {
		child := body.NamedChild(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "method_declaration":
			md := extractMethodDecl(child, src)
			td.Methods = append(td.Methods, md)
		case "constructor_declaration":
			md := extractConstructorDecl(child, src, td.Name)
			td.Methods = append(td.Methods, md)
		case "property_declaration":
			pd := extractPropertyDecl(child, src)
			td.Properties = append(td.Properties, pd)
		case "field_declaration":
			fields := extractFieldDecl(child, src)
			td.Fields = append(td.Fields, fields...)
		case "event_declaration", "event_field_declaration":
			ed := extractEventDecl(child, src)
			td.Events = append(td.Events, ed)
		case "indexer_declaration":
			md := extractIndexerDecl(child, src)
			td.Methods = append(td.Methods, md)
		case "operator_declaration":
			md := extractOperatorDecl(child, src)
			td.Methods = append(td.Methods, md)
		}
	}
}

// extractMethodDecl extracts a method declaration from an AST node.
func extractMethodDecl(node *sitter.Node, src []byte) methodDef {
	md := methodDef{
		Visibility: "private", // C# default for members
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
	}

	// Name
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		md.Name = nodeText(nameNode, src)
	}

	// Return type - C# grammar uses "returns" field name
	if retNode := node.ChildByFieldName("returns"); retNode != nil {
		md.ReturnType = nodeText(retNode, src)
	} else if retNode := node.ChildByFieldName("type"); retNode != nil {
		md.ReturnType = nodeText(retNode, src)
	}

	// Type parameters (generics) - may be "type_parameters" field or type_parameter_list child
	if tpNode := node.ChildByFieldName("type_parameters"); tpNode != nil {
		md.Generics = nodeText(tpNode, src)
	} else {
		count := node.ChildCount()
		for i := range count {
			child := node.Child(int(i))
			if child != nil && child.Type() == "type_parameter_list" {
				md.Generics = nodeText(child, src)
				break
			}
		}
	}

	// Parameters
	if paramList := node.ChildByFieldName("parameters"); paramList != nil {
		md.Params = extractParams(paramList, src)
	}

	// Modifiers
	extractMethodModifiers(node, src, &md)

	// Check if extension method (first param has 'this' modifier)
	if len(md.Params) > 0 && md.Params[0].IsThis {
		md.IsExtension = true
	}

	// Async detection from return type
	if strings.HasPrefix(md.ReturnType, "async ") {
		md.IsAsync = true
		md.ReturnType = strings.TrimPrefix(md.ReturnType, "async ")
	}

	// Build signature
	md.Signature = buildMethodSignature(md)

	// Doc comment
	md.DocComment = findPrecedingXMLDoc(src, md.StartPos)

	return md
}

// extractConstructorDecl extracts a constructor declaration.
func extractConstructorDecl(node *sitter.Node, src []byte, typeName string) methodDef {
	md := methodDef{
		Name:       typeName,
		Visibility: "private",
		ReturnType: "",
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		md.Name = nodeText(nameNode, src)
	}

	if paramList := node.ChildByFieldName("parameters"); paramList != nil {
		md.Params = extractParams(paramList, src)
	}

	extractMethodModifiers(node, src, &md)
	md.Signature = buildMethodSignature(md)
	md.DocComment = findPrecedingXMLDoc(src, md.StartPos)

	return md
}

// extractIndexerDecl extracts an indexer declaration as a method.
func extractIndexerDecl(node *sitter.Node, src []byte) methodDef {
	md := methodDef{
		Name:       "this[]",
		Visibility: "private",
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
	}

	if retNode := node.ChildByFieldName("type"); retNode != nil {
		md.ReturnType = nodeText(retNode, src)
	}

	if paramList := node.ChildByFieldName("parameters"); paramList != nil {
		md.Params = extractParams(paramList, src)
	}

	extractMethodModifiers(node, src, &md)
	md.Signature = buildMethodSignature(md)
	md.DocComment = findPrecedingXMLDoc(src, md.StartPos)

	return md
}

// extractOperatorDecl extracts an operator declaration as a method.
func extractOperatorDecl(node *sitter.Node, src []byte) methodDef {
	md := methodDef{
		Visibility: "public",
		IsStatic:   true,
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
	}

	// Operator name is usually the operator symbol
	text := nodeText(node, src)
	if idx := strings.Index(text, "operator"); idx >= 0 {
		afterOp := text[idx+8:]
		afterOp = strings.TrimSpace(afterOp)
		if parenIdx := strings.Index(afterOp, "("); parenIdx > 0 {
			md.Name = "operator" + strings.TrimSpace(afterOp[:parenIdx])
		} else {
			md.Name = "operator"
		}
	}

	if retNode := node.ChildByFieldName("type"); retNode != nil {
		md.ReturnType = nodeText(retNode, src)
	}

	if paramList := node.ChildByFieldName("parameters"); paramList != nil {
		md.Params = extractParams(paramList, src)
	}

	extractMethodModifiers(node, src, &md)
	md.Signature = buildMethodSignature(md)
	md.DocComment = findPrecedingXMLDoc(src, md.StartPos)

	return md
}

// extractMethodModifiers extracts modifiers from a method declaration node.
func extractMethodModifiers(node *sitter.Node, src []byte, md *methodDef) {
	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "modifier":
			mod := nodeText(child, src)
			switch mod {
			case "public":
				md.Visibility = "public"
			case "protected":
				md.Visibility = "protected"
			case "private":
				md.Visibility = "private"
			case "internal":
				md.Visibility = "internal"
			case "static":
				md.IsStatic = true
			case "abstract":
				md.IsAbstract = true
			case "virtual":
				md.IsVirtual = true
			case "override":
				md.IsOverride = true
			case "async":
				md.IsAsync = true
			}
		case "attribute_list":
			attr := nodeText(child, src)
			md.Attributes = append(md.Attributes, attr)
		}
	}
}

// extractParams extracts parameters from a parameter_list node.
func extractParams(paramList *sitter.Node, src []byte) []paramDef {
	var params []paramDef
	count := paramList.NamedChildCount()
	for i := range count {
		child := paramList.NamedChild(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "parameter" {
			p := paramDef{}

			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				p.Name = nodeText(nameNode, src)
			}
			if typeNode := child.ChildByFieldName("type"); typeNode != nil {
				p.Type = nodeText(typeNode, src)
			}
			if defNode := child.ChildByFieldName("default_value"); defNode != nil {
				p.HasDefault = true
				p.Default = nodeText(defNode, src)
			}

			// Check for parameter modifiers (ref, out, in, this, params)
			pCount := child.ChildCount()
			for j := range pCount {
				pChild := child.Child(int(j))
				if pChild == nil {
					continue
				}
				text := nodeText(pChild, src)
				switch text {
				case "ref":
					p.IsRef = true
				case "out":
					p.IsOut = true
				case "in":
					p.IsIn = true
				case "this":
					p.IsThis = true
				case "params":
					p.IsParams = true
				}
			}

			params = append(params, p)
		}
	}
	return params
}

// extractPropertyDecl extracts a property declaration.
func extractPropertyDecl(node *sitter.Node, src []byte) propertyDef {
	pd := propertyDef{
		Visibility: "private",
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		pd.Name = nodeText(nameNode, src)
	}
	if typeNode := node.ChildByFieldName("type"); typeNode != nil {
		pd.Type = nodeText(typeNode, src)
	}

	// Extract visibility
	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "modifier" {
			mod := nodeText(child, src)
			switch mod {
			case "public":
				pd.Visibility = "public"
			case "protected":
				pd.Visibility = "protected"
			case "private":
				pd.Visibility = "private"
			case "internal":
				pd.Visibility = "internal"
			}
		}
	}

	// Detect accessors
	if accessorList := node.ChildByFieldName("accessors"); accessorList != nil {
		aCount := accessorList.NamedChildCount()
		for i := range aCount {
			accessor := accessorList.NamedChild(int(i))
			if accessor == nil {
				continue
			}
			text := nodeText(accessor, src)
			if strings.Contains(text, "get") {
				pd.HasGetter = true
			}
			if strings.Contains(text, "set") {
				pd.HasSetter = true
			}
			if strings.Contains(text, "init") {
				pd.HasInit = true
			}
		}
		// Auto-property if accessors have no bodies
		if pd.HasGetter && (pd.HasSetter || pd.HasInit) {
			pd.IsAuto = !strings.Contains(nodeText(accessorList, src), "{")
		}
	}

	pd.DocComment = findPrecedingXMLDoc(src, pd.StartPos)
	return pd
}

// extractFieldDecl extracts field declarations (may have multiple declarators).
func extractFieldDecl(node *sitter.Node, src []byte) []fieldDef {
	visibility := "private"
	isReadonly := false
	isConst := false
	isStatic := false
	typeName := ""

	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "modifier":
			mod := nodeText(child, src)
			switch mod {
			case "public":
				visibility = "public"
			case "protected":
				visibility = "protected"
			case "private":
				visibility = "private"
			case "internal":
				visibility = "internal"
			case "readonly":
				isReadonly = true
			case "const":
				isConst = true
			case "static":
				isStatic = true
			}
		case "variable_declaration":
			if typeNode := child.ChildByFieldName("type"); typeNode != nil {
				typeName = nodeText(typeNode, src)
			}
		}
	}

	docComment := findPrecedingXMLDoc(src, int(node.StartByte()))

	var fields []fieldDef
	// Find variable declarators in the variable_declaration child
	for i := range count {
		child := node.Child(int(i))
		if child == nil || child.Type() != "variable_declaration" {
			continue
		}
		vCount := child.NamedChildCount()
		for j := range vCount {
			declarator := child.NamedChild(int(j))
			if declarator == nil || declarator.Type() != "variable_declarator" {
				continue
			}
			name := ""
			if nameNode := declarator.ChildByFieldName("name"); nameNode != nil {
				name = nodeText(nameNode, src)
			} else {
				// Fallback: first named child
				if first := declarator.NamedChild(0); first != nil {
					name = nodeText(first, src)
				}
			}
			if name != "" {
				fields = append(fields, fieldDef{
					Name:       name,
					Type:       typeName,
					Visibility: visibility,
					DocComment: docComment,
					IsReadonly: isReadonly,
					IsConst:    isConst,
					IsStatic:   isStatic,
				})
			}
		}
	}
	return fields
}

// extractEventDecl extracts an event declaration.
func extractEventDecl(node *sitter.Node, src []byte) eventDef {
	ed := eventDef{
		Visibility: "private",
	}

	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "modifier":
			mod := nodeText(child, src)
			switch mod {
			case "public":
				ed.Visibility = "public"
			case "protected":
				ed.Visibility = "protected"
			case "private":
				ed.Visibility = "private"
			case "internal":
				ed.Visibility = "internal"
			}
		}
	}

	if typeNode := node.ChildByFieldName("type"); typeNode != nil {
		ed.Type = nodeText(typeNode, src)
	}
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		ed.Name = nodeText(nameNode, src)
	}

	ed.DocComment = findPrecedingXMLDoc(src, int(node.StartByte()))
	return ed
}

// extractBaseTypes extracts base types/interfaces from a base_list node.
func extractBaseTypes(baseNode *sitter.Node, src []byte) []string {
	var bases []string
	count := baseNode.NamedChildCount()
	for i := range count {
		child := baseNode.NamedChild(int(i))
		if child != nil {
			bases = append(bases, nodeText(child, src))
		}
	}
	return bases
}

// extractRecordParams extracts positional record parameters as properties.
func extractRecordParams(paramList *sitter.Node, src []byte, td *typeDef) {
	params := extractParams(paramList, src)
	for _, p := range params {
		td.Properties = append(td.Properties, propertyDef{
			Name:       p.Name,
			Type:       p.Type,
			Visibility: "public",
			HasGetter:  true,
			HasInit:    true,
			IsAuto:     true,
		})
	}
}

// buildMethodSignature builds a human-readable method signature string.
func buildMethodSignature(md methodDef) string {
	var sb strings.Builder

	if md.Visibility != "" {
		sb.WriteString(md.Visibility)
		sb.WriteString(" ")
	}
	if md.IsStatic {
		sb.WriteString("static ")
	}
	if md.IsAsync {
		sb.WriteString("async ")
	}
	if md.IsVirtual {
		sb.WriteString("virtual ")
	}
	if md.IsAbstract {
		sb.WriteString("abstract ")
	}
	if md.IsOverride {
		sb.WriteString("override ")
	}
	if md.ReturnType != "" {
		sb.WriteString(md.ReturnType)
		sb.WriteString(" ")
	}
	sb.WriteString(md.Name)
	if md.Generics != "" {
		sb.WriteString(md.Generics)
	}
	sb.WriteString("(")

	for i, p := range md.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		if p.IsThis {
			sb.WriteString("this ")
		}
		if p.IsRef {
			sb.WriteString("ref ")
		}
		if p.IsOut {
			sb.WriteString("out ")
		}
		if p.IsIn {
			sb.WriteString("in ")
		}
		if p.IsParams {
			sb.WriteString("params ")
		}
		sb.WriteString(p.Type)
		sb.WriteString(" ")
		sb.WriteString(p.Name)
		if p.HasDefault {
			sb.WriteString(" = ")
			sb.WriteString(p.Default)
		}
	}

	sb.WriteString(")")
	return sb.String()
}

// findPrecedingXMLDoc finds the XML documentation comment (/// lines)
// immediately preceding a declaration at byte position pos.
func findPrecedingXMLDoc(content []byte, pos int) string {
	if pos <= 0 || pos > len(content) {
		return ""
	}

	// Walk backwards from pos to find consecutive /// lines
	i := pos - 1

	// Skip whitespace
	for i >= 0 && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n' || content[i] == '\r') {
		i--
	}

	// Skip attributes [...]
	for i >= 0 && content[i] == ']' {
		depth := 1
		i--
		for i >= 0 && depth > 0 {
			if content[i] == ']' {
				depth++
			} else if content[i] == '[' {
				depth--
			}
			if depth > 0 {
				i--
			}
		}
		// Skip preceding whitespace
		for i > 0 && (content[i-1] == ' ' || content[i-1] == '\t' || content[i-1] == '\n' || content[i-1] == '\r') {
			i--
		}
	}

	// Now we should be at the end of a /// comment block (or not)
	// Find the end of the last /// line
	endOfComment := i + 1

	// Walk back to find the start of the /// block
	// First find the start of the current line
	lineStart := i
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}

	// Check if this line starts with ///
	trimmed := strings.TrimLeft(string(content[lineStart:endOfComment]), " \t")
	if !strings.HasPrefix(trimmed, "///") {
		return ""
	}

	// Collect all consecutive /// lines going upward
	var lines []string
	for {
		lineContent := string(content[lineStart:endOfComment])
		trimmed := strings.TrimLeft(lineContent, " \t")
		if !strings.HasPrefix(trimmed, "///") {
			break
		}
		lines = append([]string{lineContent}, lines...)

		// Move to previous line
		if lineStart == 0 {
			break
		}
		endOfComment = lineStart
		lineStart--
		// Skip the newline
		for lineStart > 0 && (content[lineStart] == '\n' || content[lineStart] == '\r') {
			lineStart--
		}
		// Find start of previous line
		for lineStart > 0 && content[lineStart-1] != '\n' {
			lineStart--
		}
	}

	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n")
}

// nodeText extracts the text content of a tree-sitter node.
func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	start := node.StartByte()
	end := node.EndByte()
	if start >= end || int(end) > len(src) {
		return ""
	}
	return strings.TrimSpace(string(src[start:end]))
}
