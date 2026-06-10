package php

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("php/wordpress", NewWordPressFactory)
	registry.Default.Register("php/laravel", NewLaravelFactory)
	registry.Default.Register("php/symfony", NewSymfonyFactory)
	registry.Default.Register("php/doctrine", NewDoctrineFactory)
	registry.Default.Register("php/phpunit", NewPHPUnitFactory)
	registry.Default.Register("php/guzzle", NewGuzzleFactory)
	registry.Default.Register("php/monolog", NewMonologFactory)
	registry.Default.Register("php/twig", NewTwigFactory)
	registry.Default.Register("php/carbon", NewCarbonFactory)
	registry.Default.Register("php/pest", NewPestFactory)
	registry.Default.Register("php/composer", NewComposerFactory)
	registry.Default.Register("php/codeigniter", NewCodeIgniterFactory)
	registry.Default.Register("php/cakephp", NewCakePHPFactory)
	registry.Default.Register("php/yii", NewYiiFactory)
	registry.Default.Register("php/slim", NewSlimFactory)
	registry.Default.Register("php/laminas", NewLaminasFactory)
	registry.Default.Register("php/phalcon", NewPhalconFactory)
	registry.Default.Register("php/livewire", NewLivewireFactory)
	registry.Default.Register("php/swoole-framework", NewHyperfFactory)
	registry.Default.Alias("php", "php/wordpress")
	registry.Default.Alias("wpgithub", "php/wordpress")
}

// NewWordPressFactory creates a Source for WordPress PHP.
func NewWordPressFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, sopts...), nil
}

// NewLaravelFactory creates a Source for the Laravel framework.
func NewLaravelFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/laravel",
		Name:        "Laravel",
		Description: "PHP web application framework with expressive, elegant syntax",
		SourceURL:   "https://github.com/laravel/framework",
		SourceDirs:  []string{"src/Illuminate"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewSymfonyFactory creates a Source for the Symfony framework.
func NewSymfonyFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/symfony",
		Name:        "Symfony",
		Description: "Set of reusable PHP components and a web application framework",
		SourceURL:   "https://github.com/symfony/symfony",
		SourceDirs:  []string{"src/Symfony/Component", "src/Symfony/Bridge"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewDoctrineFactory creates a Source for Doctrine ORM.
func NewDoctrineFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/doctrine",
		Name:        "Doctrine ORM",
		Description: "Object-Relational Mapper for PHP providing transparent persistence",
		SourceURL:   "https://github.com/doctrine/orm",
		SourceDirs:  []string{"lib/", "src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewPHPUnitFactory creates a Source for PHPUnit.
func NewPHPUnitFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/phpunit",
		Name:        "PHPUnit",
		Description: "Programmer-oriented testing framework for PHP",
		SourceURL:   "https://github.com/sebastianbergmann/phpunit",
		SourceDirs:  []string{"src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewGuzzleFactory creates a Source for Guzzle HTTP client.
func NewGuzzleFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/guzzle",
		Name:        "Guzzle",
		Description: "Extensible PHP HTTP client with middleware architecture",
		SourceURL:   "https://github.com/guzzle/guzzle",
		SourceDirs:  []string{"src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewMonologFactory creates a Source for Monolog.
func NewMonologFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/monolog",
		Name:        "Monolog",
		Description: "Logging library for PHP with handler stack architecture",
		SourceURL:   "https://github.com/Seldaek/monolog",
		SourceDirs:  []string{"src/Monolog"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewTwigFactory creates a Source for the Twig template engine.
func NewTwigFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/twig",
		Name:        "Twig",
		Description: "Flexible, fast, and secure template engine for PHP",
		SourceURL:   "https://github.com/twigphp/Twig",
		SourceDirs:  []string{"src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewCarbonFactory creates a Source for Carbon date library.
func NewCarbonFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/carbon",
		Name:        "Carbon",
		Description: "Simple PHP API extension for DateTime",
		SourceURL:   "https://github.com/briannesbitt/Carbon",
		SourceDirs:  []string{"src/Carbon"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewPestFactory creates a Source for the Pest testing framework.
func NewPestFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/pest",
		Name:        "Pest",
		Description: "Elegant PHP testing framework with a focus on simplicity",
		SourceURL:   "https://github.com/pestphp/pest",
		SourceDirs:  []string{"src"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewComposerFactory creates a Source for Composer.
func NewComposerFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/composer",
		Name:        "Composer",
		Description: "Dependency manager for PHP",
		SourceURL:   "https://github.com/composer/composer",
		SourceDirs:  []string{"src/Composer"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewCodeIgniterFactory creates a Source for the CodeIgniter framework.
func NewCodeIgniterFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/codeigniter",
		Name:        "CodeIgniter",
		Description: "MVC web framework for PHP",
		SourceURL:   "https://github.com/bcit-ci/CodeIgniter4",
		SourceDirs:  []string{"system/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewCakePHPFactory creates a Source for the CakePHP framework.
func NewCakePHPFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/cakephp",
		Name:        "CakePHP",
		Description: "Rapid development framework for PHP",
		SourceURL:   "https://github.com/cakephp/cakephp",
		SourceDirs:  []string{"src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewYiiFactory creates a Source for the Yii 2 framework.
func NewYiiFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/yii",
		Name:        "Yii 2",
		Description: "High-performance framework for PHP",
		SourceURL:   "https://github.com/yiisoft/yii2",
		SourceDirs:  []string{"framework/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewSlimFactory creates a Source for the Slim framework.
func NewSlimFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/slim",
		Name:        "Slim",
		Description: "Micro HTTP framework for PHP",
		SourceURL:   "https://github.com/slimphp/Slim",
		SourceDirs:  []string{"Slim/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewLaminasFactory creates a Source for the Laminas MVC framework.
func NewLaminasFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/laminas",
		Name:        "Laminas MVC",
		Description: "Enterprise MVC framework for PHP",
		SourceURL:   "https://github.com/laminas/laminas-mvc",
		SourceDirs:  []string{"src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewPhalconFactory creates a Source for the Phalcon framework (IDE stubs).
func NewPhalconFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/phalcon",
		Name:        "Phalcon",
		Description: "C-extension framework for PHP",
		SourceURL:   "https://github.com/phalcon/ide-stubs",
		SourceDirs:  []string{"src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewLivewireFactory creates a Source for the Livewire framework.
func NewLivewireFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/livewire",
		Name:        "Livewire",
		Description: "Full-stack reactive framework for PHP",
		SourceURL:   "https://github.com/livewire/livewire",
		SourceDirs:  []string{"src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewHyperfFactory creates a Source for the Hyperf framework.
func NewHyperfFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/swoole-framework",
		Name:        "Hyperf",
		Description: "Swoole-based framework for PHP",
		SourceURL:   "https://github.com/hyperf/hyperf",
		SourceDirs:  []string{"src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}
