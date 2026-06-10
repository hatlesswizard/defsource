package ruby

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// fileAnalysis holds all entities extracted from one Ruby source file.
type fileAnalysis struct {
	Classes   []classDef
	Modules   []moduleDef
	Functions []methodDef
	Calls     []callRef
}

// callRef is a lightweight call-site reference.
type callRef struct {
	Name string
	Kind string // "method", "function"
}

// classDef holds data extracted from a Ruby class definition.
type classDef struct {
	Name             string
	QualifiedName    string
	Superclass       string
	StartPos         int
	EndPos           int
	StartLine        int
	DocComment       string
	Methods          []methodDef
	SingletonMethods []methodDef
	Attributes       []attrDef
	Includes         []string // included modules
	Extends          []string // extended modules
	Prepends         []string // prepended modules
	Constants        []constDef
}

// moduleDef holds data extracted from a Ruby module definition.
type moduleDef struct {
	Name             string
	QualifiedName    string
	StartPos         int
	EndPos           int
	StartLine        int
	DocComment       string
	Methods          []methodDef
	SingletonMethods []methodDef
	Includes         []string
	Extends          []string
	Prepends         []string
	Constants        []constDef
}

// methodDef holds data extracted from a Ruby method definition.
type methodDef struct {
	Name       string
	Params     []paramDef
	Visibility string // "public", "protected", "private"
	StartPos   int
	EndPos     int
	StartLine  int
	DocComment string
}

// paramDef holds data for a method parameter.
type paramDef struct {
	Name        string
	Type        string // from YARD annotations
	Default     string
	Splat       bool // *args
	DoubleSplat bool // **opts
	Block       bool // &block
}

// attrDef holds data for an attr_reader/writer/accessor.
type attrDef struct {
	Name       string
	Type       string
	Kind       string // "reader", "writer", "accessor"
	Visibility string
}

// constDef holds data for a constant definition.
type constDef struct {
	Name     string
	Value    string
	StartPos int
	EndPos   int
}

// parseFile parses a Ruby source file using tree-sitter and returns
// the collected analysis.
func parseFile(src []byte) *fileAnalysis {
	parser, err := treesitter.Get(treesitter.Ruby)
	if err != nil {
		return &fileAnalysis{}
	}
	defer treesitter.Put(treesitter.Ruby, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return &fileAnalysis{}
	}
	defer tree.Close()

	result := &fileAnalysis{}
	root := tree.RootNode()
	walkNode(root, src, result, nil)
	return result
}

// walkNode recursively walks the Ruby AST collecting definitions.
// namespace tracks the current Module::Class nesting for qualified names.
func walkNode(node *sitter.Node, src []byte, result *fileAnalysis, namespace []string) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "class":
		cd := extractClass(node, src, namespace)
		result.Classes = append(result.Classes, cd)
		// Also recurse into body to find nested classes/modules
		bodyNode := node.ChildByFieldName("body")
		if bodyNode != nil {
			walkNodeChildren(bodyNode, src, result, append(namespace, cd.Name))
		}
		return

	case "module":
		md := extractModule(node, src, namespace)
		result.Modules = append(result.Modules, md)
		// Recurse into body to find nested classes/modules
		bodyNode := node.ChildByFieldName("body")
		if bodyNode != nil {
			walkNodeChildren(bodyNode, src, result, append(namespace, md.Name))
		}
		return

	case "method":
		// Top-level method (not inside a class/module body)
		if len(namespace) == 0 {
			md := extractMethod(node, src, "public")
			result.Functions = append(result.Functions, md)
		}
		return

	case "call":
		extractCallRef(node, src, result)
	}

	// Default: recurse into all children (both named and unnamed).
	walkNodeChildren(node, src, result, namespace)
}

// walkNodeChildren recurses into all named children of a node.
func walkNodeChildren(node *sitter.Node, src []byte, result *fileAnalysis, namespace []string) {
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		walkNode(node.NamedChild(i), src, result, namespace)
	}
}

