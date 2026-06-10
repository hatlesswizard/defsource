// Package php — ast.go: tree-sitter PHP AST walker and wrapper detector.
// This file is intentionally separate from phpdoc.go (regex-based PHPDoc
// parser) because the two parsers have different inputs ([]byte vs string),
// different dependencies (go-tree-sitter vs regexp), and different reasons
// to change.
package php

import (
	"context"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/php"
)

// parserPool is a package-level pool of tree-sitter PHP parsers.
// Creating a sitter.Parser involves a CGo heap allocation and grammar
// initialisation; pooling amortises that cost across the thousands of PHP
// files parsed during a crawl (CRIT-001 from ch17 smells/heuristics).
// sync.Pool is goroutine-safe: each Get/Put is atomic.
var parserPool = sync.Pool{
	New: func() any {
		p := sitter.NewParser()
		p.SetLanguage(php.GetLanguage())
		return p
	},
}

// fileAnalysis is the top-level container returned by parseFile, holding
// all AST-extracted entities from one PHP source file.
type fileAnalysis struct {
	Classes   []classDef
	Functions []functionDef
	Calls     []callRef
}

// callRef is a lightweight call-site reference collected during AST walking.
type callRef struct {
	Name string
	Kind string // "function", "method", or "static"
}

// classDef holds data extracted from a class_declaration AST node.
type classDef struct {
	Name       string
	StartPos   int
	EndPos     int
	DocComment string
	Methods    []methodDef
	Properties []propertyDef
}

// functionDef holds data extracted from a top-level function_definition node.
type functionDef struct {
	Name       string
	Signature  string
	StartPos   int
	EndPos     int
	DocComment string
}

// methodDef holds data extracted from a method_declaration AST node.
type methodDef struct {
	Name       string
	Signature  string
	Visibility string
	Static     bool
	StartPos   int
	EndPos     int
	DocComment string
}

// propertyDef holds data extracted from one property_element within a
// property_declaration AST node.
type propertyDef struct {
	Name       string
	Type       string
	Visibility string
	DocComment string
}

// parseFile parses a PHP source file using tree-sitter and returns the
// collected analysis: classes (with methods/properties), top-level
// functions, and call references.
func parseFile(src []byte) *fileAnalysis {
	p := parserPool.Get().(*sitter.Parser)
	defer parserPool.Put(p)

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return &fileAnalysis{}
	}
	defer tree.Close()

	result := &fileAnalysis{}
	root := tree.RootNode()
	walkNode(root, src, result)
	return result
}

// walkNode recursively walks the AST and dispatches on node type, collecting
// classes, functions, and call references into result.
//
// result is an output parameter — accumulation is the idiomatic Go pattern
// for recursive tree walkers (avoids allocating a new slice on every frame).
func walkNode(node *sitter.Node, src []byte, result *fileAnalysis) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "class_declaration":
		cd := extractClass(node, src)
		result.Classes = append(result.Classes, cd)
		// Recurse into class body for call expressions inside method bodies.
		if body := node.ChildByFieldName("body"); body != nil {
			count := body.NamedChildCount()
			for i := range count {
				walkNode(body.NamedChild(int(i)), src, result)
			}
		}
		return

	case "function_definition":
		fd := extractFunction(node, src)
		result.Functions = append(result.Functions, fd)
		// Walk children for calls inside the function body.
		count := node.NamedChildCount()
		for i := range count {
			walkNode(node.NamedChild(int(i)), src, result)
		}
		return

	case "function_call_expression":
		fnNode := node.ChildByFieldName("function")
		if fnNode != nil {
			name := strings.TrimSpace(string(src[fnNode.StartByte():fnNode.EndByte()]))
			if name != "" {
				result.Calls = append(result.Calls, callRef{Name: name, Kind: "function"})
			}
		}
		// Continue walking arguments etc. for nested calls.
		count := node.NamedChildCount()
		for i := range count {
			walkNode(node.NamedChild(int(i)), src, result)
		}
		return

	case "member_call_expression":
		objNode := node.ChildByFieldName("object")
		nameNode := node.ChildByFieldName("name")
		if objNode != nil && nameNode != nil {
			objText := strings.TrimSpace(string(src[objNode.StartByte():objNode.EndByte()]))
			nameText := strings.TrimSpace(string(src[nameNode.StartByte():nameNode.EndByte()]))
			if objText == "$this" && nameText != "" {
				result.Calls = append(result.Calls, callRef{Name: nameText, Kind: "method"})
			}
		}
		count := node.NamedChildCount()
		for i := range count {
			walkNode(node.NamedChild(int(i)), src, result)
		}
		return

	case "scoped_call_expression":
		scopeNode := node.ChildByFieldName("scope")
		nameNode := node.ChildByFieldName("name")
		if scopeNode != nil && nameNode != nil {
			scopeText := strings.TrimSpace(string(src[scopeNode.StartByte():scopeNode.EndByte()]))
			nameText := strings.TrimSpace(string(src[nameNode.StartByte():nameNode.EndByte()]))
			if scopeText != "" && nameText != "" {
				result.Calls = append(result.Calls, callRef{
					Name: scopeText + "::" + nameText,
					Kind: "static",
				})
			}
		}
		count := node.NamedChildCount()
		for i := range count {
			walkNode(node.NamedChild(int(i)), src, result)
		}
		return
	}

	// Default: recurse into all named children.
	count := node.NamedChildCount()
	for i := range count {
		walkNode(node.NamedChild(int(i)), src, result)
	}
}

