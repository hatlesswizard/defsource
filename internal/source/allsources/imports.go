// Package allsources is a convenience import that triggers init() registration
// of all language source adapters into the default registry. Binaries that need
// access to all registered sources should import this package for side effects:
//
//	import _ "github.com/hatlesswizard/defsource/internal/source/allsources"
package allsources

import (
	_ "github.com/hatlesswizard/defsource/internal/source/clang"
	_ "github.com/hatlesswizard/defsource/internal/source/cpp"
	_ "github.com/hatlesswizard/defsource/internal/source/csharp"
	_ "github.com/hatlesswizard/defsource/internal/source/golang"
	_ "github.com/hatlesswizard/defsource/internal/source/java"
	_ "github.com/hatlesswizard/defsource/internal/source/javascript"
	_ "github.com/hatlesswizard/defsource/internal/source/python"
	_ "github.com/hatlesswizard/defsource/internal/source/ruby"
	_ "github.com/hatlesswizard/defsource/internal/source/rust"
	_ "github.com/hatlesswizard/defsource/internal/source/typescript"
	_ "github.com/hatlesswizard/defsource/internal/source/php"
)
