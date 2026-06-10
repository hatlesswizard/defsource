package javascript

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// detectWrapper analyzes a function/method body source and detects whether
// it is a thin wrapper around another call. Returns (isWrapper, targetName,
// targetKind) where targetKind is "function", "method", or "module_method".
//
// Wrapper patterns detected:
// 1. return anotherFunction(...args)
// 2. return this.inner.method(...)
// 3. return obj.method(...)
// 4. Single call statement (void wrapper)
func detectWrapper(src []byte, builtins map[string]bool) (bool, string, string) {
	// Wrap with a function so tree-sitter has a complete program.
	wrapped := []byte("function __wrapper() " + string(src))

	parser, err := treesitter.Get(treesitter.JavaScript)
	if err != nil {
		return false, "", ""
	}
	defer treesitter.Put(treesitter.JavaScript, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, wrapped)
	if err != nil || tree == nil {
		return false, "", ""
	}
	defer tree.Close()

	bodyNode := findFunctionBody(tree.RootNode())
	if bodyNode == nil {
		return false, "", ""
	}

	// Collect non-comment statement children.
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

	if len(stmts) == 0 || len(stmts) > 2 {
		return false, "", ""
	}

	// Look for a single return statement.
	var returnStmt *sitter.Node
	returnCount := 0
	for _, s := range stmts {
		if s.Type() == "return_statement" {
			returnStmt = s
			returnCount++
		}
	}

	if returnCount == 1 {
		// Find call expression in the return statement.
		var callExpr *sitter.Node
		c := returnStmt.NamedChildCount()
		for i := range c {
			ch := returnStmt.NamedChild(int(i))
			if ch == nil {
				continue
			}
			if ch.Type() == "call_expression" {
				callExpr = ch
				break
			}
		}

		if callExpr != nil {
			return classifyCall(callExpr, wrapped, builtins)
		}
	}

	// Single expression statement — void wrapper.
	if len(stmts) == 1 {
		s := stmts[0]
		if s.Type() == "expression_statement" {
			c := s.NamedChildCount()
			for i := range c {
				ch := s.NamedChild(int(i))
				if ch != nil && ch.Type() == "call_expression" {
					return classifyCall(ch, wrapped, builtins)
				}
			}
		}
	}

	return false, "", ""
}

// findFunctionBody searches for the first function_declaration and returns
// its body child.
func findFunctionBody(root *sitter.Node) *sitter.Node {
	if root == nil {
		return nil
	}
	if root.Type() == "function_declaration" || root.Type() == "function" {
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

// classifyCall examines a call_expression node and determines if it represents
// a wrapper target. Returns (isWrapper, name, kind).
func classifyCall(callExpr *sitter.Node, src []byte, builtins map[string]bool) (bool, string, string) {
	fn := callExpr.ChildByFieldName("function")
	if fn == nil {
		return false, "", ""
	}

	switch fn.Type() {
	case "identifier":
		// Simple function call: someFunction(...)
		name := strings.TrimSpace(string(src[fn.StartByte():fn.EndByte()]))
		if name == "" {
			return false, "", ""
		}
		if builtins != nil && builtins[name] {
			return false, "", ""
		}
		return true, name, "function"

	case "member_expression":
		// Method call: obj.method(...) or this.method(...)
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("property")
		if obj == nil || prop == nil {
			return false, "", ""
		}

		objText := strings.TrimSpace(string(src[obj.StartByte():obj.EndByte()]))
		propText := strings.TrimSpace(string(src[prop.StartByte():prop.EndByte()]))

		if objText == "" || propText == "" {
			return false, "", ""
		}

		fullName := objText + "." + propText

		// Check builtins for the full member expression.
		if builtins != nil && builtins[fullName] {
			return false, "", ""
		}

		// this.method() — self delegation.
		if objText == "this" {
			return true, propText, "method"
		}

		// this.inner.method() — inner delegation.
		if strings.HasPrefix(objText, "this.") {
			return true, objText + "." + propText, "method"
		}

		// obj.method() — module method.
		return true, fullName, "module_method"
	}

	return false, "", ""
}