// extractClass builds a classDef from a class_declaration AST node,
// collecting its name, methods, properties, and any preceding PHPDoc comment.
func extractClass(node *sitter.Node, src []byte) classDef {
	var cd classDef
	cd.StartPos = int(node.StartByte())
	cd.EndPos = int(node.EndByte())

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		cd.Name = string(src[nameNode.StartByte():nameNode.EndByte()])
	}

	if doc, ok := findPrecedingDoc(src, cd.StartPos); ok {
		cd.DocComment = doc
	}

	body := node.ChildByFieldName("body")
	if body == nil {
		return cd
	}

	count := body.NamedChildCount()
	for i := range count {
		child := body.NamedChild(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "method_declaration":
			cd.Methods = append(cd.Methods, extractMethod(child, src))
		case "property_declaration":
			cd.Properties = append(cd.Properties, extractProperties(child, src)...)
		}
	}

	return cd
}

// extractFunction builds a functionDef from a function_definition AST node,
// including its signature and any preceding PHPDoc comment.
func extractFunction(node *sitter.Node, src []byte) functionDef {
	var fd functionDef
	fd.StartPos = int(node.StartByte())
	fd.EndPos = int(node.EndByte())

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		fd.Name = string(src[nameNode.StartByte():nameNode.EndByte()])
	}

	paramsNode := node.ChildByFieldName("parameters")
	returnTypeNode := node.ChildByFieldName("return_type")
	fd.Signature = buildSignatureFromNode(fd.Name, paramsNode, returnTypeNode, src)

	if doc, ok := findPrecedingDoc(src, fd.StartPos); ok {
		fd.DocComment = doc
	}

	return fd
}

// extractMethod builds a methodDef from a method_declaration AST node,
// including visibility, static modifier, signature, and any preceding PHPDoc comment.
func extractMethod(node *sitter.Node, src []byte) methodDef {
	var md methodDef
	md.StartPos = int(node.StartByte())
	md.EndPos = int(node.EndByte())
	md.Visibility = "public" // PHP default

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		md.Name = string(src[nameNode.StartByte():nameNode.EndByte()])
	}

	// Look for visibility and static modifiers among children.
	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "visibility_modifier":
			md.Visibility = strings.TrimSpace(string(src[child.StartByte():child.EndByte()]))
		case "static_modifier":
			md.Static = true
		}
	}

	paramsNode := node.ChildByFieldName("parameters")
	returnTypeNode := node.ChildByFieldName("return_type")
	md.Signature = buildSignatureFromNode(md.Name, paramsNode, returnTypeNode, src)

	if doc, ok := findPrecedingDoc(src, md.StartPos); ok {
		md.DocComment = doc
	}

	return md
}

