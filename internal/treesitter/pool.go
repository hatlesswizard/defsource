package treesitter

import (
	"fmt"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	ts_c "github.com/smacker/go-tree-sitter/c"
	ts_cpp "github.com/smacker/go-tree-sitter/cpp"
	ts_csharp "github.com/smacker/go-tree-sitter/csharp"
	ts_golang "github.com/smacker/go-tree-sitter/golang"
	ts_java "github.com/smacker/go-tree-sitter/java"
	ts_javascript "github.com/smacker/go-tree-sitter/javascript"
	ts_php "github.com/smacker/go-tree-sitter/php"
	ts_python "github.com/smacker/go-tree-sitter/python"
	ts_ruby "github.com/smacker/go-tree-sitter/ruby"
	ts_rust "github.com/smacker/go-tree-sitter/rust"
	ts_typescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

type Language string

const (
	PHP        Language = "php"
	JavaScript Language = "javascript"
	TypeScript Language = "typescript"
	Python     Language = "python"
	Go         Language = "go"
	Java       Language = "java"
	C          Language = "c"
	Cpp        Language = "cpp"
	CSharp     Language = "csharp"
	Ruby       Language = "ruby"
	Rust       Language = "rust"
)

var languages = map[Language]*sitter.Language{
	PHP:        ts_php.GetLanguage(),
	JavaScript: ts_javascript.GetLanguage(),
	TypeScript: ts_typescript.GetLanguage(),
	Python:     ts_python.GetLanguage(),
	Go:         ts_golang.GetLanguage(),
	Java:       ts_java.GetLanguage(),
	C:          ts_c.GetLanguage(),
	Cpp:        ts_cpp.GetLanguage(),
	CSharp:     ts_csharp.GetLanguage(),
	Ruby:       ts_ruby.GetLanguage(),
	Rust:       ts_rust.GetLanguage(),
}

func GetLanguage(lang Language) (*sitter.Language, error) {
	l, ok := languages[lang]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %q", lang)
	}
	return l, nil
}

type Pool struct {
	pools map[Language]*sync.Pool
}

var defaultPool *Pool

func init() {
	defaultPool = newPool()
}

func newPool() *Pool {
	p := &Pool{
		pools: make(map[Language]*sync.Pool, len(languages)),
	}
	for lang, grammar := range languages {
		g := grammar
		p.pools[lang] = &sync.Pool{
			New: func() any {
				parser := sitter.NewParser()
				parser.SetLanguage(g)
				return parser
			},
		}
	}
	return p
}

func Get(lang Language) (*sitter.Parser, error) {
	return defaultPool.Get(lang)
}

func Put(lang Language, parser *sitter.Parser) {
	defaultPool.Put(lang, parser)
}

func (p *Pool) Get(lang Language) (*sitter.Parser, error) {
	pool, ok := p.pools[lang]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %q", lang)
	}
	return pool.Get().(*sitter.Parser), nil
}

func (p *Pool) Put(lang Language, parser *sitter.Parser) {
	pool, ok := p.pools[lang]
	if !ok {
		return
	}
	pool.Put(parser)
}

func SupportedLanguages() []Language {
	langs := make([]Language, 0, len(languages))
	for l := range languages {
		langs = append(langs, l)
	}
	return langs
}
