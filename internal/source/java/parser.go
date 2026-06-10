package java

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/docparser"
	"github.com/hatlesswizard/defsource/internal/docparser/javadoc"
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// javadocParser is the shared JavaDoc parser instance.
var javadocParser = javadoc.New()

// parseFile parses a Java source file using tree-sitter and returns the
// collected analysis: types (classes, interfaces, enums, records, annotations)
// with their methods, fields, and inner types.
func parseFile(src []byte) *fileAnalysis {
	parser, err := treesitter.Get(treesitter.Java)
	if err != nil {
		return nil
	}
	defer treesitter.Put(treesitter.Java, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return &fileAnalysis{}
	}
	defer tree.Close()

	result := &fileAnalysis{}
	root := tree.RootNode()
	walkTopLevel(root, src, result)
	return result
}

// walkTopLevel walks the top-level AST nodes looking for type declarations.
func walkTopLevel(node *sitter.Node, src []byte, result *fileAnalysis) {
	if node == nil {
		return
	}

	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "class_declaration":
			t := extractType(child, src, source.KindClass)
			result.Types = append(result.Types, t)
		case "interface_declaration":
			t := extractType(child, src, source.KindInterface)
			result.Types = append(result.Types, t)
		case "enum_declaration":
			t := extractType(child, src, source.KindEnum)
			result.Types = append(result.Types, t)
		case "record_declaration":
			t := extractType(child, src, source.KindRecord)
			result.Types = append(result.Types, t)
		case "annotation_type_declaration":
			t := extractType(child, src, source.KindAnnotation)
			result.Types = append(result.Types, t)
		case "program":
			// Recurse into program node (root).
			walkTopLevel(child, src, result)
		}
	}
}

// extractType extracts a full type definition from a declaration node.
func extractType(node *sitter.Node, src []byte, kind string) javaType {
	t := javaType{
		Kind:     kind,
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	// Extract name.
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		t.Name = nodeText(nameNode, src)
	}

	// Extract modifiers (visibility, static, abstract, etc.).
	t.Visibility, t.Sealed, t.Annotations = extractModifiers(node, src)
	if t.Visibility == "" {
		t.Visibility = "package"
	}

	// Extract type parameters.
	if tpNode := node.ChildByFieldName("type_parameters"); tpNode != nil {
		t.TypeParams = nodeText(tpNode, src)
	}

	// Extract superclass.
	if scNode := node.ChildByFieldName("superclass"); scNode != nil {
		t.Extends = nodeText(scNode, src)
		// Remove the "extends " prefix if present.
		t.Extends = strings.TrimPrefix(t.Extends, "extends ")
		t.Extends = strings.TrimSpace(t.Extends)
	}

	// Extract interfaces.
	if implNode := node.ChildByFieldName("interfaces"); implNode != nil {
		t.Implements = extractInterfaceList(implNode, src)
	}

	// Extract permits (sealed classes).
	if permitsNode := node.ChildByFieldName("permits"); permitsNode != nil {
		t.Permits = extractTypeList(permitsNode, src)
	}

	// Extract JavaDoc.
	if docStr, ok := findPrecedingDoc(src, t.StartPos); ok {
		t.Doc = javadocParser.Parse(docStr)
	}

	// Extract body members.
	if bodyNode := node.ChildByFieldName("body"); bodyNode != nil {
		extractBodyMembers(bodyNode, src, &t)
	}

	// For records, extract components as fields.
	if kind == source.KindRecord {
		extractRecordComponents(node, src, &t)
	}

	return t
}

// extractBodyMembers walks the body of a type and extracts methods, fields, and inner types.
// Handles both regular class_body and enum_body (where methods are in enum_body_declarations).
func extractBodyMembers(body *sitter.Node, src []byte, t *javaType) {
	count := body.NamedChildCount()
	for i := range count {
		child := body.NamedChild(int(i))
		if child == nil {
			continue
		}
		extractMemberNode(child, src, t)
	}
}