// extractProperties handles a property_declaration node which may declare
// multiple property_element entries. Returns one propertyDef per element.
func extractProperties(node *sitter.Node, src []byte) []propertyDef {
	visibility := "public"
	var typeName string

	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "visibility_modifier" {
			visibility = strings.TrimSpace(string(src[child.StartByte():child.EndByte()]))
		}
	}

	if typeNode := node.ChildByFieldName("type"); typeNode != nil {
		typeName = strings.TrimSpace(string(src[typeNode.StartByte():typeNode.EndByte()]))
	}

	doc, _ := findPrecedingDoc(src, int(node.StartByte()))

	if typeName == "" && doc != "" {
		parsed := parsePhpDoc(doc)
		if parsed.VarType != "" {
			typeName = parsed.VarType
		}
	}

	var props []propertyDef
	namedCount := node.NamedChildCount()
	for i := range namedCount {
		child := node.NamedChild(int(i))
		if child == nil || child.Type() != "property_element" {
			continue
		}

		// The property_element contains a variable_name node like $foo.
		var name string
		ec := child.NamedChildCount()
		for j := range ec {
			sub := child.NamedChild(int(j))
			if sub == nil {
				continue
			}
			if sub.Type() == "variable_name" {
				name = strings.TrimSpace(string(src[sub.StartByte():sub.EndByte()]))
				break
			}
		}
		if name == "" {
			// Fallback: take the raw text of the property_element up to "=" or end.
			text := string(src[child.StartByte():child.EndByte()])
			if idx := strings.Index(text, "="); idx >= 0 {
				text = text[:idx]
			}
			name = strings.TrimSpace(text)
		}
		if name == "" {
			continue
		}
		if !strings.HasPrefix(name, "$") {
			name = "$" + name
		}

		props = append(props, propertyDef{
			Name:       name,
			Type:       typeName,
			Visibility: visibility,
			DocComment: doc,
		})
	}

	return props
}

// buildSignatureFromNode builds a PHP function/method signature string from
// the given name, parameter-list node, and optional return-type node.
func buildSignatureFromNode(name string, paramsNode, returnTypeNode *sitter.Node, src []byte) string {
	var b strings.Builder
	b.WriteString("function ")
	b.WriteString(name)
	b.WriteString("( ")

	if paramsNode != nil {
		var parts []string
		count := paramsNode.NamedChildCount()
		for i := range count {
			p := paramsNode.NamedChild(int(i))
			if p == nil {
				continue
			}
			t := p.Type()
			if t != "simple_parameter" && t != "variadic_parameter" && t != "property_promotion_parameter" {
				continue
			}
			text := strings.TrimSpace(string(src[p.StartByte():p.EndByte()]))
			if text != "" {
				parts = append(parts, text)
			}
		}
		b.WriteString(strings.Join(parts, ", "))
	}

	b.WriteString(" )")

	if returnTypeNode != nil {
		rt := strings.TrimSpace(string(src[returnTypeNode.StartByte():returnTypeNode.EndByte()]))
		if rt != "" {
			b.WriteString(": ")
			b.WriteString(rt)
		}
	}

	return b.String()
}

// findFunctionBody searches root for the first function_definition node and
// returns its body child. Returns nil if no function_definition is found.
// This is extracted from the closure in detectWrapperAST to eliminate hidden
// temporal coupling via captured variables (ch15 Pass 6).
func findFunctionBody(root *sitter.Node) *sitter.Node {
	if root == nil {
		return nil
	}
	if root.Type() == "function_definition" {
		return root.ChildByFieldName("body")
	}
	count := root.NamedChildCount()
	for i := range count {
		if body := findFunctionBody(root.NamedChild(int(i))); body != nil {
			return body
		}
	}
	return nil
}

