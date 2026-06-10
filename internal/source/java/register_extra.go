package java

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("java/helidon", NewHelidonFactory)
	registry.Default.Register("java/blade", NewBladeFactory)
	registry.Default.Register("java/jooby", NewJoobyFactory)
}

// NewHelidonFactory creates a Source for the Helidon cloud-native framework.
func NewHelidonFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:           "helidon-io",
		Repo:            "helidon",
		LibraryID:       "java/helidon",
		Name:            "Helidon",
		Description:     "Oracle cloud-native framework for Java",
		SourceURL:       "https://github.com/helidon-io/helidon",
		SourceRoots:     []string{"webserver/webserver/src/main/java/"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewBladeFactory creates a Source for the Blade lightweight MVC framework.
func NewBladeFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:           "lets-blade",
		Repo:            "blade",
		LibraryID:       "java/blade",
		Name:            "Blade",
		Description:     "Lightweight MVC framework on Netty4 for Java",
		SourceURL:       "https://github.com/lets-blade/blade",
		// Release tags use a multi-module layout (blade-core); fall back
		// to the single-module layout used on the development branch.
		SourceRoots:     []string{"blade-core/src/main/java", "src/main/java"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewJoobyFactory creates a Source for the Jooby web framework.
// Replaces Ktor, which is written in Kotlin and cannot be indexed by
// the Java adapter.
func NewJoobyFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:           "jooby-project",
		Repo:            "jooby",
		LibraryID:       "java/jooby",
		Name:            "Jooby",
		Description:     "Modular web framework for Java",
		SourceURL:       "https://github.com/jooby-project/jooby",
		SourceRoots:     []string{"jooby/src/main/java"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}