// extractMemberNode dispatches a single member node to the appropriate handler.
func extractMemberNode(child *sitter.Node, src []byte, t *javaType) {
	switch child.Type() {
	case "method_declaration":
		m := extractMethod(child, src)
		t.Methods = append(t.Methods, m)
	case "constructor_declaration":
		m := extractConstructor(child, src, t.Name)
		t.Methods = append(t.Methods, m)
	case "field_declaration":
		fields := extractFields(child, src)
		t.Fields = append(t.Fields, fields...)
	case "class_declaration":
		inner := extractType(child, src, source.KindClass)
		t.InnerTypes = append(t.InnerTypes, inner)
	case "interface_declaration":
		inner := extractType(child, src, source.KindInterface)
		t.InnerTypes = append(t.InnerTypes, inner)
	case "enum_declaration":
		inner := extractType(child, src, source.KindEnum)
		t.InnerTypes = append(t.InnerTypes, inner)
	case "record_declaration":
		inner := extractType(child, src, source.KindRecord)
		t.InnerTypes = append(t.InnerTypes, inner)
	case "annotation_type_declaration":
		inner := extractType(child, src, source.KindAnnotation)
		t.InnerTypes = append(t.InnerTypes, inner)
	case "annotation_type_element_declaration":
		// Annotation elements are similar to methods.
		m := extractAnnotationElement(child, src)
		t.Methods = append(t.Methods, m)
	case "constant_declaration":
		// Interface constants.
		fields := extractFields(child, src)
		t.Fields = append(t.Fields, fields...)
	case "enum_body_declarations":
		// Enum methods and fields are inside enum_body_declarations.
		cc := child.NamedChildCount()
		for j := range cc {
			sub := child.NamedChild(int(j))
			if sub != nil {
				extractMemberNode(sub, src, t)
			}
		}
	}
}

// extractMethod extracts a javaMethod from a method_declaration node.
func extractMethod(node *sitter.Node, src []byte) javaMethod {
	m := javaMethod{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	// Name.
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		m.Name = nodeText(nameNode, src)
	}

	// Modifiers.
	m.Visibility, m.Static, m.Abstract, m.Final, m.Default, m.Deprecated, m.Annotations = extractMethodModifiers(node, src)
	if m.Visibility == "" {
		m.Visibility = "package"
	}

	// Type parameters.
	if tpNode := node.ChildByFieldName("type_parameters"); tpNode != nil {
		m.TypeParams = nodeText(tpNode, src)
	}

	// Return type.
	if rtNode := node.ChildByFieldName("type"); rtNode != nil {
		m.ReturnType = nodeText(rtNode, src)
	}

	// Parameters.
	if paramsNode := node.ChildByFieldName("parameters"); paramsNode != nil {
		m.Params = extractParams(paramsNode, src)
	}

	// Throws.
	m.Throws = extractThrows(node, src)

	// JavaDoc.
	if docStr, ok := findPrecedingDoc(src, m.StartPos); ok {
		m.Doc = javadocParser.Parse(docStr)
		// Enrich parameter descriptions from JavaDoc.
		enrichParamsFromDoc(m.Params, m.Doc)
	}

	return m
}

// extractConstructor extracts a javaMethod from a constructor_declaration node.
func extractConstructor(node *sitter.Node, src []byte, className string) javaMethod {
	m := javaMethod{
		Name:     className,
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	m.Visibility, m.Static, m.Abstract, m.Final, m.Default, m.Deprecated, m.Annotations = extractMethodModifiers(node, src)
	if m.Visibility == "" {
		m.Visibility = "package"
	}

	// Type parameters.
	if tpNode := node.ChildByFieldName("type_parameters"); tpNode != nil {
		m.TypeParams = nodeText(tpNode, src)
	}

	// Parameters.
	if paramsNode := node.ChildByFieldName("parameters"); paramsNode != nil {
		m.Params = extractParams(paramsNode, src)
	}

	// Throws.
	m.Throws = extractThrows(node, src)

	// JavaDoc.
	if docStr, ok := findPrecedingDoc(src, m.StartPos); ok {
		m.Doc = javadocParser.Parse(docStr)
		enrichParamsFromDoc(m.Params, m.Doc)
	}

	return m
}

// extractAnnotationElement extracts a method-like element from annotation type.
func extractAnnotationElement(node *sitter.Node, src []byte) javaMethod {
	m := javaMethod{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		m.Name = nodeText(nameNode, src)
	}
	if rtNode := node.ChildByFieldName("type"); rtNode != nil {
		m.ReturnType = nodeText(rtNode, src)
	}

	m.Visibility = "public"

	if docStr, ok := findPrecedingDoc(src, m.StartPos); ok {
		m.Doc = javadocParser.Parse(docStr)
	}

	return m
}

// extractFields extracts javaField entries from a field_declaration node.
func extractFields(node *sitter.Node, src []byte) []javaField {
	visibility, isStatic, _, isFinal, _, deprecated, annotations := extractMethodModifiers(node, src)
	if visibility == "" {
		visibility = "package"
	}

	// Get the type.
	var typeName string
	if typeNode := node.ChildByFieldName("type"); typeNode != nil {
		typeName = nodeText(typeNode, src)
	}

	// Get the JavaDoc.
	var doc *docparser.DocComment
	if docStr, ok := findPrecedingDoc(src, int(node.StartByte())); ok {
		doc = javadocParser.Parse(docStr)
	}

	// Extract declarators (variable names).
	var fields []javaField
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "variable_declarator" {
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				f := javaField{
					Name:        nodeText(nameNode, src),
					Type:        typeName,
					Visibility:  visibility,
					Static:      isStatic,
					Final:       isFinal,
					Annotations: annotations,
					Doc:         doc,
					StartPos:    int(node.StartByte()),
					EndPos:      int(node.EndByte()),
				}
				if deprecated {
					// Mark deprecated fields via doc.
					if f.Doc == nil {
						f.Doc = &docparser.DocComment{Deprecated: "true"}
					}
				}
				fields = append(fields, f)
			}
		}
	}
	return fields
}