// extractClass builds a classDef from a class AST node.
func extractClass(node *sitter.Node, src []byte, namespace []string) classDef {
	cd := classDef{
		StartPos:  int(node.StartByte()),
		EndPos:    int(node.EndByte()),
		StartLine: int(node.StartPoint().Row) + 1,
	}

	// Extract class name
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		cd.Name = nodeText(nameNode, src)
	}

	// Build qualified name
	if len(namespace) > 0 {
		cd.QualifiedName = strings.Join(namespace, "::") + "::" + cd.Name
	} else {
		cd.QualifiedName = cd.Name
	}

	// Handle scope resolution in name (e.g., "class Foo::Bar")
	if strings.Contains(cd.Name, "::") {
		cd.QualifiedName = cd.Name
		parts := strings.Split(cd.Name, "::")
		cd.Name = parts[len(parts)-1]
	}

	// Extract superclass — the superclass field contains "< Parent",
	// we need to find the constant child node.
	superNode := node.ChildByFieldName("superclass")
	if superNode != nil {
		cd.Superclass = extractSuperclassName(superNode, src)
	}

	// Extract preceding YARD comment
	cd.DocComment = findPrecedingComment(src, cd.StartPos)

	// Walk class body for methods, attributes, mixins
	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		walkClassBody(bodyNode, src, &cd.Methods, &cd.SingletonMethods, &cd.Attributes, &cd.Includes, &cd.Extends, &cd.Prepends, &cd.Constants)
	}

	return cd
}

// extractSuperclassName extracts just the class name from a superclass node.
// The superclass node contains "< Parent" as text; we find the named child.
func extractSuperclassName(node *sitter.Node, src []byte) string {
	// Look for constant or scope_resolution child
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "constant", "scope_resolution":
			return nodeText(child, src)
		}
	}
	// Fallback: strip "< " prefix
	text := nodeText(node, src)
	text = strings.TrimPrefix(text, "<")
	return strings.TrimSpace(text)
}

// extractModule builds a moduleDef from a module AST node.
func extractModule(node *sitter.Node, src []byte, namespace []string) moduleDef {
	md := moduleDef{
		StartPos:  int(node.StartByte()),
		EndPos:    int(node.EndByte()),
		StartLine: int(node.StartPoint().Row) + 1,
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		md.Name = nodeText(nameNode, src)
	}

	if len(namespace) > 0 {
		md.QualifiedName = strings.Join(namespace, "::") + "::" + md.Name
	} else {
		md.QualifiedName = md.Name
	}

	// Handle scope resolution in name (e.g., "module Foo::Bar")
	if strings.Contains(md.Name, "::") {
		md.QualifiedName = md.Name
		parts := strings.Split(md.Name, "::")
		md.Name = parts[len(parts)-1]
	}

	md.DocComment = findPrecedingComment(src, md.StartPos)

	bodyNode := node.ChildByFieldName("body")
	if bodyNode != nil {
		walkClassBody(bodyNode, src, &md.Methods, &md.SingletonMethods, nil, &md.Includes, &md.Extends, &md.Prepends, &md.Constants)
	}

	return md
}

// walkClassBody walks the body of a class or module, extracting methods,
// attributes, mixins, and constants. It tracks visibility modifiers.
func walkClassBody(body *sitter.Node, src []byte, methods *[]methodDef, singletonMethods *[]methodDef, attrs *[]attrDef, includes, extends, prepends *[]string, constants *[]constDef) {
	if body == nil {
		return
	}

	currentVisibility := "public"

	count := int(body.NamedChildCount())
	for i := 0; i < count; i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "method":
			md := extractMethod(child, src, currentVisibility)
			*methods = append(*methods, md)

		case "singleton_method":
			md := extractSingletonMethod(child, src)
			*singletonMethods = append(*singletonMethods, md)

		case "singleton_class":
			// class << self block
			sBody := child.ChildByFieldName("body")
			if sBody != nil {
				walkSingletonBlock(sBody, src, singletonMethods)
			}

		case "call":
			handleClassLevelCall(child, src, &currentVisibility, attrs, includes, extends, prepends)

		case "identifier":
			// Bare visibility modifiers: private, protected, public
			text := nodeText(child, src)
			switch text {
			case "private":
				currentVisibility = "private"
			case "protected":
				currentVisibility = "protected"
			case "public":
				currentVisibility = "public"
			}

		case "assignment":
			if constants != nil {
				cd := extractConstant(child, src)
				if cd.Name != "" {
					*constants = append(*constants, cd)
				}
			}

		case "class", "module":
			// Nested class/module — these are discovered separately at the
			// top level via walkNode's recursion into body_statement
		}
	}
}

