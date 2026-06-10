package rust

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/hatlesswizard/defsource/internal/treesitter"
)

// fileAnalysis holds all AST-extracted entities from one Rust source file.
type fileAnalysis struct {
	Structs     []structDef
	Enums       []enumDef
	Traits      []traitDef
	Functions   []functionDef
	TypeAliases []typeAliasDef
	Macros      []macroDef
	Constants   []constantDef
	ImplBlocks  []implBlockDef
	ReExports   []reExport
}

// structDef holds data extracted from a struct_item AST node.
type structDef struct {
	Name       string
	Generics   string
	Fields     []fieldDef
	Derives    []string
	DocComment string
	StartPos   int
	EndPos     int
	Visibility string
}

// fieldDef holds data for a struct field.
type fieldDef struct {
	Name       string
	Type       string
	Visibility string
	DocComment string
}

// enumDef holds data extracted from an enum_item AST node.
type enumDef struct {
	Name       string
	Generics   string
	Variants   []variantDef
	DocComment string
	StartPos   int
	EndPos     int
	Visibility string
}

// variantDef holds data for an enum variant.
type variantDef struct {
	Name       string
	Fields     string // "(i32, i32)" or "{ x: i32 }" or ""
	DocComment string
}

// traitDef holds data extracted from a trait_item AST node.
type traitDef struct {
	Name       string
	Generics   string
	Methods    []methodDef
	AssocTypes []assocTypeDef
	DocComment string
	StartPos   int
	EndPos     int
	Visibility string
}

// assocTypeDef holds data for an associated type in a trait.
type assocTypeDef struct {
	Name  string
	Bound string
}

// functionDef holds data extracted from a function_item AST node.
type functionDef struct {
	Name       string
	Signature  string
	Params     []paramDef
	ReturnType string
	Generics   string
	IsAsync    bool
	IsUnsafe   bool
	DocComment string
	StartPos   int
	EndPos     int
	Visibility string
}

// paramDef holds data for a function/method parameter.
type paramDef struct {
	Name       string
	Type       string
	HasDefault bool
}

// methodDef holds data for a method within an impl block or trait.
type methodDef struct {
	Name       string
	Signature  string
	Params     []paramDef
	ReturnType string
	Generics   string
	IsAsync    bool
	IsUnsafe   bool
	HasBody    bool // false for trait method declarations without default impl
	DocComment string
	StartPos   int
	EndPos     int
	Visibility string
}

// implBlockDef holds data for an impl block.
type implBlockDef struct {
	TypeName  string
	TraitName string // empty for inherent impl
	Methods   []methodDef
	StartPos  int
	EndPos    int
}

// typeAliasDef holds data for a type alias.
type typeAliasDef struct {
	Name       string
	Type       string
	Generics   string
	DocComment string
	StartPos   int
	EndPos     int
	Visibility string
}

// macroDef holds data for a macro_rules! definition.
type macroDef struct {
	Name       string
	DocComment string
	StartPos   int
	EndPos     int
}

// constantDef holds data for a const or static item.
type constantDef struct {
	Name       string
	Type       string
	IsStatic   bool
	DocComment string
	StartPos   int
	EndPos     int
	Visibility string
}

// reExport holds data for a pub use re-export.
type reExport struct {
	Path  string // full path (e.g., "crate::runtime::Runtime")
	Alias string // renamed as, or empty
}

// parseFile parses a Rust source file using tree-sitter and returns the
// collected analysis.
func parseFile(src []byte) *fileAnalysis {
	parser, err := treesitter.Get(treesitter.Rust)
	if err != nil {
		return &fileAnalysis{}
	}
	defer treesitter.Put(treesitter.Rust, parser)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return &fileAnalysis{}
	}
	defer tree.Close()

	result := &fileAnalysis{}
	root := tree.RootNode()
	walkNode(root, src, result, true)
	return result
}

