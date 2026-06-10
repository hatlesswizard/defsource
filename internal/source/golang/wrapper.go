package golang

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// detectWrapper analyzes Go function/method source code to determine if it is
// a thin wrapper that delegates to another function. It detects:
// - Single return statement calling exactly one function
// - Error-check-and-return: result, err := foo(); if err != nil { return ..., err }; return result, nil
// - Method delegation: return c.engine.Method()
//
// Returns (isWrapper, targetName, targetKind).
func detectWrapper(code []byte, idx *codebaseIndex) (bool, string, string) {
	if len(code) == 0 {
		return false, "", ""
	}

	parser, err := treesitter.Get(treesitter.Go)
	if err != nil {
		return false, "", ""
	}
	defer treesitter.Put(treesitter.Go, parser)

	// Wrap in a minimal function to make it parseable if it's just a body
	src := code
	if !startsWithFunc(code) {
		src = append([]byte("package p\n"), code...)
	}

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return false, "", ""
	}
	defer tree.Close()

	root := tree.RootNode()

	// Find the function body
	body := findFunctionBody(root)
	if body == nil {
		return false, "", ""
	}

	// Count statements in body (excluding braces)
	stmts := collectStatements(body)

	// Pattern 1: Single return with one function call
	if len(stmts) == 1 {
		if ret := findReturnStatement(stmts[0]); ret != nil {
			if target, kind := extractSingleCallTarget(ret, src); target != "" {
				if idx != nil && idx.IsStdlib(target) {
					return false, "", ""
				}
				return true, target, kind
			}
		}
	}

	// Pattern 2: Error-check-and-return
	// result, err := foo(args); if err != nil { return ..., err }; return result, nil
	if len(stmts) >= 2 && len(stmts) <= 3 {
		if target, kind := detectErrorCheckWrapper(stmts, src); target != "" {
			if idx != nil && idx.IsStdlib(target) {
				return false, "", ""
			}
			return true, target, kind
		}
	}

	return false, "", ""
}

// startsWithFunc checks if code starts with a function declaration.
func startsWithFunc(code []byte) bool {
	s := strings.TrimSpace(string(code))
	return strings.HasPrefix(s, "func ") || strings.HasPrefix(s, "package ")
}

// findFunctionBody locates the block node that is the function body.
func findFunctionBody(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	if node.Type() == "block" {
		// Check if parent is a function/method declaration
		return node
	}

	if node.Type() == "function_declaration" || node.Type() == "method_declaration" || node.Type() == "func_literal" {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "block" {
				return child
			}
		}
	}

	// Recurse
	for i := 0; i < int(node.ChildCount()); i++ {
		if result := findFunctionBody(node.Child(i)); result != nil {
			return result
		}
	}
	return nil
}

// collectStatements returns all statement-level children of a block node.
func collectStatements(block *sitter.Node) []*sitter.Node {
	var stmts []*sitter.Node
	for i := 0; i < int(block.ChildCount()); i++ {
		child := block.Child(i)
		t := child.Type()
		// Skip braces, comments, and whitespace-only text nodes
		if t == "{" || t == "}" || t == "comment" || t == "\n" {
			continue
		}
		// Skip anonymous/unnamed nodes that are just whitespace
		if !child.IsNamed() {
			continue
		}
		stmts = append(stmts, child)
	}
	return stmts
}

// findReturnStatement returns the node if it is a return_statement.
func findReturnStatement(node *sitter.Node) *sitter.Node {
	if node.Type() == "return_statement" {
		return node
	}
	return nil
}

// extractSingleCallTarget checks if a return statement returns exactly one
// function call, and returns the target name and kind.
func extractSingleCallTarget(ret *sitter.Node, content []byte) (string, string) {
	// Find expression_list children of the return statement
	var callNode *sitter.Node
	callCount := 0

	for i := 0; i < int(ret.ChildCount()); i++ {
		child := ret.Child(i)
		if child.Type() == "expression_list" {
			// Check children of expression list
			for j := 0; j < int(child.ChildCount()); j++ {
				expr := child.Child(j)
				if expr.Type() == "call_expression" {
					callNode = expr
					callCount++
				} else if expr.Type() != "," {
					// Non-call expression in return list means not a simple wrapper
					return "", ""
				}
			}
		} else if child.Type() == "call_expression" {
			callNode = child
			callCount++
		}
	}

	if callCount != 1 || callNode == nil {
		return "", ""
	}

	return extractCallTarget(callNode, content)
}

// extractCallTarget extracts the function/method name and kind from a call_expression.
func extractCallTarget(node *sitter.Node, content []byte) (string, string) {
	if node.ChildCount() == 0 {
		return "", ""
	}

	fn := node.Child(0)
	switch fn.Type() {
	case "identifier":
		name := nodeText(fn, content)
		return name, "function"
	case "selector_expression":
		// Could be pkg.Func or receiver.Method
		var receiver, method string
		for i := 0; i < int(fn.ChildCount()); i++ {
			child := fn.Child(i)
			switch child.Type() {
			case "identifier":
				receiver = nodeText(child, content)
			case "field_identifier":
				method = nodeText(child, content)
			case "selector_expression":
				// Nested: a.b.Method() - extract the final method
				receiver = nodeText(child, content)
			}
		}
		if method != "" {
			if receiver != "" {
				return receiver + "." + method, "method"
			}
			return method, "method"
		}
	case "parenthesized_expression":
		// Type assertion call, not a wrapper
		return "", ""
	}

	return "", ""
}

// detectErrorCheckWrapper detects the pattern:
// result, err := foo(args)
// if err != nil { return ..., err }
// return result, nil
func detectErrorCheckWrapper(stmts []*sitter.Node, content []byte) (string, string) {
	if len(stmts) < 2 {
		return "", ""
	}

	// First statement should be short_var_declaration with a call
	first := stmts[0]
	if first.Type() != "short_var_declaration" {
		return "", ""
	}

	// Find the call expression in the right side
	var callNode *sitter.Node
	for i := 0; i < int(first.ChildCount()); i++ {
		child := first.Child(i)
		if child.Type() == "expression_list" {
			for j := 0; j < int(child.ChildCount()); j++ {
				expr := child.Child(j)
				if expr.Type() == "call_expression" {
					callNode = expr
				}
			}
		}
	}
	if callNode == nil {
		return "", ""
	}

	// Second statement should be if_statement checking err
	if len(stmts) >= 2 {
		second := stmts[1]
		if second.Type() != "if_statement" {
			return "", ""
		}
		// Verify it's an error check (contains "err != nil" or "err == nil")
		ifText := nodeText(second, content)
		if !strings.Contains(ifText, "err != nil") && !strings.Contains(ifText, "err == nil") {
			return "", ""
		}
	}

	// Third statement (if present) should be a return
	if len(stmts) == 3 {
		third := stmts[2]
		if third.Type() != "return_statement" {
			return "", ""
		}
	}

	return extractCallTarget(callNode, content)
}