// extractRecordComponents extracts record components as fields.
func extractRecordComponents(node *sitter.Node, src []byte, t *javaType) {
	// Record components are in the formal_parameters field.
	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode == nil {
		return
	}
	count := paramsNode.NamedChildCount()
	for i := range count {
		child := paramsNode.NamedChild(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "formal_parameter" || child.Type() == "record_pattern_component" {
			var pName, pType string
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				pName = nodeText(nameNode, src)
			}
			if typeNode := child.ChildByFieldName("type"); typeNode != nil {
				pType = nodeText(typeNode, src)
			}
			if pName != "" {
				t.Fields = append(t.Fields, javaField{
					Name:       pName,
					Type:       pType,
					Visibility: "public",
					Final:      true,
				})
			}
		}
	}
}

// extractParams extracts method parameters from a formal_parameters node.
func extractParams(node *sitter.Node, src []byte) []javaParam {
	var params []javaParam
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "formal_parameter":
			p := extractSingleParam(child, src, false)
			params = append(params, p)
		case "spread_parameter":
			p := extractSingleParam(child, src, true)
			params = append(params, p)
		case "receiver_parameter":
			// Skip the receiver parameter (this keyword in annotations).
		}
	}
	return params
}

// extractSingleParam extracts a single parameter from a formal_parameter
// or spread_parameter node.
//
// In the Java tree-sitter grammar:
// - formal_parameter: has "type" and "name" fields accessible via ChildByFieldName
// - spread_parameter: type is the first named child; name is inside a
//   variable_declarator child; "..." is an unnamed node between them
func extractSingleParam(node *sitter.Node, src []byte, variadic bool) javaParam {
	p := javaParam{Variadic: variadic}

	if variadic {
		// spread_parameter: type is first named child, name is in variable_declarator
		var typeName string
		count := node.NamedChildCount()
		for i := range count {
			child := node.NamedChild(int(i))
			if child == nil {
				continue
			}
			switch child.Type() {
			case "type_identifier", "generic_type", "array_type", "scoped_type_identifier":
				typeName = nodeText(child, src)
			case "variable_declarator":
				// Name is the first child (identifier) of the variable_declarator.
				if nameNode := child.ChildByFieldName("name"); nameNode != nil {
					p.Name = nodeText(nameNode, src)
				} else if child.NamedChildCount() > 0 {
					p.Name = nodeText(child.NamedChild(0), src)
				}
			case "identifier":
				// Sometimes name is a direct child.
				if p.Name == "" {
					p.Name = nodeText(child, src)
				}
			}
		}
		if typeName != "" {
			p.Type = typeName + "..."
		}
	} else {
		// formal_parameter: use field names.
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			p.Name = nodeText(nameNode, src)
		}
		if typeNode := node.ChildByFieldName("type"); typeNode != nil {
			p.Type = nodeText(typeNode, src)
		}
		// Check for array dimensions on the parameter name (int arr[]).
		if dims := node.ChildByFieldName("dimensions"); dims != nil {
			p.Type += nodeText(dims, src)
		}
	}

	return p
}

// extractThrows extracts throw declarations from a method/constructor.
func extractThrows(node *sitter.Node, src []byte) []string {
	var throws []string
	count := node.NamedChildCount()
	for i := range count {
		child := node.NamedChild(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "throws" {
			// Walk children to find type identifiers.
			tc := child.NamedChildCount()
			for j := range tc {
				tChild := child.NamedChild(int(j))
				if tChild != nil {
					text := nodeText(tChild, src)
					if text != "" {
						throws = append(throws, text)
					}
				}
			}
		}
	}
	return throws
}

// extractModifiers extracts visibility, sealed flag, and annotations from
// a type declaration's modifiers child.
func extractModifiers(node *sitter.Node, src []byte) (visibility string, sealed bool, annotations []string) {
	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "modifiers":
			visibility, sealed, annotations = parseModifiersNode(child, src)
			return
		}
	}
	return "", false, nil
}