// detectWrapperAST analyzes a function/method body source and detects whether
// it is a thin wrapper around another call. Returns (isWrapper, targetName,
// targetKind) where targetKind is "function", "self_method", or
// "static_method". The src argument should be the full source of the
// function body, typically starting with "{" and ending with "}".
func detectWrapperAST(src []byte, builtins map[string]bool) (bool, string, string) {
	// Wrap with a dummy function so tree-sitter has a complete program.
	wrapped := []byte("<?php function __wrapper() " + string(src))

	p := parserPool.Get().(*sitter.Parser)
	defer parserPool.Put(p)

	tree, err := p.ParseCtx(context.Background(), nil, wrapped)
	if err != nil || tree == nil {
		return false, "", ""
	}
	defer tree.Close()

	bodyNode := findFunctionBody(tree.RootNode())
	if bodyNode == nil {
		return false, "", ""
	}

	// Collect non-comment statement children of the body.
	var stmts []*sitter.Node
	count := bodyNode.NamedChildCount()
	for i := range count {
		child := bodyNode.NamedChild(int(i))
		if child == nil {
			continue
		}
		t := child.Type()
		if t == "comment" {
			continue
		}
		stmts = append(stmts, child)
	}

	if len(stmts) == 0 || len(stmts) > 5 {
		return false, "", ""
	}

	// Look for a single return_statement among the statements.
	var returnStmt *sitter.Node
	returnCount := 0
	for _, s := range stmts {
		if s.Type() == "return_statement" {
			returnStmt = s
			returnCount++
		}
	}

	if returnCount == 1 {
		// Find the call expression child of the return statement.
		var callExpr *sitter.Node
		c := returnStmt.NamedChildCount()
		for i := range c {
			ch := returnStmt.NamedChild(int(i))
			if ch == nil {
				continue
			}
			t := ch.Type()
			if t == "function_call_expression" || t == "member_call_expression" || t == "scoped_call_expression" {
				callExpr = ch
				break
			}
		}

		if callExpr != nil {
			return classifyCall(callExpr, wrapped, builtins)
		}
	}

	// Single non-return bare call statement → void wrapper.
	if len(stmts) == 1 {
		s := stmts[0]
		if s.Type() == "expression_statement" {
			c := s.NamedChildCount()
			for i := range c {
				ch := s.NamedChild(int(i))
				if ch == nil {
					continue
				}
				t := ch.Type()
				if t == "function_call_expression" || t == "member_call_expression" || t == "scoped_call_expression" {
					return classifyCall(ch, wrapped, builtins)
				}
			}
		}
	}

	return false, "", ""
}

// classifyCall examines a single call-expression node and determines whether
// it represents a wrapper target. Returns (isWrapper, name, kind).
func classifyCall(callExpr *sitter.Node, src []byte, builtins map[string]bool) (bool, string, string) {
	switch callExpr.Type() {
	case "function_call_expression":
		fn := callExpr.ChildByFieldName("function")
		if fn == nil {
			return false, "", ""
		}
		name := strings.TrimSpace(string(src[fn.StartByte():fn.EndByte()]))
		if name == "" {
			return false, "", ""
		}
		if builtins != nil && builtins[name] {
			return false, "", ""
		}
		return true, name, "function"

	case "member_call_expression":
		obj := callExpr.ChildByFieldName("object")
		nm := callExpr.ChildByFieldName("name")
		if obj == nil || nm == nil {
			return false, "", ""
		}
		objText := strings.TrimSpace(string(src[obj.StartByte():obj.EndByte()]))
		nameText := strings.TrimSpace(string(src[nm.StartByte():nm.EndByte()]))
		if objText == "$this" && nameText != "" {
			return true, nameText, "self_method"
		}
		return false, "", ""

	case "scoped_call_expression":
		scope := callExpr.ChildByFieldName("scope")
		nm := callExpr.ChildByFieldName("name")
		if scope == nil || nm == nil {
			return false, "", ""
		}
		scopeText := strings.TrimSpace(string(src[scope.StartByte():scope.EndByte()]))
		nameText := strings.TrimSpace(string(src[nm.StartByte():nm.EndByte()]))
		if scopeText == "" || nameText == "" {
			return false, "", ""
		}
		lower := strings.ToLower(scopeText)
		if lower == "self" || lower == "static" {
			return true, nameText, "self_method"
		}
		return true, scopeText + "::" + nameText, "static_method"
	}

	return false, "", ""
}