// walkNode recursively walks the AST, dispatching on node type.
// topLevel indicates whether we're at the module top level (for visibility filtering).
func walkNode(node *sitter.Node, src []byte, result *fileAnalysis, topLevel bool) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "struct_item":
		if sd := extractStruct(node, src); sd != nil {
			result.Structs = append(result.Structs, *sd)
		}
		return

	case "enum_item":
		if ed := extractEnum(node, src); ed != nil {
			result.Enums = append(result.Enums, *ed)
		}
		return

	case "trait_item":
		if td := extractTrait(node, src); td != nil {
			result.Traits = append(result.Traits, *td)
		}
		return

	case "function_item":
		if fd := extractFunction(node, src); fd != nil {
			result.Functions = append(result.Functions, *fd)
		}
		return

	case "impl_item":
		if ib := extractImplBlock(node, src); ib != nil {
			result.ImplBlocks = append(result.ImplBlocks, *ib)
		}
		return

	case "type_item":
		if ta := extractTypeAlias(node, src); ta != nil {
			result.TypeAliases = append(result.TypeAliases, *ta)
		}
		return

	case "macro_definition":
		if md := extractMacro(node, src); md != nil {
			result.Macros = append(result.Macros, *md)
		}
		return

	case "const_item":
		if cd := extractConstant(node, src, false); cd != nil {
			result.Constants = append(result.Constants, *cd)
		}
		return

	case "static_item":
		if cd := extractConstant(node, src, true); cd != nil {
			result.Constants = append(result.Constants, *cd)
		}
		return

	case "use_declaration":
		if re := extractReExport(node, src); re != nil {
			result.ReExports = append(result.ReExports, *re)
		}
		return

	case "mod_item":
		// Skip test modules
		if hasTestCfgAttr(node, src) {
			return
		}
		// Recurse into module body if inline
		if body := childByFieldOrType(node, "body", "declaration_list"); body != nil {
			count := int(body.NamedChildCount())
			for i := 0; i < count; i++ {
				walkNode(body.NamedChild(i), src, result, true)
			}
		}
		return
	}

	// Default: recurse into all named children.
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		walkNode(node.NamedChild(i), src, result, topLevel)
	}
}

// extractStruct extracts a struct definition from a struct_item node.
func extractStruct(node *sitter.Node, src []byte) *structDef {
	if isDocHidden(node, src) {
		return nil
	}

	vis := getVisibility(node, src)
	if vis != "public" {
		return nil
	}

	sd := &structDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		Visibility: vis,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		sd.Name = nodeText(nameNode, src)
	}
	if sd.Name == "" {
		return nil
	}

	if genNode := node.ChildByFieldName("type_parameters"); genNode != nil {
		sd.Generics = nodeText(genNode, src)
	}

	sd.DocComment = findPrecedingDocComment(src, sd.StartPos)
	sd.Derives = extractDerives(node, src)

	// Extract fields from field_declaration_list
	if body := childByFieldOrType(node, "body", "field_declaration_list"); body != nil {
		count := int(body.NamedChildCount())
		for i := 0; i < count; i++ {
			child := body.NamedChild(i)
			if child == nil {
				continue
			}
			if child.Type() == "field_declaration" {
				fd := extractField(child, src)
				if fd != nil {
					sd.Fields = append(sd.Fields, *fd)
				}
			}
		}
	}

	return sd
}

// extractField extracts a field definition from a field_declaration node.
func extractField(node *sitter.Node, src []byte) *fieldDef {
	fd := &fieldDef{
		Visibility: "private",
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		fd.Name = nodeText(nameNode, src)
	}
	if fd.Name == "" {
		return nil
	}

	if typeNode := node.ChildByFieldName("type"); typeNode != nil {
		fd.Type = nodeText(typeNode, src)
	}

	// Check visibility
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "visibility_modifier" {
			vis := nodeText(child, src)
			if vis == "pub" {
				fd.Visibility = "public"
			}
		}
	}

	fd.DocComment = findPrecedingDocComment(src, int(node.StartByte()))
	return fd
}

