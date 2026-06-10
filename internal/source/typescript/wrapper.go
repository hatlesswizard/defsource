package typescript

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// detectWrapper analyzes a method's source code to determine if it is a thin
// wrapper around another function or method. Returns (isWrapper, targetName, targetKind).
//
// TypeScript wrapper patterns detected:
// 1. Arrow functions with single call: const foo = (x) => bar(x)
// 2. Functions with single return: function wrapper(x) { return target(x); }
// 3. Decorator-based delegation: methods that delegate to injected services
// 4. Re-exports: export { X } from './module'
func detectWrapper(sourceCode string, idx *codebaseIndex) (bool, string, string) {
	src := []byte(sourceCode)

	parser, err := treesitter.Get(treesitter.TypeScript)
	if err != nil {
		return false, "", ""
	}
	defer treesitter.Put(treesitter.TypeScript, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return false, "", ""
	}
	defer tree.Close()

	root := tree.RootNode()

	// Try to find a wrapper pattern in the AST
	isWrapper, target, kind := analyzeWrapperAST(root, src, idx)
	return isWrapper, target, kind
}

// analyzeWrapperAST walks the parsed source looking for wrapper patterns.
func analyzeWrapperAST(root *sitter.Node, src []byte, idx *codebaseIndex) (bool, string, string) {
	// Find the function/method body
	body := findFunctionBody(root, src)
	if body == nil {
		// Could be an arrow function without braces
		return analyzeArrowExpression(root, src, idx)
	}

	// Count statements in the body (excluding empty statements and comments)
	stmts := countStatements(body)

	// A wrapper has exactly one statement: a return statement with a single call
	if stmts != 1 {
		return false, "", ""
	}

	// Find the single return statement
	returnNode := findSingleReturn(body)
	if returnNode == nil {
		// Could be an expression statement (no return keyword in arrow body)
		exprNode := findSingleExpression(body)
		if exprNode != nil {
			return analyzeCallExpression(exprNode, src, idx)
		}
		return false, "", ""
	}

	// The return statement should contain a single call expression
	callNode := findCallInReturn(returnNode)
	if callNode == nil {
		return false, "", ""
	}

	return analyzeCallExpression(callNode, src, idx)
}

// analyzeArrowExpression checks if the root contains a concise arrow function
// body that is a single call.
func analyzeArrowExpression(root *sitter.Node, src []byte, idx *codebaseIndex) (bool, string, string) {
	// Walk to find an arrow_function with expression body
	arrowNode := findNodeOfType(root, "arrow_function")
	if arrowNode == nil {
		return false, "", ""
	}

	// The last child of an arrow function without braces is the expression body
	lastChild := arrowNode.Child(int(arrowNode.ChildCount()) - 1)
	if lastChild == nil {
		return false, "", ""
	}

	if lastChild.Type() == "call_expression" {
		return analyzeCallExpression(lastChild, src, idx)
	}

	return false, "", ""
}

// analyzeCallExpression extracts the call target and determines wrapper kind.
func analyzeCallExpression(node *sitter.Node, src []byte, idx *codebaseIndex) (bool, string, string) {
	if node.Type() != "call_expression" {
		return false, "", ""
	}

	// Get the function being called
	fnNode := node.Child(0)
	if fnNode == nil {
		return false, "", ""
	}

	callTarget := strings.TrimSpace(nodeText(fnNode, src))
	if callTarget == "" {
		return false, "", ""
	}

	// Determine the kind based on the call pattern
	switch fnNode.Type() {
	case "identifier":
		// Simple function call: bar(x)
		if idx != nil && idx.HasEntity(callTarget) {
			return true, callTarget, "function"
		}
		// Even if not in index, it's still a wrapper pattern
		return true, callTarget, "function"

	case "member_expression":
		// Method call: this.service.method(x) or obj.method(x)
		return true, callTarget, "method"

	default:
		return false, "", ""
	}
}

// findFunctionBody finds the statement_block body of a function/method.
func findFunctionBody(node *sitter.Node, src []byte) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "statement_block" {
		return node
	}

	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "statement_block" {
			return child
		}
		// Recurse into function/method/arrow bodies
		if child.Type() == "function_declaration" || child.Type() == "method_definition" ||
			child.Type() == "arrow_function" || child.Type() == "function" {
			subCount := int(child.ChildCount())
			for j := 0; j < subCount; j++ {
				sub := child.Child(j)
				if sub != nil && sub.Type() == "statement_block" {
					return sub
				}
			}
		}
		if result := findFunctionBody(child, src); result != nil {
			return result
		}
	}

	return nil
}

// countStatements counts meaningful statements in a statement block.
func countStatements(body *sitter.Node) int {
	count := 0
	childCount := int(body.ChildCount())
	for i := 0; i < childCount; i++ {
		child := body.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "{", "}", "comment", "empty_statement":
			continue
		default:
			count++
		}
	}
	return count
}

// findSingleReturn finds a return_statement node in a statement block.
func findSingleReturn(body *sitter.Node) *sitter.Node {
	childCount := int(body.ChildCount())
	for i := 0; i < childCount; i++ {
		child := body.Child(i)
		if child != nil && child.Type() == "return_statement" {
			return child
		}
	}
	return nil
}

// findSingleExpression finds a single expression_statement node.
func findSingleExpression(body *sitter.Node) *sitter.Node {
	childCount := int(body.ChildCount())
	for i := 0; i < childCount; i++ {
		child := body.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "expression_statement" {
			// Get the expression inside
			exprCount := int(child.ChildCount())
			for j := 0; j < exprCount; j++ {
				expr := child.Child(j)
				if expr != nil && expr.Type() == "call_expression" {
					return expr
				}
			}
		}
	}
	return nil
}

// findCallInReturn finds a call_expression within a return statement.
func findCallInReturn(node *sitter.Node) *sitter.Node {
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "call_expression" {
			return child
		}
		// Check for await call_expression
		if child.Type() == "await_expression" {
			awaitCount := int(child.ChildCount())
			for j := 0; j < awaitCount; j++ {
				awaitChild := child.Child(j)
				if awaitChild != nil && awaitChild.Type() == "call_expression" {
					return awaitChild
				}
			}
		}
	}
	return nil
}

// findNodeOfType finds the first descendant node of a given type.
func findNodeOfType(node *sitter.Node, nodeType string) *sitter.Node {
	if node == nil {
		return nil
	}
	if node.Type() == nodeType {
		return node
	}
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		if result := findNodeOfType(node.Child(i), nodeType); result != nil {
			return result
		}
	}
	return nil
}
