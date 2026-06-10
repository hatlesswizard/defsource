package php

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("php/swoole", NewSwooleFactory)
	registry.Default.Register("php/workerman", NewWorkermanFactory)
	registry.Default.Register("php/flight", NewFlightFactory)
}

// NewSwooleFactory creates a Source for the Swoole coroutine-based async framework (PHP library).
func NewSwooleFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/swoole",
		Name:        "Swoole",
		Description: "Coroutine-based async framework for PHP",
		SourceURL:   "https://github.com/swoole/library",
		SourceDirs:  []string{"src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewWorkermanFactory creates a Source for the Workerman async event-driven socket framework.
func NewWorkermanFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/workerman",
		Name:        "Workerman",
		Description: "Async event-driven socket framework for PHP",
		SourceURL:   "https://github.com/walkor/workerman",
		SourceDirs:  []string{"src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}

// NewFlightFactory creates a Source for the Flight extensible micro-framework.
func NewFlightFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := PHPConfig{
		LibraryID:   "php/flight",
		Name:        "Flight",
		Description: "Extensible micro-framework for PHP",
		SourceURL:   "https://github.com/flightphp/core",
		SourceDirs:  []string{"flight/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	sopts = append(sopts, WithConfig(cfg))
	return New(repoPath, sopts...), nil
}