// extractEnum extracts an enum definition from an enum_item node.
func extractEnum(node *sitter.Node, src []byte) *enumDef {
	if isDocHidden(node, src) {
		return nil
	}

	vis := getVisibility(node, src)
	if vis != "public" {
		return nil
	}

	ed := &enumDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		Visibility: vis,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		ed.Name = nodeText(nameNode, src)
	}
	if ed.Name == "" {
		return nil
	}

	if genNode := node.ChildByFieldName("type_parameters"); genNode != nil {
		ed.Generics = nodeText(genNode, src)
	}

	ed.DocComment = findPrecedingDocComment(src, ed.StartPos)

	// Extract variants from enum_variant_list
	if body := childByFieldOrType(node, "body", "enum_variant_list"); body != nil {
		count := int(body.NamedChildCount())
		for i := 0; i < count; i++ {
			child := body.NamedChild(i)
			if child == nil {
				continue
			}
			if child.Type() == "enum_variant" {
				vd := extractVariant(child, src)
				if vd != nil {
					ed.Variants = append(ed.Variants, *vd)
				}
			}
		}
	}

	return ed
}

// extractVariant extracts an enum variant.
func extractVariant(node *sitter.Node, src []byte) *variantDef {
	vd := &variantDef{}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		vd.Name = nodeText(nameNode, src)
	}
	if vd.Name == "" {
		return nil
	}

	// Extract variant fields (tuple or struct style)
	if body := node.ChildByFieldName("body"); body != nil {
		vd.Fields = nodeText(body, src)
	}

	vd.DocComment = findPrecedingDocComment(src, int(node.StartByte()))
	return vd
}

// extractTrait extracts a trait definition from a trait_item node.
func extractTrait(node *sitter.Node, src []byte) *traitDef {
	if isDocHidden(node, src) {
		return nil
	}

	vis := getVisibility(node, src)
	if vis != "public" {
		return nil
	}

	td := &traitDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		Visibility: vis,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		td.Name = nodeText(nameNode, src)
	}
	if td.Name == "" {
		return nil
	}

	if genNode := node.ChildByFieldName("type_parameters"); genNode != nil {
		td.Generics = nodeText(genNode, src)
	}

	td.DocComment = findPrecedingDocComment(src, td.StartPos)

	// Extract trait body items
	if body := childByFieldOrType(node, "body", "declaration_list"); body != nil {
		count := int(body.NamedChildCount())
		for i := 0; i < count; i++ {
			child := body.NamedChild(i)
			if child == nil {
				continue
			}
			switch child.Type() {
			case "function_signature_item":
				if md := extractMethodSig(child, src, false); md != nil {
					td.Methods = append(td.Methods, *md)
				}
			case "function_item":
				if md := extractMethodFromFuncItem(child, src, true); md != nil {
					td.Methods = append(td.Methods, *md)
				}
			case "associated_type":
				if at := extractAssocType(child, src); at != nil {
					td.AssocTypes = append(td.AssocTypes, *at)
				}
			}
		}
	}

	return td
}

// extractAssocType extracts an associated type definition.
func extractAssocType(node *sitter.Node, src []byte) *assocTypeDef {
	at := &assocTypeDef{}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		at.Name = nodeText(nameNode, src)
	}
	if at.Name == "" {
		return nil
	}

	// Look for bounds
	if boundsNode := node.ChildByFieldName("bounds"); boundsNode != nil {
		at.Bound = nodeText(boundsNode, src)
	}

	return at
}

// extractFunction extracts a function definition from a function_item node.
func extractFunction(node *sitter.Node, src []byte) *functionDef {
	if isDocHidden(node, src) {
		return nil
	}

	vis := getVisibility(node, src)
	if vis != "public" {
		return nil
	}

	fd := &functionDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		Visibility: vis,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		fd.Name = nodeText(nameNode, src)
	}
	if fd.Name == "" {
		return nil
	}

	if genNode := node.ChildByFieldName("type_parameters"); genNode != nil {
		fd.Generics = nodeText(genNode, src)
	}

	fd.IsAsync = hasModifier(node, src, "async")
	fd.IsUnsafe = hasModifier(node, src, "unsafe")

	fd.Params = extractParams(node, src)
	fd.ReturnType = extractReturnType(node, src)
	fd.Signature = buildFuncSignature(fd)
	fd.DocComment = findPrecedingDocComment(src, fd.StartPos)

	return fd
}

