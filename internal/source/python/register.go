package python

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("python/stdlib", NewStdlibFactory)
	registry.Default.Register("python/django", NewDjangoFactory)
	registry.Default.Register("python/flask", NewFlaskFactory)
	registry.Default.Register("python/fastapi", NewFastAPIFactory)
	registry.Default.Register("python/numpy", NewNumPyFactory)
	registry.Default.Register("python/pandas", NewPandasFactory)
	registry.Default.Register("python/sqlalchemy", NewSQLAlchemyFactory)
	registry.Default.Register("python/requests", NewRequestsFactory)
	registry.Default.Register("python/pytest", NewPytestFactory)
	registry.Default.Register("python/celery", NewCeleryFactory)
	registry.Default.Register("python/pydantic", NewPydanticFactory)
	registry.Default.Register("python/scikit-learn", NewScikitLearnFactory)
	registry.Default.Register("python/click", NewClickFactory)
	registry.Default.Register("python/tornado", NewTornadoFactory)
	registry.Default.Register("python/pyramid", NewPyramidFactory)
	registry.Default.Register("python/sanic", NewSanicFactory)
	registry.Default.Register("python/starlette", NewStarletteFactory)
	registry.Default.Register("python/aiohttp", NewAiohttpFactory)
	registry.Default.Register("python/falcon", NewFalconFactory)
	registry.Default.Register("python/bottle", NewBottleFactory)
	registry.Default.Register("python/dash", NewDashFactory)
	registry.Default.Register("python/quart", NewQuartFactory)
	registry.Default.Register("python/litestar", NewLitestarFactory)
	registry.Default.Register("python/robyn", NewRobynFactory)
}

// NewStdlibFactory creates a Source for the Python standard library.
func NewStdlibFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/stdlib",
		Name:        "Python Standard Library",
		Description: "Complete reference for the Python standard library",
		SourceURL:   "https://github.com/python/cpython",
		SourceRoots: []string{"Lib"},
		TrustScore:  0.95,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewDjangoFactory creates a Source for the Django web framework.
func NewDjangoFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/django",
		Name:        "Django",
		Description: "High-level Python web framework for rapid development",
		SourceURL:   "https://github.com/django/django",
		SourceRoots: []string{"django"},
		TrustScore:  0.92,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewFlaskFactory creates a Source for the Flask microframework.
func NewFlaskFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/flask",
		Name:        "Flask",
		Description: "Lightweight WSGI web application framework",
		SourceURL:   "https://github.com/pallets/flask",
		SourceRoots: []string{"src/flask"},
		TrustScore:  0.90,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewFastAPIFactory creates a Source for the FastAPI framework.
func NewFastAPIFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/fastapi",
		Name:        "FastAPI",
		Description: "Modern, fast web framework for building APIs with Python type hints",
		SourceURL:   "https://github.com/tiangolo/fastapi",
		SourceRoots: []string{"fastapi"},
		TrustScore:  0.90,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewNumPyFactory creates a Source for the NumPy library.
func NewNumPyFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/numpy",
		Name:        "NumPy",
		Description: "Fundamental package for scientific computing with Python",
		SourceURL:   "https://github.com/numpy/numpy",
		SourceRoots: []string{"numpy"},
		TrustScore:  0.92,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewPandasFactory creates a Source for the Pandas library.
func NewPandasFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/pandas",
		Name:        "Pandas",
		Description: "Powerful data structures and data analysis tools for Python",
		SourceURL:   "https://github.com/pandas-dev/pandas",
		SourceRoots: []string{"pandas"},
		TrustScore:  0.92,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewSQLAlchemyFactory creates a Source for the SQLAlchemy ORM.
func NewSQLAlchemyFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/sqlalchemy",
		Name:        "SQLAlchemy",
		Description: "Python SQL toolkit and Object Relational Mapper",
		SourceURL:   "https://github.com/sqlalchemy/sqlalchemy",
		SourceRoots: []string{"lib/sqlalchemy"},
		TrustScore:  0.90,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewRequestsFactory creates a Source for the Requests library.
func NewRequestsFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/requests",
		Name:        "Requests",
		Description: "Elegant and simple HTTP library for Python",
		SourceURL:   "https://github.com/psf/requests",
		SourceRoots: []string{"src/requests"},
		TrustScore:  0.90,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewPytestFactory creates a Source for the pytest testing framework.
func NewPytestFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/pytest",
		Name:        "pytest",
		Description: "Full-featured Python testing framework",
		SourceURL:   "https://github.com/pytest-dev/pytest",
		SourceRoots: []string{"src/_pytest"},
		TrustScore:  0.90,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewCeleryFactory creates a Source for the Celery task queue.
func NewCeleryFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/celery",
		Name:        "Celery",
		Description: "Distributed task queue for Python",
		SourceURL:   "https://github.com/celery/celery",
		SourceRoots: []string{"celery"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewPydanticFactory creates a Source for the Pydantic data validation library.
func NewPydanticFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/pydantic",
		Name:        "Pydantic",
		Description: "Data validation using Python type annotations",
		SourceURL:   "https://github.com/pydantic/pydantic",
		SourceRoots: []string{"pydantic"},
		TrustScore:  0.90,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewScikitLearnFactory creates a Source for the scikit-learn machine learning library.
func NewScikitLearnFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/scikit-learn",
		Name:        "scikit-learn",
		Description: "Machine learning library for Python",
		SourceURL:   "https://github.com/scikit-learn/scikit-learn",
		SourceRoots: []string{"sklearn"},
		TrustScore:  0.92,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewClickFactory creates a Source for the Click CLI framework.
func NewClickFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/click",
		Name:        "Click",
		Description: "Composable command-line interface toolkit for Python",
		SourceURL:   "https://github.com/pallets/click",
		SourceRoots: []string{"src/click"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewTornadoFactory creates a Source for the Tornado web framework.
func NewTornadoFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/tornado",
		Name:        "Tornado",
		Description: "Async web framework for Python",
		SourceURL:   "https://github.com/tornadoweb/tornado",
		SourceRoots: []string{"tornado/"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewPyramidFactory creates a Source for the Pyramid web framework.
func NewPyramidFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/pyramid",
		Name:        "Pyramid",
		Description: "Flexible web framework for Python",
		SourceURL:   "https://github.com/Pylons/pyramid",
		SourceRoots: []string{"src/pyramid/"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewSanicFactory creates a Source for the Sanic web framework.
func NewSanicFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/sanic",
		Name:        "Sanic",
		Description: "Async web framework for Python",
		SourceURL:   "https://github.com/sanic-org/sanic",
		SourceRoots: []string{"sanic/"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewStarletteFactory creates a Source for the Starlette ASGI framework.
func NewStarletteFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/starlette",
		Name:        "Starlette",
		Description: "ASGI framework for Python",
		SourceURL:   "https://github.com/encode/starlette",
		SourceRoots: []string{"starlette/"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewAiohttpFactory creates a Source for the aiohttp async HTTP framework.
func NewAiohttpFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/aiohttp",
		Name:        "aiohttp",
		Description: "Async HTTP framework for Python",
		SourceURL:   "https://github.com/aio-libs/aiohttp",
		SourceRoots: []string{"aiohttp/"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewFalconFactory creates a Source for the Falcon REST framework.
func NewFalconFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/falcon",
		Name:        "Falcon",
		Description: "Minimalist REST framework for Python",
		SourceURL:   "https://github.com/falconry/falcon",
		SourceRoots: []string{"falcon/"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewBottleFactory creates a Source for the Bottle micro framework.
func NewBottleFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/bottle",
		Name:        "Bottle",
		Description: "Micro framework for Python",
		SourceURL:   "https://github.com/bottlepy/bottle",
		SourceRoots: []string{"./"},
		TrustScore:  0.85,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewDashFactory creates a Source for the Dash analytical web framework.
func NewDashFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/dash",
		Name:        "Dash",
		Description: "Analytical web framework for Python",
		SourceURL:   "https://github.com/plotly/dash",
		SourceRoots: []string{"dash/"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewQuartFactory creates a Source for the Quart async framework.
func NewQuartFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/quart",
		Name:        "Quart",
		Description: "Async Flask-like framework for Python",
		SourceURL:   "https://github.com/pallets/quart",
		SourceRoots: []string{"src/quart/"},
		TrustScore:  0.85,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewLitestarFactory creates a Source for the Litestar ASGI framework.
func NewLitestarFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/litestar",
		Name:        "Litestar",
		Description: "Modern ASGI framework for Python",
		SourceURL:   "https://github.com/litestar-org/litestar",
		SourceRoots: []string{"litestar/"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewRobynFactory creates a Source for the Robyn async framework.
func NewRobynFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "python/robyn",
		Name:        "Robyn",
		Description: "Fast async framework for Python",
		SourceURL:   "https://github.com/sparckles/Robyn",
		SourceRoots: []string{"robyn/"},
		TrustScore:  0.85,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}
