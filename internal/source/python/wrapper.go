package python

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// pythonBuiltins is the set of Python built-in functions that should not
// be resolved as wrapper targets (analogous to phpBuiltins in the php package).
var pythonBuiltins = map[string]bool{
	"abs": true, "all": true, "any": true, "ascii": true,
	"bin": true, "bool": true, "breakpoint": true, "bytearray": true,
	"bytes": true, "callable": true, "chr": true, "classmethod": true,
	"compile": true, "complex": true, "delattr": true, "dict": true,
	"dir": true, "divmod": true, "enumerate": true, "eval": true,
	"exec": true, "filter": true, "float": true, "format": true,
	"frozenset": true, "getattr": true, "globals": true, "hasattr": true,
	"hash": true, "help": true, "hex": true, "id": true,
	"input": true, "int": true, "isinstance": true, "issubclass": true,
	"iter": true, "len": true, "list": true, "locals": true,
	"map": true, "max": true, "memoryview": true, "min": true,
	"next": true, "object": true, "oct": true, "open": true,
	"ord": true, "pow": true, "print": true, "property": true,
	"range": true, "repr": true, "reversed": true, "round": true,
	"set": true, "setattr": true, "slice": true, "sorted": true,
	"staticmethod": true, "str": true, "sum": true, "super": true,
	"tuple": true, "type": true, "vars": true, "zip": true,
	"__import__": true,
}

// detectWrapper analyzes a method/function source code for wrapper patterns.
// A function is a wrapper if:
// 1. Its body is a single return statement calling another function.
// 2. It delegates via self.delegate.method() or similar patterns.
// 3. It has @functools.wraps decorator (detected by caller from decorators).
//
// Returns (isWrapper, targetName, targetKind) where targetKind is
// "function", "method", or "delegate_method".
func detectWrapper(src []byte, idx *codebaseIndex) (bool, string, string) {
	// Wrap in a function so tree-sitter can parse the body.
	wrapped := append([]byte("def __wrapper__():\n"), indentSource(src)...)

	parser, err := treesitter.Get(treesitter.Python)
	if err != nil {
		return false, "", ""
	}
	defer treesitter.Put(treesitter.Python, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, wrapped)
	if err != nil || tree == nil {
		return false, "", ""
	}
	defer tree.Close()

	root := tree.RootNode()
	bodyNode := findFunctionBody(root)
	if bodyNode == nil {
		return false, "", ""
	}

	// Count non-trivial statements in the body (skip docstrings and pass)
	stmts := countStatements(bodyNode, wrapped)
	if stmts != 1 {
		return false, "", ""
	}

	// Find the single return statement
	returnNode := findReturnStatement(bodyNode)
	if returnNode == nil {
		return false, "", ""
	}

	// The return value should be a call expression
	callNode := findCallInReturn(returnNode, wrapped)
	if callNode == nil {
		return false, "", ""
	}

	fnNode := callNode.ChildByFieldName("function")
	if fnNode == nil {
		return false, "", ""
	}

	callText := strings.TrimSpace(string(wrapped[fnNode.StartByte():fnNode.EndByte()]))
	if callText == "" {
		return false, "", ""
	}

	// Skip Python builtins
	baseName := callText
	if idx := strings.LastIndex(callText, "."); idx >= 0 {
		baseName = callText[idx+1:]
	}
	if pythonBuiltins[baseName] || pythonBuiltins[callText] {
		return false, "", ""
	}

	// Classify the call
	if strings.HasPrefix(callText, "self.") {
		// self.method() or self.attr.method()
		rest := callText[5:]
		if strings.Contains(rest, ".") {
			// Delegation: self.delegate.method()
			return true, rest, "delegate_method"
		}
		// Direct self method call
		return true, rest, "method"
	}

	if strings.Contains(callText, ".") {
		// Attribute access like module.function() or obj.method()
		return true, callText, "method"
	}

	// Plain function call
	if idx != nil && (idx.HasFunction(callText) || idx.HasClass(callText)) {
		return true, callText, "function"
	}

	// If we can't resolve it but it looks like a simple delegation, still report
	return true, callText, "function"
}

// indentSource adds 4-space indentation to each line of src for wrapping
// inside a function definition.
func indentSource(src []byte) []byte {
	lines := strings.Split(string(src), "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if strings.TrimSpace(line) != "" {
			b.WriteString("    ")
		}
		b.WriteString(line)
	}
	return []byte(b.String())
}

// findFunctionBody searches for the first function_definition and returns
// its body child node.
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

// countStatements counts non-trivial statements in a block node.
// Skips expression_statement nodes that are just docstrings or pass.
func countStatements(body *sitter.Node, src []byte) int {
	count := 0
	namedCount := body.NamedChildCount()
	for i := range namedCount {
		child := body.NamedChild(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "expression_statement":
			// Skip docstrings (string literals as expression statements)
			if child.NamedChildCount() > 0 {
				inner := child.NamedChild(0)
				if inner != nil && (inner.Type() == "string" || inner.Type() == "concatenated_string") {
					continue
				}
			}
			count++
		case "pass_statement":
			continue
		default:
			count++
		}
	}
	return count
}

// findReturnStatement finds the first return_statement in a block node.
func findReturnStatement(body *sitter.Node) *sitter.Node {
	namedCount := body.NamedChildCount()
	for i := range namedCount {
		child := body.NamedChild(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "return_statement" {
			return child
		}
	}
	return nil
}

// findCallInReturn finds the call node in a return statement's value.
func findCallInReturn(returnNode *sitter.Node, src []byte) *sitter.Node {
	// The return statement's value is in its named children
	count := returnNode.NamedChildCount()
	for i := range count {
		child := returnNode.NamedChild(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "call" {
			return child
		}
		// Handle "await some_call()" — the await expression wraps the call
		if child.Type() == "await" {
			awaitCount := child.NamedChildCount()
			for j := range awaitCount {
				inner := child.NamedChild(int(j))
				if inner != nil && inner.Type() == "call" {
					return inner
				}
			}
		}
	}
	return nil
}