// walkSingletonBlock walks a "class << self" body to extract singleton methods.
func walkSingletonBlock(body *sitter.Node, src []byte, singletonMethods *[]methodDef) {
	count := int(body.NamedChildCount())
	for i := 0; i < count; i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type() == "method" {
			md := extractMethod(child, src, "public")
			*singletonMethods = append(*singletonMethods, md)
		}
	}
}

// handleClassLevelCall processes method calls at class body level, handling
// visibility modifiers, attr_* declarations, and module mixins.
func handleClassLevelCall(node *sitter.Node, src []byte, visibility *string, attrs *[]attrDef, includes, extends, prepends *[]string) {
	methodNode := node.ChildByFieldName("method")
	if methodNode == nil {
		return
	}
	methodName := nodeText(methodNode, src)

	switch methodName {
	case "private":
		args := collectCallArgs(node, src)
		if len(args) == 0 {
			*visibility = "private"
		}

	case "protected":
		args := collectCallArgs(node, src)
		if len(args) == 0 {
			*visibility = "protected"
		}

	case "public":
		args := collectCallArgs(node, src)
		if len(args) == 0 {
			*visibility = "public"
		}

	case "attr_reader":
		if attrs != nil {
			names := collectSymbolArgs(node, src)
			for _, name := range names {
				*attrs = append(*attrs, attrDef{
					Name:       name,
					Kind:       "reader",
					Visibility: *visibility,
				})
			}
		}

	case "attr_writer":
		if attrs != nil {
			names := collectSymbolArgs(node, src)
			for _, name := range names {
				*attrs = append(*attrs, attrDef{
					Name:       name,
					Kind:       "writer",
					Visibility: *visibility,
				})
			}
		}

	case "attr_accessor":
		if attrs != nil {
			names := collectSymbolArgs(node, src)
			for _, name := range names {
				*attrs = append(*attrs, attrDef{
					Name:       name,
					Kind:       "accessor",
					Visibility: *visibility,
				})
			}
		}

	case "include":
		if includes != nil {
			args := collectCallArgs(node, src)
			*includes = append(*includes, args...)
		}

	case "extend":
		if extends != nil {
			args := collectCallArgs(node, src)
			*extends = append(*extends, args...)
		}

	case "prepend":
		if prepends != nil {
			args := collectCallArgs(node, src)
			*prepends = append(*prepends, args...)
		}
	}
}

// extractMethod builds a methodDef from a Ruby method node.
func extractMethod(node *sitter.Node, src []byte, visibility string) methodDef {
	md := methodDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		StartLine:  int(node.StartPoint().Row) + 1,
		Visibility: visibility,
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		md.Name = nodeText(nameNode, src)
	}

	// Extract parameters
	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode != nil {
		md.Params = extractParams(paramsNode, src)
	}

	// Extract YARD comment
	md.DocComment = findPrecedingComment(src, md.StartPos)

	return md
}

// extractSingletonMethod builds a methodDef from a singleton_method node (def self.foo).
func extractSingletonMethod(node *sitter.Node, src []byte) methodDef {
	md := methodDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		StartLine:  int(node.StartPoint().Row) + 1,
		Visibility: "public",
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		md.Name = nodeText(nameNode, src)
	}

	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode != nil {
		md.Params = extractParams(paramsNode, src)
	}

	md.DocComment = findPrecedingComment(src, md.StartPos)

	return md
}

// extractParams extracts parameters from a method_parameters node.
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
		case "identifier":
			params = append(params, paramDef{
				Name: nodeText(child, src),
			})

		case "optional_parameter":
			p := paramDef{}
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				p.Name = nodeText(nameNode, src)
			}
			valNode := child.ChildByFieldName("value")
			if valNode != nil {
				p.Default = nodeText(valNode, src)
			}
			params = append(params, p)

		case "splat_parameter":
			p := paramDef{Splat: true}
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				p.Name = nodeText(nameNode, src)
			} else {
				p.Name = "args"
			}
			params = append(params, p)

		case "hash_splat_parameter":
			p := paramDef{DoubleSplat: true}
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				p.Name = nodeText(nameNode, src)
			} else {
				p.Name = "opts"
			}
			params = append(params, p)

		case "block_parameter":
			p := paramDef{Block: true}
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				p.Name = nodeText(nameNode, src)
			} else {
				p.Name = "block"
			}
			params = append(params, p)

		case "keyword_parameter":
			p := paramDef{}
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				p.Name = nodeText(nameNode, src)
			}
			valNode := child.ChildByFieldName("value")
			if valNode != nil {
				p.Default = nodeText(valNode, src)
			}
			params = append(params, p)
		}
	}

	return params
}

