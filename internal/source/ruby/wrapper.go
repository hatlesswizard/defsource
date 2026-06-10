package ruby

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// detectWrapper analyzes a Ruby method body for wrapper patterns.
// Returns (isWrapper, targetName, targetKind).
// targetKind is one of: "method" (instance method call), "function" (bare call),
// "delegate" (delegation pattern).
func detectWrapper(src []byte, idx *codebaseIndex) (bool, string, string) {
	parser, err := treesitter.Get(treesitter.Ruby)
	if err != nil {
		return false, "", ""
	}
	defer treesitter.Put(treesitter.Ruby, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return false, "", ""
	}
	defer tree.Close()

	root := tree.RootNode()

	// Find the method body node
	bodyNode := findMethodBody(root)
	if bodyNode == nil {
		// Try to look at the root directly for short methods
		bodyNode = root
	}

	// Collect non-comment statements from the body
	stmts := collectStatements(bodyNode, src)
	if len(stmts) == 0 || len(stmts) > 3 {
		return false, "", ""
	}

	// Single statement: check for call expression
	if len(stmts) == 1 {
		return classifyStatement(stmts[0], src, idx)
	}

	return false, "", ""
}

// findMethodBody searches for a method node and returns its body.
func findMethodBody(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}

	switch node.Type() {
	case "method", "singleton_method":
		return node.ChildByFieldName("body")
	}

	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		if body := findMethodBody(node.NamedChild(i)); body != nil {
			return body
		}
	}
	return nil
}

// collectStatements collects non-comment child nodes from a body node.
func collectStatements(body *sitter.Node, src []byte) []*sitter.Node {
	if body == nil {
		return nil
	}

	var stmts []*sitter.Node
	count := int(body.NamedChildCount())
	for i := 0; i < count; i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}
		t := child.Type()
		if t == "comment" {
			continue
		}
		stmts = append(stmts, child)
	}
	return stmts
}

// classifyStatement examines a single statement and determines whether
// it represents a wrapper call.
func classifyStatement(stmt *sitter.Node, src []byte, idx *codebaseIndex) (bool, string, string) {
	switch stmt.Type() {
	case "call":
		return classifyCallNode(stmt, src, idx)

	case "method_call":
		return classifyCallNode(stmt, src, idx)

	case "return":
		// return expr — check if expr is a single call
		count := int(stmt.NamedChildCount())
		for i := 0; i < count; i++ {
			child := stmt.NamedChild(i)
			if child == nil {
				continue
			}
			if child.Type() == "call" || child.Type() == "method_call" {
				return classifyCallNode(child, src, idx)
			}
		}
	}

	return false, "", ""
}

// classifyCallNode examines a call or method_call node.
func classifyCallNode(node *sitter.Node, src []byte, idx *codebaseIndex) (bool, string, string) {
	methodNode := node.ChildByFieldName("method")
	if methodNode == nil {
		return false, "", ""
	}
	methodName := nodeText(methodNode, src)
	if methodName == "" {
		return false, "", ""
	}

	receiverNode := node.ChildByFieldName("receiver")
	if receiverNode != nil {
		receiver := nodeText(receiverNode, src)
		if receiver == "" {
			return false, "", ""
		}

		// self.method_name — delegate to same class
		if receiver == "self" {
			return true, methodName, "method"
		}

		// @ivar.method_name or receiver.method_name — delegation
		if strings.HasPrefix(receiver, "@") || isSimpleIdentifier(receiver) {
			return true, receiver + "." + methodName, "delegate"
		}

		// ClassName.method_name — static call
		if len(receiver) > 0 && receiver[0] >= 'A' && receiver[0] <= 'Z' {
			return true, receiver + "#" + methodName, "method"
		}
	}

	// Bare method call — could be a forwarding wrapper
	return true, methodName, "function"
}

// isSimpleIdentifier returns true if s looks like a simple Ruby identifier
// (lowercase letter or underscore followed by word characters).
func isSimpleIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || c == '_') {
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
