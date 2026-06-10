package java

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// detectWrapper analyzes a method body and detects delegation patterns.
// Returns (isWrapper, targetName, targetKind) where targetKind is one of:
//   - "method" — delegation to another class's method (e.g., delegate.method())
//   - "self_method" — delegation to this.method() or super.method()
//   - "static_method" — delegation to ClassName.method()
//
// Returns (false, "", "") if not a wrapper or if the delegation is to a
// standard library method.
func detectWrapper(src []byte, idx *codebaseIndex) (bool, string, string) {
	if len(src) == 0 {
		return false, "", ""
	}

	// Wrap in a class method so tree-sitter gets a complete program.
	wrapped := []byte("class __Wrapper { void __detect() " + string(src) + " }")

	parser, err := treesitter.Get(treesitter.Java)
	if err != nil {
		return false, "", ""
	}
	defer treesitter.Put(treesitter.Java, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, wrapped)
	if err != nil || tree == nil {
		return false, "", ""
	}
	defer tree.Close()

	bodyNode := findMethodBody(tree.RootNode())
	if bodyNode == nil {
		return false, "", ""
	}

	// Collect non-comment statements.
	var stmts []*sitter.Node
	count := bodyNode.NamedChildCount()
	for i := range count {
		child := bodyNode.NamedChild(int(i))
		if child == nil {
			continue
		}
		t := child.Type()
		if t == "line_comment" || t == "block_comment" {
			continue
		}
		stmts = append(stmts, child)
	}

	if len(stmts) == 0 || len(stmts) > 5 {
		return false, "", ""
	}

	// Pattern 1: Single return with a method call.
	if len(stmts) == 1 && stmts[0].Type() == "return_statement" {
		callExpr := findCallInReturn(stmts[0])
		if callExpr != nil {
			return classifyJavaCall(callExpr, wrapped, idx)
		}
	}

	// Pattern 2: Single return among assignment + return.
	var returnStmt *sitter.Node
	returnCount := 0
	for _, s := range stmts {
		if s.Type() == "return_statement" {
			returnStmt = s
			returnCount++
		}
	}
	if returnCount == 1 && len(stmts) <= 3 {
		callExpr := findCallInReturn(returnStmt)
		if callExpr != nil {
			return classifyJavaCall(callExpr, wrapped, idx)
		}
	}

	// Pattern 3: Single void expression statement with a method call.
	if len(stmts) == 1 && stmts[0].Type() == "expression_statement" {
		callExpr := findCallInExpression(stmts[0])
		if callExpr != nil {
			return classifyJavaCall(callExpr, wrapped, idx)
		}
	}

	return false, "", ""
}

// findMethodBody locates the block ({...}) body of the first method_declaration
// in the AST.
func findMethodBody(root *sitter.Node) *sitter.Node {
	if root == nil {
		return nil
	}
	if root.Type() == "method_declaration" || root.Type() == "constructor_declaration" {
		return root.ChildByFieldName("body")
	}
	count := root.NamedChildCount()
	for i := range count {
		if body := findMethodBody(root.NamedChild(int(i))); body != nil {
			return body
		}
	}
	return nil
}

// findCallInReturn finds a method_invocation node within a return_statement.
func findCallInReturn(returnNode *sitter.Node) *sitter.Node {
	count := returnNode.NamedChildCount()
	for i := range count {
		child := returnNode.NamedChild(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "method_invocation" {
			return child
		}
		// Check for cast expression wrapping a call.
		if child.Type() == "cast_expression" {
			cc := child.NamedChildCount()
			for j := range cc {
				sub := child.NamedChild(int(j))
				if sub != nil && sub.Type() == "method_invocation" {
					return sub
				}
			}
		}
	}
	return nil
}

// findCallInExpression finds a method_invocation in an expression_statement.
func findCallInExpression(exprStmt *sitter.Node) *sitter.Node {
	count := exprStmt.NamedChildCount()
	for i := range count {
		child := exprStmt.NamedChild(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "method_invocation" {
			return child
		}
	}
	return nil
}

// classifyJavaCall examines a method_invocation node and determines wrapper info.
func classifyJavaCall(callNode *sitter.Node, src []byte, idx *codebaseIndex) (bool, string, string) {
	if callNode.Type() != "method_invocation" {
		return false, "", ""
	}

	// method_invocation has: object (optional), name, arguments
	objNode := callNode.ChildByFieldName("object")
	nameNode := callNode.ChildByFieldName("name")

	if nameNode == nil {
		return false, "", ""
	}
	methodName := strings.TrimSpace(string(src[nameNode.StartByte():nameNode.EndByte()]))
	if methodName == "" {
		return false, "", ""
	}

	// No object: could be a local method call or static import.
	if objNode == nil {
		// Treat as self-method.
		return true, methodName, "self_method"
	}

	objText := strings.TrimSpace(string(src[objNode.StartByte():objNode.EndByte()]))

	// this.method() or super.method()
	if objText == "this" || objText == "super" {
		return true, methodName, "self_method"
	}

	// Check if object is a known class name (static call pattern: ClassName.method()).
	if idx != nil && idx.HasClass(objText) {
		return true, objText + "." + methodName, "static_method"
	}

	// Check if object starts with uppercase (heuristic for static call).
	if len(objText) > 0 && objText[0] >= 'A' && objText[0] <= 'Z' && !strings.Contains(objText, ".") {
		return true, objText + "::" + methodName, "method"
	}

	// delegate.method() pattern — field delegation.
	if isSimpleIdentifier(objText) {
		return true, objText + "::" + methodName, "method"
	}

	return false, "", ""
}

// isSimpleIdentifier checks if a string is a simple Java identifier (no dots, no calls).
func isSimpleIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
				return false
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
				return false
			}
		}
	}
	return true
}