// extractConstant extracts a constant definition from an assignment node.
func extractConstant(node *sitter.Node, src []byte) constDef {
	cd := constDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	leftNode := node.ChildByFieldName("left")
	if leftNode != nil {
		name := nodeText(leftNode, src)
		// Ruby constants start with uppercase
		if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
			cd.Name = name
		}
	}

	rightNode := node.ChildByFieldName("right")
	if rightNode != nil {
		cd.Value = nodeText(rightNode, src)
	}

	return cd
}

// extractCallRef collects a method call reference for the index.
func extractCallRef(node *sitter.Node, src []byte, result *fileAnalysis) {
	methodNode := node.ChildByFieldName("method")
	if methodNode == nil {
		return
	}
	name := nodeText(methodNode, src)
	if name == "" {
		return
	}

	receiverNode := node.ChildByFieldName("receiver")
	if receiverNode != nil {
		receiver := nodeText(receiverNode, src)
		if receiver != "" && receiver != "self" {
			result.Calls = append(result.Calls, callRef{
				Name: receiver + "." + name,
				Kind: "method",
			})
			return
		}
	}
	result.Calls = append(result.Calls, callRef{Name: name, Kind: "function"})
}

// collectCallArgs collects the text of all arguments to a method call node.
func collectCallArgs(node *sitter.Node, src []byte) []string {
	argsNode := node.ChildByFieldName("arguments")
	if argsNode == nil {
		return nil
	}

	var args []string
	count := int(argsNode.NamedChildCount())
	for i := 0; i < count; i++ {
		child := argsNode.NamedChild(i)
		if child == nil {
			continue
		}
		text := nodeText(child, src)
		// Strip leading colon for symbols
		text = strings.TrimPrefix(text, ":")
		if text != "" {
			args = append(args, text)
		}
	}
	return args
}

// collectSymbolArgs collects symbol arguments from a call node,
// stripping the leading colon.
func collectSymbolArgs(node *sitter.Node, src []byte) []string {
	argsNode := node.ChildByFieldName("arguments")
	if argsNode == nil {
		return nil
	}

	var names []string
	count := int(argsNode.NamedChildCount())
	for i := 0; i < count; i++ {
		child := argsNode.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type() == "simple_symbol" || child.Type() == "symbol" {
			text := nodeText(child, src)
			text = strings.TrimPrefix(text, ":")
			if text != "" {
				names = append(names, text)
			}
		}
	}
	return names
}

// findPrecedingComment extracts the YARD comment block immediately
// preceding a declaration at the given byte position.
func findPrecedingComment(src []byte, pos int) string {
	if pos <= 0 || pos > len(src) {
		return ""
	}

	// Walk backward from pos to find comment lines
	i := pos - 1

	// Skip whitespace (spaces, tabs, single newline)
	for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i--
	}

	if i < 0 {
		return ""
	}

	// Now we are at the last non-whitespace char before the declaration.
	// Find what line this belongs to and check if it's a comment.
	var commentLines []string

	for {
		// Find start of current line
		lineStart := i
		for lineStart > 0 && src[lineStart-1] != '\n' {
			lineStart--
		}

		line := strings.TrimSpace(string(src[lineStart : i+1]))
		if !strings.HasPrefix(line, "#") {
			break
		}

		commentLines = append(commentLines, line)

		// Move to previous line
		if lineStart == 0 {
			break
		}
		// Move past the \n
		i = lineStart - 2
		if i < 0 {
			break
		}

		// Skip trailing whitespace on previous line
		for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\r') {
			i--
		}
		if i < 0 {
			break
		}
		// If we hit a newline, that means an empty line — stop
		if src[i] == '\n' {
			break
		}
	}

	if len(commentLines) == 0 {
		return ""
	}

	// Reverse the lines (we collected them bottom-up)
	for i, j := 0, len(commentLines)-1; i < j; i, j = i+1, j-1 {
		commentLines[i], commentLines[j] = commentLines[j], commentLines[i]
	}

	return strings.Join(commentLines, "\n")
}

// nodeText returns the source text for a tree-sitter node.
func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	start := node.StartByte()
	end := node.EndByte()
	if int(start) >= len(src) || int(end) > len(src) {
		return ""
	}
	return strings.TrimSpace(string(src[start:end]))
}
