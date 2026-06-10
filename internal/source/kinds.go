package source

const (
	KindClass      = "class"
	KindInterface  = "interface"
	KindTrait      = "trait"
	KindFunction   = "function"
	KindStruct     = "struct"
	KindEnum       = "enum"
	KindModule     = "module"
	KindTypeAlias  = "type_alias"
	KindMacro      = "macro"
	KindConstant   = "constant"
	KindNamespace  = "namespace"
	KindRecord     = "record"
	KindDelegate   = "delegate"
	KindAnnotation = "annotation"
	KindConcept    = "concept"
	KindUnion      = "union"
)

var validKinds = map[string]bool{
	KindClass: true, KindInterface: true, KindTrait: true,
	KindFunction: true, KindStruct: true, KindEnum: true,
	KindModule: true, KindTypeAlias: true, KindMacro: true,
	KindConstant: true, KindNamespace: true, KindRecord: true,
	KindDelegate: true, KindAnnotation: true, KindConcept: true,
	KindUnion: true,
}

func ValidKind(s string) bool {
	return validKinds[s]
}