// extractImplBlock extracts an impl block.
func extractImplBlock(node *sitter.Node, src []byte) *implBlockDef {
	ib := &implBlockDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	// Determine type name and optional trait name
	// impl Trait for Type { ... }  or  impl Type { ... }
	if typeNode := node.ChildByFieldName("type"); typeNode != nil {
		ib.TypeName = extractTypeName(typeNode, src)
	}

	if traitNode := node.ChildByFieldName("trait"); traitNode != nil {
		ib.TraitName = extractTypeName(traitNode, src)
	}

	if ib.TypeName == "" {
		return nil
	}

	// Extract methods from the body
	if body := childByFieldOrType(node, "body", "declaration_list"); body != nil {
		count := int(body.NamedChildCount())
		for i := 0; i < count; i++ {
			child := body.NamedChild(i)
			if child == nil {
				continue
			}
			if child.Type() == "function_item" {
				if md := extractMethodFromFuncItem(child, src, true); md != nil {
					ib.Methods = append(ib.Methods, *md)
				}
			}
		}
	}

	return ib
}

// extractMethodFromFuncItem extracts a methodDef from a function_item node
// inside an impl block or trait.
func extractMethodFromFuncItem(node *sitter.Node, src []byte, hasBody bool) *methodDef {
	if isDocHidden(node, src) {
		return nil
	}

	md := &methodDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
		HasBody:  hasBody,
	}

	vis := getVisibility(node, src)
	if vis == "" {
		// In impl blocks, default visibility depends on context
		vis = "public"
	}
	md.Visibility = vis

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		md.Name = nodeText(nameNode, src)
	}
	if md.Name == "" {
		return nil
	}

	if genNode := node.ChildByFieldName("type_parameters"); genNode != nil {
		md.Generics = nodeText(genNode, src)
	}

	md.IsAsync = hasModifier(node, src, "async")
	md.IsUnsafe = hasModifier(node, src, "unsafe")
	md.Params = extractParams(node, src)
	md.ReturnType = extractReturnType(node, src)
	md.Signature = buildMethodSignature(md)
	md.DocComment = findPrecedingDocComment(src, md.StartPos)

	return md
}

// extractMethodSig extracts a methodDef from a function_signature_item node
// (trait method without default body).
func extractMethodSig(node *sitter.Node, src []byte, hasBody bool) *methodDef {
	if isDocHidden(node, src) {
		return nil
	}

	md := &methodDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		HasBody:    hasBody,
		Visibility: "public",
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		md.Name = nodeText(nameNode, src)
	}
	if md.Name == "" {
		return nil
	}

	if genNode := node.ChildByFieldName("type_parameters"); genNode != nil {
		md.Generics = nodeText(genNode, src)
	}

	md.IsAsync = hasModifier(node, src, "async")
	md.IsUnsafe = hasModifier(node, src, "unsafe")
	md.Params = extractParams(node, src)
	md.ReturnType = extractReturnType(node, src)
	md.Signature = buildMethodSignature(md)
	md.DocComment = findPrecedingDocComment(src, md.StartPos)

	return md
}

// extractTypeAlias extracts a type alias from a type_item node.
func extractTypeAlias(node *sitter.Node, src []byte) *typeAliasDef {
	if isDocHidden(node, src) {
		return nil
	}

	vis := getVisibility(node, src)
	if vis != "public" {
		return nil
	}

	ta := &typeAliasDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		Visibility: vis,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		ta.Name = nodeText(nameNode, src)
	}
	if ta.Name == "" {
		return nil
	}

	if genNode := node.ChildByFieldName("type_parameters"); genNode != nil {
		ta.Generics = nodeText(genNode, src)
	}

	if typeNode := node.ChildByFieldName("type"); typeNode != nil {
		ta.Type = nodeText(typeNode, src)
	}

	ta.DocComment = findPrecedingDocComment(src, ta.StartPos)
	return ta
}

