package ruby

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("ruby/stdlib", NewStdlibFactory)
	registry.Default.Register("ruby/rails", NewRailsFactory)
	registry.Default.Register("ruby/rspec", NewRSpecFactory)
	registry.Default.Register("ruby/devise", NewDeviseFactory)
	registry.Default.Register("ruby/sidekiq", NewSidekiqFactory)
	registry.Default.Register("ruby/sinatra", NewSinatraFactory)
	registry.Default.Register("ruby/rack", NewRackFactory)
	registry.Default.Register("ruby/minitest", NewMinitestFactory)
	registry.Default.Register("ruby/nokogiri", NewNokogiriFactory)
	registry.Default.Register("ruby/puma", NewPumaFactory)
	registry.Default.Register("ruby/faraday", NewFaradayFactory)
	registry.Default.Register("ruby/dry-rb", NewDryRbFactory)
	registry.Default.Register("ruby/hanami", NewHanamiFactory)
	registry.Default.Register("ruby/grape", NewGrapeFactory)
	registry.Default.Register("ruby/roda", NewRodaFactory)
	registry.Default.Register("ruby/padrino", NewPadrinoFactory)
	registry.Default.Register("ruby/cuba", NewCubaFactory)
	registry.Default.Register("ruby/dry-system", NewDrySystemFactory)
	registry.Default.Register("ruby/camping", NewCampingFactory)
	registry.Default.Register("ruby/sequel", NewSequelFactory)
	registry.Default.Register("ruby/ramaze", NewRamazeFactory)
}

// NewStdlibFactory creates a Source for the Ruby standard library.
func NewStdlibFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/stdlib",
		Name:        "Ruby Standard Library",
		Description: "Complete reference for the Ruby standard library",
		SourceURL:   "https://github.com/ruby/ruby",
		SourceRoots: []string{"lib"},
		TrustScore:  0.95,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewRailsFactory creates a Source for Ruby on Rails.
func NewRailsFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/rails",
		Name:        "Ruby on Rails",
		Description: "Full-stack web application framework for Ruby",
		SourceURL:   "https://github.com/rails/rails",
		SourceRoots: []string{"activerecord/lib", "actionpack/lib", "activesupport/lib", "activemodel/lib", "actionview/lib"},
		TrustScore:  0.92,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewRSpecFactory creates a Source for the RSpec testing framework.
func NewRSpecFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/rspec",
		Name:        "RSpec",
		Description: "Behaviour Driven Development framework for Ruby",
		SourceURL:   "https://github.com/rspec/rspec-core",
		SourceRoots: []string{"lib"},
		TrustScore:  0.90,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewDeviseFactory creates a Source for Devise authentication.
func NewDeviseFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/devise",
		Name:        "Devise",
		Description: "Flexible authentication solution for Rails based on Warden",
		SourceURL:   "https://github.com/heartcombo/devise",
		SourceRoots: []string{"lib", "app"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewSidekiqFactory creates a Source for Sidekiq.
func NewSidekiqFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/sidekiq",
		Name:        "Sidekiq",
		Description: "Efficient background processing framework for Ruby",
		SourceURL:   "https://github.com/sidekiq/sidekiq",
		SourceRoots: []string{"lib"},
		TrustScore:  0.90,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewSinatraFactory creates a Source for Sinatra.
func NewSinatraFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/sinatra",
		Name:        "Sinatra",
		Description: "DSL for quickly creating web applications in Ruby",
		SourceURL:   "https://github.com/sinatra/sinatra",
		SourceRoots: []string{"lib"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewRackFactory creates a Source for Rack.
func NewRackFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/rack",
		Name:        "Rack",
		Description: "Modular Ruby web server interface",
		SourceURL:   "https://github.com/rack/rack",
		SourceRoots: []string{"lib"},
		TrustScore:  0.90,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewMinitestFactory creates a Source for Minitest.
func NewMinitestFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/minitest",
		Name:        "Minitest",
		Description: "Complete suite of testing facilities for Ruby",
		SourceURL:   "https://github.com/minitest/minitest",
		SourceRoots: []string{"lib"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewNokogiriFactory creates a Source for Nokogiri.
func NewNokogiriFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/nokogiri",
		Name:        "Nokogiri",
		Description: "HTML, XML, SAX, and Reader parser with XPath and CSS selector support",
		SourceURL:   "https://github.com/sparklemotion/nokogiri",
		SourceRoots: []string{"lib"},
		TrustScore:  0.90,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewPumaFactory creates a Source for Puma.
func NewPumaFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/puma",
		Name:        "Puma",
		Description: "Concurrent HTTP server for Ruby and Rack applications",
		SourceURL:   "https://github.com/puma/puma",
		SourceRoots: []string{"lib"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewFaradayFactory creates a Source for Faraday.
func NewFaradayFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/faraday",
		Name:        "Faraday",
		Description: "Simple and flexible HTTP client library with middleware architecture",
		SourceURL:   "https://github.com/lostisland/faraday",
		SourceRoots: []string{"lib"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewDryRbFactory creates a Source for Dry-rb.
func NewDryRbFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/dry-rb",
		Name:        "Dry-rb",
		Description: "Collection of next-generation Ruby libraries for type safety and validation",
		SourceURL:   "https://github.com/dry-rb/dry-types",
		SourceRoots: []string{"lib"},
		TrustScore:  0.85,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewHanamiFactory creates a Source for the Hanami web framework.
func NewHanamiFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/hanami",
		Name:        "Hanami",
		Description: "Modern web framework for Ruby",
		SourceURL:   "https://github.com/hanami/hanami",
		SourceRoots: []string{"lib"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewGrapeFactory creates a Source for the Grape REST API framework.
func NewGrapeFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/grape",
		Name:        "Grape",
		Description: "REST API framework for Ruby",
		SourceURL:   "https://github.com/ruby-grape/grape",
		SourceRoots: []string{"lib"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewRodaFactory creates a Source for the Roda routing tree web framework.
func NewRodaFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/roda",
		Name:        "Roda",
		Description: "Routing tree web framework for Ruby",
		SourceURL:   "https://github.com/jeremyevans/roda",
		SourceRoots: []string{"lib"},
		TrustScore:  0.85,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewPadrinoFactory creates a Source for the Padrino full-stack framework.
func NewPadrinoFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/padrino",
		Name:        "Padrino",
		Description: "Full-stack framework for Ruby",
		SourceURL:   "https://github.com/padrino/padrino-framework",
		SourceRoots: []string{"padrino-core/lib"},
		TrustScore:  0.85,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewCubaFactory creates a Source for the Cuba micro framework.
func NewCubaFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/cuba",
		Name:        "Cuba",
		Description: "Micro framework for Ruby",
		SourceURL:   "https://github.com/soveran/cuba",
		SourceRoots: []string{"lib"},
		TrustScore:  0.85,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewDrySystemFactory creates a Source for the dry-system application framework.
func NewDrySystemFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/dry-system",
		Name:        "dry-system",
		Description: "Application framework for Ruby",
		SourceURL:   "https://github.com/dry-rb/dry-system",
		SourceRoots: []string{"lib"},
		TrustScore:  0.85,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewCampingFactory creates a Source for the Camping micro web framework.
func NewCampingFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/camping",
		Name:        "Camping",
		Description: "Micro web framework for Ruby",
		SourceURL:   "https://github.com/camping/camping",
		SourceRoots: []string{"lib"},
		TrustScore:  0.82,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewSequelFactory creates a Source for the Sequel database toolkit.
// Replaces Lucky, which is written in Crystal and cannot be indexed by
// the Ruby adapter.
func NewSequelFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/sequel",
		Name:        "Sequel",
		Description: "Database toolkit for Ruby",
		SourceURL:   "https://github.com/jeremyevans/sequel",
		SourceRoots: []string{"lib"},
		TrustScore:  0.88,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewRamazeFactory creates a Source for the Ramaze modular web framework.
func NewRamazeFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "ruby/ramaze",
		Name:        "Ramaze",
		Description: "Modular web framework for Ruby",
		SourceURL:   "https://github.com/Ramaze/ramaze",
		SourceRoots: []string{"lib"},
		TrustScore:  0.82,
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}
