package golang

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("go/stdlib", NewStdlibFactory)
	registry.Default.Register("go/gin", NewGinFactory)
	registry.Default.Register("go/echo", NewEchoFactory)
	registry.Default.Register("go/gorm", NewGormFactory)
	registry.Default.Register("go/fiber", NewFiberFactory)
	registry.Default.Register("go/chi", NewChiFactory)
	registry.Default.Register("go/beego", NewBeegoFactory)
	registry.Default.Register("go/buffalo", NewBuffaloFactory)
	registry.Default.Register("go/iris", NewIrisFactory)
	registry.Default.Register("go/revel", NewRevelFactory)
	registry.Default.Register("go/kratos", NewKratosFactory)
	registry.Default.Register("go/go-zero", NewGoZeroFactory)
	registry.Default.Register("go/hertz", NewHertzFactory)
	registry.Default.Register("go/goframe", NewGoFrameFactory)
}

// NewStdlibFactory creates a Source for the Go standard library.
func NewStdlibFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/stdlib",
		Name:            "Go Standard Library",
		Description:     "Complete reference for the Go standard library",
		SourceURL:       "https://github.com/golang/go",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "golang/go",
		RootDirs:        []string{"src"},
		ExcludeDirs:     []string{"cmd", "testdata", "vendor"},
	}
	return New(repoPath, cfg), nil
}

// NewGinFactory creates a Source for the Gin web framework.
func NewGinFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/gin",
		Name:            "Gin Web Framework",
		Description:     "HTTP web framework for Go with performance and productivity",
		SourceURL:       "https://github.com/gin-gonic/gin",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "gin-gonic/gin",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}

// NewEchoFactory creates a Source for the Echo web framework.
func NewEchoFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/echo",
		Name:            "Echo Web Framework",
		Description:     "High performance, minimalist Go web framework",
		SourceURL:       "https://github.com/labstack/echo",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "labstack/echo",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor"},
	}
	return New(repoPath, cfg), nil
}

// NewGormFactory creates a Source for the GORM ORM library.
func NewGormFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/gorm",
		Name:            "GORM ORM",
		Description:     "Object-Relational Mapping library for Go",
		SourceURL:       "https://github.com/go-gorm/gorm",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "go-gorm/gorm",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "tests"},
	}
	return New(repoPath, cfg), nil
}

// NewFiberFactory creates a Source for the Fiber web framework.
func NewFiberFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/fiber",
		Name:            "Fiber",
		Description:     "Express-inspired web framework for Go",
		SourceURL:       "https://github.com/gofiber/fiber",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "gofiber/fiber",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}

// NewChiFactory creates a Source for the Chi HTTP framework.
func NewChiFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/chi",
		Name:            "Chi",
		Description:     "Lightweight HTTP framework for Go",
		SourceURL:       "https://github.com/go-chi/chi",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "go-chi/chi",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}

// NewBeegoFactory creates a Source for the Beego web framework.
func NewBeegoFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/beego",
		Name:            "Beego",
		Description:     "Full-stack MVC framework for Go",
		SourceURL:       "https://github.com/beego/beego",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "beego/beego",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}

// NewBuffaloFactory creates a Source for the Buffalo web framework.
func NewBuffaloFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/buffalo",
		Name:            "Buffalo",
		Description:     "Rapid web dev framework for Go",
		SourceURL:       "https://github.com/gobuffalo/buffalo",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "gobuffalo/buffalo",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}

// NewIrisFactory creates a Source for the Iris web framework.
func NewIrisFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/iris",
		Name:            "Iris",
		Description:     "Fast web framework for Go",
		SourceURL:       "https://github.com/kataras/iris",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "kataras/iris",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}

// NewRevelFactory creates a Source for the Revel web framework.
func NewRevelFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/revel",
		Name:            "Revel",
		Description:     "Full-stack web framework for Go",
		SourceURL:       "https://github.com/revel/revel",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "revel/revel",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}

// NewKratosFactory creates a Source for the Kratos microservice framework.
func NewKratosFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/kratos",
		Name:            "Kratos",
		Description:     "Microservice framework for Go",
		SourceURL:       "https://github.com/go-kratos/kratos",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "go-kratos/kratos",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}

// NewGoZeroFactory creates a Source for the go-zero microservice framework.
func NewGoZeroFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/go-zero",
		Name:            "go-zero",
		Description:     "Cloud-native microservice framework for Go",
		SourceURL:       "https://github.com/zeromicro/go-zero",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "zeromicro/go-zero",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}

// NewHertzFactory creates a Source for the Hertz HTTP framework.
func NewHertzFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/hertz",
		Name:            "Hertz",
		Description:     "HTTP framework (ByteDance) for Go",
		SourceURL:       "https://github.com/cloudwego/hertz",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "cloudwego/hertz",
		RootDirs:        []string{"pkg"},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}

// NewGoFrameFactory creates a Source for the GoFrame web framework.
func NewGoFrameFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:       "go/goframe",
		Name:            "GoFrame",
		Description:     "Modular web framework for Go",
		SourceURL:       "https://github.com/gogf/gf",
		Ref:             opts.Ref,
		GitHubOwnerRepo: "gogf/gf",
		RootDirs:        []string{""},
		ExcludeDirs:     []string{"testdata", "vendor", "examples"},
	}
	return New(repoPath, cfg), nil
}