// extractMacro extracts a macro_rules! definition.
func extractMacro(node *sitter.Node, src []byte) *macroDef {
	if isDocHidden(node, src) {
		return nil
	}

	md := &macroDef{
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		md.Name = nodeText(nameNode, src)
	}
	if md.Name == "" {
		return nil
	}

	md.DocComment = findPrecedingDocComment(src, md.StartPos)
	return md
}

// extractConstant extracts a const or static item.
func extractConstant(node *sitter.Node, src []byte, isStatic bool) *constantDef {
	if isDocHidden(node, src) {
		return nil
	}

	vis := getVisibility(node, src)
	if vis != "public" {
		return nil
	}

	cd := &constantDef{
		StartPos:   int(node.StartByte()),
		EndPos:     int(node.EndByte()),
		IsStatic:   isStatic,
		Visibility: vis,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		cd.Name = nodeText(nameNode, src)
	}
	if cd.Name == "" {
		return nil
	}

	if typeNode := node.ChildByFieldName("type"); typeNode != nil {
		cd.Type = nodeText(typeNode, src)
	}

	cd.DocComment = findPrecedingDocComment(src, cd.StartPos)
	return cd
}

// extractReExport extracts a pub use re-export.
func extractReExport(node *sitter.Node, src []byte) *reExport {
	vis := getVisibility(node, src)
	if vis != "public" {
		return nil
	}

	// Get the full use path text
	text := nodeText(node, src)
	// Strip "pub use " prefix and trailing ";"
	text = strings.TrimPrefix(text, "pub use ")
	text = strings.TrimSuffix(text, ";")
	text = strings.TrimSpace(text)

	if text == "" {
		return nil
	}

	re := &reExport{Path: text}

	// Check for "as Alias" pattern
	if idx := strings.LastIndex(text, " as "); idx >= 0 {
		re.Path = strings.TrimSpace(text[:idx])
		re.Alias = strings.TrimSpace(text[idx+4:])
	}

	return re
}

// extractParams extracts function/method parameters from a function node.
func extractParams(node *sitter.Node, src []byte) []paramDef {
	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode == nil {
		return nil
	}

	var params []paramDef
	count := int(paramsNode.NamedChildCount())
	for i := 0; i < count; i++ {
		child := paramsNode.NamedChild(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "self_parameter":
			// &self, &mut self, self, mut self
			text := nodeText(child, src)
			params = append(params, paramDef{Name: text, Type: "Self"})

		case "parameter":
			pd := paramDef{}
			if patNode := child.ChildByFieldName("pattern"); patNode != nil {
				pd.Name = nodeText(patNode, src)
			}
			if typeNode := child.ChildByFieldName("type"); typeNode != nil {
				pd.Type = nodeText(typeNode, src)
			}
			if pd.Name != "" {
				params = append(params, pd)
			}
		}
	}

	return params
}

// extractReturnType extracts the return type from a function node.
func extractReturnType(node *sitter.Node, src []byte) string {
	if retNode := node.ChildByFieldName("return_type"); retNode != nil {
		text := nodeText(retNode, src)
		// Strip the leading "-> " if present
		text = strings.TrimPrefix(text, "-> ")
		return strings.TrimSpace(text)
	}
	return ""
}

// extractTypeName extracts a clean type name from a type node, stripping generics.
func extractTypeName(node *sitter.Node, src []byte) string {
	text := nodeText(node, src)
	// Strip generic parameters for the base name
	if idx := strings.Index(text, "<"); idx > 0 {
		return strings.TrimSpace(text[:idx])
	}
	return strings.TrimSpace(text)
}

