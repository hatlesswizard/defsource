package rust

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// detectWrapper analyzes a method's source code for wrapper patterns.
// Returns (isWrapper, targetName, targetKind).
//
// Detected patterns:
//  1. Single-expression forwarding: fn foo(x: T) -> U { bar(x) }
//  2. Self-method delegation: fn foo(&self) -> U { self.bar() }
//  3. Deref/DerefMut delegation: fn deref(&self) -> &T { &self.inner }
//  4. Newtype unwrap: fn method(&self) -> U { self.0.method() }
func detectWrapper(sourceCode string) (bool, string, string) {
	src := []byte(sourceCode)

	parser, err := treesitter.Get(treesitter.Rust)
	if err != nil {
		return false, "", ""
	}
	defer treesitter.Put(treesitter.Rust, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return false, "", ""
	}
	defer tree.Close()

	root := tree.RootNode()

	// Find the function_item in the parsed snippet
	fnNode := findFunctionItem(root)
	if fnNode == nil {
		return false, "", ""
	}

	// Get the function body
	body := fnNode.ChildByFieldName("body")
	if body == nil {
		return false, "", ""
	}

	// A wrapper has a block with exactly one expression statement or a
	// single return expression.
	expr := getSingleExpression(body, src)
	if expr == nil {
		return false, "", ""
	}

	// Analyze the single expression
	return analyzeWrapperExpr(expr, src)
}

// findFunctionItem finds the first function_item node in the tree.
func findFunctionItem(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	if node.Type() == "function_item" {
		return node
	}
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		if found := findFunctionItem(node.NamedChild(i)); found != nil {
			return found
		}
	}
	return nil
}

// getSingleExpression extracts the single expression from a block body.
// Returns nil if the block has more than one statement.
func getSingleExpression(body *sitter.Node, src []byte) *sitter.Node {
	if body == nil {
		return nil
	}

	// Count significant children (skip braces)
	var exprs []*sitter.Node
	count := int(body.NamedChildCount())
	for i := 0; i < count; i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}
		exprs = append(exprs, child)
	}

	if len(exprs) != 1 {
		return nil
	}

	expr := exprs[0]

	// Handle expression_statement wrapping
	if expr.Type() == "expression_statement" {
		if inner := expr.NamedChild(0); inner != nil {
			return inner
		}
	}

	// Handle return_expression
	if expr.Type() == "return_expression" {
		if inner := expr.NamedChild(0); inner != nil {
			return inner
		}
	}

	return expr
}

// analyzeWrapperExpr determines if a single expression is a wrapper delegation.
func analyzeWrapperExpr(expr *sitter.Node, src []byte) (bool, string, string) {
	if expr == nil {
		return false, "", ""
	}

	switch expr.Type() {
	case "call_expression":
		// fn foo(x: T) -> U { bar(x) }
		fnNode := expr.ChildByFieldName("function")
		if fnNode == nil {
			return false, "", ""
		}
		target := strings.TrimSpace(string(src[fnNode.StartByte():fnNode.EndByte()]))
		if target == "" {
			return false, "", ""
		}

		// Check if it's a method call on self
		if strings.HasPrefix(target, "self.") {
			methodName := strings.TrimPrefix(target, "self.")
			return true, methodName, "self_method"
		}

		// Check for Type::method pattern
		if strings.Contains(target, "::") {
			return true, target, "function"
		}

		// Simple function call
		return true, target, "function"

	case "method_call_expression":
		// self.bar() or self.0.method()
		receiverNode := expr.ChildByFieldName("object")
		methodNode := expr.ChildByFieldName("name")
		if receiverNode == nil || methodNode == nil {
			return false, "", ""
		}

		receiver := strings.TrimSpace(string(src[receiverNode.StartByte():receiverNode.EndByte()]))
		method := strings.TrimSpace(string(src[methodNode.StartByte():methodNode.EndByte()]))

		if receiver == "self" || receiver == "(*self)" {
			return true, method, "self_method"
		}

		// Newtype pattern: self.0.method() or self.inner.method()
		if strings.HasPrefix(receiver, "self.") {
			inner := strings.TrimPrefix(receiver, "self.")
			_ = inner
			return true, method, "self_method"
		}

		return false, "", ""

	case "field_expression":
		// Deref pattern: &self.inner or self.0
		receiverNode := expr.ChildByFieldName("value")
		fieldNode := expr.ChildByFieldName("field")
		if receiverNode == nil || fieldNode == nil {
			return false, "", ""
		}

		receiver := strings.TrimSpace(string(src[receiverNode.StartByte():receiverNode.EndByte()]))
		if receiver == "self" || receiver == "(*self)" {
			field := strings.TrimSpace(string(src[fieldNode.StartByte():fieldNode.EndByte()]))
			return true, field, "self_method"
		}
		return false, "", ""

	case "reference_expression":
		// &self.inner (Deref impl pattern)
		if inner := expr.NamedChild(0); inner != nil {
			return analyzeWrapperExpr(inner, src)
		}
		return false, "", ""

	case "try_expression":
		// expr? -- unwrap the inner expression
		if inner := expr.NamedChild(0); inner != nil {
			return analyzeWrapperExpr(inner, src)
		}
		return false, "", ""

	case "await_expression":
		// expr.await -- unwrap the inner expression
		if inner := expr.NamedChild(0); inner != nil {
			return analyzeWrapperExpr(inner, src)
		}
		return false, "", ""
	}

	return false, "", ""
}