// parseModifiersNode walks a modifiers node to extract individual modifiers.
// Note: In the Java tree-sitter grammar, keyword modifiers (public, static, etc.)
// are unnamed child nodes whose node type IS the keyword itself.
func parseModifiersNode(node *sitter.Node, src []byte) (visibility string, sealed bool, annotations []string) {
	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child == nil {
			continue
		}
		switch child.Type() {
		case "marker_annotation", "annotation":
			ann := nodeText(child, src)
			annotations = append(annotations, ann)
		case "public", "protected", "private":
			visibility = child.Type()
		case "sealed":
			sealed = true
		}
	}
	return
}

// extractMethodModifiers extracts all modifiers relevant to methods/fields.
// Note: In the Java tree-sitter grammar, keyword modifiers (public, static, etc.)
// are unnamed child nodes whose node type IS the keyword itself.
func extractMethodModifiers(node *sitter.Node, src []byte) (
	visibility string, static, abstract, final, isDefault, deprecated bool, annotations []string) {

	count := node.ChildCount()
	for i := range count {
		child := node.Child(int(i))
		if child == nil {
			continue
		}
		if child.Type() == "modifiers" {
			mc := child.ChildCount()
			for j := range mc {
				mod := child.Child(int(j))
				if mod == nil {
					continue
				}
				switch mod.Type() {
				case "marker_annotation", "annotation":
					ann := nodeText(mod, src)
					annotations = append(annotations, ann)
					if strings.Contains(ann, "Deprecated") {
						deprecated = true
					}
				case "public", "protected", "private":
					visibility = mod.Type()
				case "static":
					static = true
				case "abstract":
					abstract = true
				case "final":
					final = true
				case "default":
					isDefault = true
				}
			}
			return
		}
	}
	return
}

// extractInterfaceList extracts interface names from an interfaces/super_interfaces node.
func extractInterfaceList(node *sitter.Node, src []byte) []string {
	var interfaces []string
	// The node text is typically "implements Foo, Bar" or "extends Foo, Bar".
	text := nodeText(node, src)
	text = strings.TrimPrefix(text, "implements ")
	text = strings.TrimPrefix(text, "extends ")
	for _, part := range strings.Split(text, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			interfaces = append(interfaces, part)
		}
	}
	return interfaces
}

// extractTypeList extracts type names from a permits clause.
func extractTypeList(node *sitter.Node, src []byte) []string {
	var types []string
	text := nodeText(node, src)
	text = strings.TrimPrefix(text, "permits ")
	for _, part := range strings.Split(text, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			types = append(types, part)
		}
	}
	return types
}

// enrichParamsFromDoc merges JavaDoc @param descriptions into extracted parameters.
func enrichParamsFromDoc(params []javaParam, doc *docparser.DocComment) {
	if doc == nil {
		return
	}
	for i := range params {
		for _, dp := range doc.Params {
			if dp.Name == params[i].Name {
				params[i].Description = dp.Description
				break
			}
		}
	}
}

// findPrecedingDoc locates the JavaDoc comment (/** ... */) immediately before
// a declaration at byte position pos.
func findPrecedingDoc(src []byte, pos int) (string, bool) {
	i := pos - 1
	if i < 0 {
		return "", false
	}

	// Skip whitespace and annotations.
	for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i--
	}

	// Skip any annotations between doc and declaration (e.g., @Override).
	for i >= 0 && src[i] == ')' {
		// Skip annotation with arguments.
		depth := 1
		i--
		for i >= 0 && depth > 0 {
			if src[i] == ')' {
				depth++
			} else if src[i] == '(' {
				depth--
			}
			if depth > 0 {
				i--
			}
		}
		// Skip the annotation name and @.
		if i >= 0 && src[i] == '(' {
			i--
		}
		for i >= 0 && src[i] != '@' && src[i] != '\n' {
			i--
		}
		if i >= 0 && src[i] == '@' {
			i--
		}
		for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
			i--
		}
	}

	// Skip marker annotations (no parens).
	for i >= 0 {
		// Check if we are at the end of an annotation identifier.
		if i >= 0 && isJavaIdentChar(src[i]) {
			// Walk back through the identifier.
			end := i
			for i >= 0 && isJavaIdentChar(src[i]) {
				i--
			}
			// Check for @.
			if i >= 0 && src[i] == '@' {
				i--
				for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
					i--
				}
				continue
			}
			// Not an annotation, restore position.
			i = end
			break
		}
		break
	}

	// Now check for end of a doc comment: */
	if i < 1 || src[i] != '/' || src[i-1] != '*' {
		return "", false
	}

	end := i
	i -= 2
	for i >= 1 {
		if src[i] == '/' && i+1 < len(src) && src[i+1] == '*' && i+2 < len(src) && src[i+2] == '*' {
			docComment := string(src[i : end+1])
			return docComment, true
		}
		i--
	}

	return "", false
}

// isJavaIdentChar checks if a byte is a valid Java identifier character.
func isJavaIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '.'
}

// nodeText extracts the text of a tree-sitter node.
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