// extractDerives extracts #[derive(...)] attributes from a node.
func extractDerives(node *sitter.Node, src []byte) []string {
	var derives []string
	startByte := int(node.StartByte())

	// Look backwards from the node start for attribute nodes
	// We scan the preceding text for #[derive(...)]
	docComment := findPrecedingDocComment(src, startByte)
	_ = docComment

	// Scan preceding content for #[derive(...)] attributes
	searchStart := startByte - 500
	if searchStart < 0 {
		searchStart = 0
	}
	preceding := string(src[searchStart:startByte])

	for {
		idx := strings.Index(preceding, "#[derive(")
		if idx < 0 {
			break
		}
		// Find the closing paren
		start := idx + len("#[derive(")
		end := strings.Index(preceding[start:], ")]")
		if end < 0 {
			break
		}
		deriveList := preceding[start : start+end]
		for _, d := range strings.Split(deriveList, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				derives = append(derives, d)
			}
		}
		preceding = preceding[start+end:]
	}

	return derives
}

// buildFuncSignature builds a display signature for a function.
func buildFuncSignature(fd *functionDef) string {
	var sb strings.Builder

	if fd.IsUnsafe {
		sb.WriteString("unsafe ")
	}
	if fd.IsAsync {
		sb.WriteString("async ")
	}
	sb.WriteString("fn ")
	sb.WriteString(fd.Name)
	if fd.Generics != "" {
		sb.WriteString(fd.Generics)
	}
	sb.WriteString("(")

	for i, p := range fd.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		if p.Name == "self" || p.Name == "&self" || p.Name == "&mut self" || p.Name == "mut self" {
			sb.WriteString(p.Name)
		} else {
			sb.WriteString(p.Name)
			if p.Type != "" {
				sb.WriteString(": ")
				sb.WriteString(p.Type)
			}
		}
	}

	sb.WriteString(")")
	if fd.ReturnType != "" {
		sb.WriteString(" -> ")
		sb.WriteString(fd.ReturnType)
	}
	return sb.String()
}

// buildMethodSignature builds a display signature for a method.
func buildMethodSignature(md *methodDef) string {
	var sb strings.Builder

	if md.IsUnsafe {
		sb.WriteString("unsafe ")
	}
	if md.IsAsync {
		sb.WriteString("async ")
	}
	sb.WriteString("fn ")
	sb.WriteString(md.Name)
	if md.Generics != "" {
		sb.WriteString(md.Generics)
	}
	sb.WriteString("(")

	for i, p := range md.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		if p.Name == "self" || p.Name == "&self" || p.Name == "&mut self" || p.Name == "mut self" {
			sb.WriteString(p.Name)
		} else {
			sb.WriteString(p.Name)
			if p.Type != "" {
				sb.WriteString(": ")
				sb.WriteString(p.Type)
			}
		}
	}

	sb.WriteString(")")
	if md.ReturnType != "" {
		sb.WriteString(" -> ")
		sb.WriteString(md.ReturnType)
	}
	return sb.String()
}

// --- Utility functions ---

// nodeText returns the text content of a tree-sitter node.
func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	return string(src[node.StartByte():node.EndByte()])
}

// getVisibility returns the visibility level of a node ("public", "private", or "").
// Only "pub" (without restriction) counts as "public".
func getVisibility(node *sitter.Node, src []byte) string {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "visibility_modifier" {
			text := nodeText(child, src)
			// "pub" is public, "pub(crate)", "pub(super)", "pub(self)" are not fully public
			if text == "pub" {
				return "public"
			}
			// pub(crate), pub(super), pub(in ...) are restricted
			return "private"
		}
	}
	return "private"
}

// hasModifier checks if a function/method node has a specific modifier (async, unsafe, etc.).
func hasModifier(node *sitter.Node, src []byte, modifier string) bool {
	count := int(node.ChildCount())
	for i := 0; i < count; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		text := nodeText(child, src)
		if text == modifier {
			return true
		}
	}
	return false
}

// isDocHidden checks if an item has #[doc(hidden)] attribute.
// It looks at the region immediately preceding the node (between the end of
// the previous sibling/item and this node's start) for #[doc(hidden)].
func isDocHidden(node *sitter.Node, src []byte) bool {
	startByte := int(node.StartByte())
	if startByte <= 0 {
		return false
	}

	// Find the boundary: scan backwards from startByte to find a line that
	// does NOT start with //, #[, or whitespace. This marks the end of the
	// preceding item, limiting our attribute search window.
	searchStart := startByte - 1
	// Walk backwards past attributes and doc comments that belong to this node
	for searchStart > 0 {
		// Find start of current line
		lineStart := searchStart
		for lineStart > 0 && src[lineStart-1] != '\n' {
			lineStart--
		}
		line := strings.TrimSpace(string(src[lineStart : searchStart+1]))

		// Lines that can appear between items and attributes
		if line == "" || strings.HasPrefix(line, "///") || strings.HasPrefix(line, "//!") || strings.HasPrefix(line, "#[") {
			searchStart = lineStart - 1
			if searchStart < 0 {
				searchStart = 0
			}
			continue
		}
		// We've hit a non-attribute, non-comment, non-empty line -- stop
		searchStart = lineStart
		break
	}
	if searchStart < 0 {
		searchStart = 0
	}

	preceding := string(src[searchStart:startByte])
	return strings.Contains(preceding, "#[doc(hidden)]")
}

// hasTestCfgAttr checks if a module node has #[cfg(test)] attribute.
func hasTestCfgAttr(node *sitter.Node, src []byte) bool {
	startByte := int(node.StartByte())
	searchStart := startByte - 100
	if searchStart < 0 {
		searchStart = 0
	}
	preceding := string(src[searchStart:startByte])
	return strings.Contains(preceding, "#[cfg(test)]")
}

// findPrecedingDocComment finds /// or //! doc comments before a given position.
func findPrecedingDocComment(src []byte, pos int) string {
	if pos <= 0 {
		return ""
	}

	// Scan backwards from pos to find doc comment lines
	i := pos - 1

	// Skip whitespace
	for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i--
	}

	// Skip #[...] attributes between doc comment and declaration
	for i >= 0 && src[i] == ']' {
		depth := 1
		i--
		for i >= 0 && depth > 0 {
			if src[i] == ']' {
				depth++
			} else if src[i] == '[' {
				depth--
			}
			if depth > 0 {
				i--
			}
		}
		if i >= 1 && src[i] == '[' && src[i-1] == '#' {
			i -= 2
		} else {
			break
		}
		for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
			i--
		}
	}

	// Now we should be at the end of doc comment lines.
	// Scan backwards to collect all consecutive doc comment lines (/// or //!).
	endOfComments := i + 1

	// Find the start of the comment block by scanning backwards line by line.
	var lines []string
	lineEnd := endOfComments

	for lineEnd > 0 {
		// Find start of current line
		lineStart := lineEnd - 1
		for lineStart > 0 && src[lineStart-1] != '\n' {
			lineStart--
		}

		line := strings.TrimSpace(string(src[lineStart:lineEnd]))

		if strings.HasPrefix(line, "///") || strings.HasPrefix(line, "//!") {
			lines = append([]string{line}, lines...)
			lineEnd = lineStart
			// Skip the newline
			if lineEnd > 0 && src[lineEnd-1] == '\n' {
				lineEnd--
			}
			if lineEnd > 0 && src[lineEnd-1] == '\r' {
				lineEnd--
			}
		} else {
			break
		}
	}

	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n")
}

// childByFieldOrType looks up a child node by field name first, then falls
// back to searching by node type name.
func childByFieldOrType(node *sitter.Node, fieldName, typeName string) *sitter.Node {
	if child := node.ChildByFieldName(fieldName); child != nil {
		return child
	}
	count := int(node.NamedChildCount())
	for i := 0; i < count; i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type() == typeName {
			return child
		}
	}
	return nil
}
