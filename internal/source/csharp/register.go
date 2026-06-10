package csharp

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("csharp/dotnet-bcl", NewDotNetBCLFactory)
	registry.Default.Register("csharp/aspnetcore", NewASPNetCoreFactory)
	registry.Default.Register("csharp/ef-core", NewEFCoreFactory)
	registry.Default.Register("csharp/newtonsoft-json", NewNewtonsoftJsonFactory)
	registry.Default.Register("csharp/xunit", NewXUnitFactory)
	registry.Default.Register("csharp/nunit", NewNUnitFactory)
	registry.Default.Register("csharp/automapper", NewAutoMapperFactory)
	registry.Default.Register("csharp/mediatr", NewMediatRFactory)
	registry.Default.Register("csharp/serilog", NewSerilogFactory)
	registry.Default.Register("csharp/dapper", NewDapperFactory)
	registry.Default.Register("csharp/fluent-validation", NewFluentValidationFactory)
	registry.Default.Register("csharp/polly", NewPollyFactory)
	registry.Default.Register("csharp/maui", NewMAUIFactory)
	registry.Default.Register("csharp/blazor", NewBlazorFactory)
	registry.Default.Register("csharp/orleans", NewOrleansFactory)
	registry.Default.Register("csharp/avalonia", NewAvaloniaFactory)
	registry.Default.Register("csharp/abp", NewABPFactory)
	registry.Default.Register("csharp/servicestack", NewServiceStackFactory)
	registry.Default.Register("csharp/masstransit", NewMassTransitFactory)
	registry.Default.Register("csharp/wolverine", NewWolverineFactory)
	registry.Default.Register("csharp/carter", NewCarterFactory)
}

// NewDotNetBCLFactory creates a Source for the .NET Base Class Library.
func NewDotNetBCLFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/dotnet-bcl",
		Name:        ".NET Base Class Library",
		Description: "Core class library for the .NET platform",
		SourceURL:   "https://github.com/dotnet/runtime",
		SourceDirs:  []string{"src/libraries"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewASPNetCoreFactory creates a Source for ASP.NET Core.
func NewASPNetCoreFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/aspnetcore",
		Name:        "ASP.NET Core",
		Description: "Cross-platform framework for building web APIs and applications",
		SourceURL:   "https://github.com/dotnet/aspnetcore",
		SourceDirs:  []string{"src/Http", "src/Mvc", "src/Hosting"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewEFCoreFactory creates a Source for Entity Framework Core.
func NewEFCoreFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/ef-core",
		Name:        "Entity Framework Core",
		Description: "Modern object-database mapper for .NET supporting LINQ queries",
		SourceURL:   "https://github.com/dotnet/efcore",
		SourceDirs:  []string{"src/EFCore", "src/EFCore.Relational"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewNewtonsoftJsonFactory creates a Source for Newtonsoft.Json.
func NewNewtonsoftJsonFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/newtonsoft-json",
		Name:        "Newtonsoft.Json",
		Description: "High-performance JSON framework for .NET",
		SourceURL:   "https://github.com/JamesNK/Newtonsoft.Json",
		SourceDirs:  []string{"Src/Newtonsoft.Json"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewXUnitFactory creates a Source for xUnit.net.
func NewXUnitFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/xunit",
		Name:        "xUnit.net",
		Description: "Free, open-source, community-focused unit testing tool for .NET",
		SourceURL:   "https://github.com/xunit/xunit",
		// xUnit v3 renamed src/xunit.core to src/xunit.v3.core; both
		// layouts are listed and missing roots are skipped.
		SourceDirs: []string{
			"src/xunit.v3.core", "src/xunit.v3.assert",
			"src/xunit.core", "src/xunit.assert",
		},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewNUnitFactory creates a Source for NUnit.
func NewNUnitFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/nunit",
		Name:        "NUnit",
		Description: "Unit-testing framework for all .NET languages",
		SourceURL:   "https://github.com/nunit/nunit",
		SourceDirs:  []string{"src/NUnitFramework/framework"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewAutoMapperFactory creates a Source for AutoMapper.
func NewAutoMapperFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/automapper",
		Name:        "AutoMapper",
		Description: "Convention-based object-object mapper for .NET",
		SourceURL:   "https://github.com/AutoMapper/AutoMapper",
		SourceDirs:  []string{"src/AutoMapper"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewMediatRFactory creates a Source for MediatR.
func NewMediatRFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/mediatr",
		Name:        "MediatR",
		Description: "Simple mediator implementation in .NET for in-process messaging",
		SourceURL:   "https://github.com/jbogard/MediatR",
		SourceDirs:  []string{"src/MediatR"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewSerilogFactory creates a Source for Serilog.
func NewSerilogFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/serilog",
		Name:        "Serilog",
		Description: "Structured logging library for .NET with rich event data",
		SourceURL:   "https://github.com/serilog/serilog",
		SourceDirs:  []string{"src/Serilog"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewDapperFactory creates a Source for Dapper.
func NewDapperFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/dapper",
		Name:        "Dapper",
		Description: "Simple object mapper for .NET with high performance micro-ORM",
		SourceURL:   "https://github.com/DapperLib/Dapper",
		SourceDirs:  []string{"Dapper"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewFluentValidationFactory creates a Source for FluentValidation.
func NewFluentValidationFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/fluent-validation",
		Name:        "FluentValidation",
		Description: "Validation library for .NET with fluent interface and lambda expressions",
		SourceURL:   "https://github.com/FluentValidation/FluentValidation",
		SourceDirs:  []string{"src/FluentValidation"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewPollyFactory creates a Source for Polly.
func NewPollyFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/polly",
		Name:        "Polly",
		Description: "Resilience and transient-fault-handling library for .NET",
		SourceURL:   "https://github.com/App-vNext/Polly",
		SourceDirs:  []string{"src/Polly"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewMAUIFactory creates a Source for .NET MAUI.
func NewMAUIFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/maui",
		Name:        ".NET MAUI",
		Description: "Cross-platform app framework for .NET",
		SourceURL:   "https://github.com/dotnet/maui",
		SourceDirs:  []string{"src/Core/src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewBlazorFactory creates a Source for Blazor.
func NewBlazorFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/blazor",
		Name:        "Blazor",
		Description: "WebAssembly UI framework for .NET",
		SourceURL:   "https://github.com/dotnet/aspnetcore",
		SourceDirs:  []string{"src/Components/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewOrleansFactory creates a Source for Orleans.
func NewOrleansFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/orleans",
		Name:        "Orleans",
		Description: "Distributed actor framework for .NET",
		SourceURL:   "https://github.com/dotnet/orleans",
		SourceDirs:  []string{"src/Orleans.Core/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewAvaloniaFactory creates a Source for Avalonia UI.
func NewAvaloniaFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/avalonia",
		Name:        "Avalonia",
		Description: "UI framework for .NET",
		SourceURL:   "https://github.com/AvaloniaUI/Avalonia",
		SourceDirs:  []string{"src/Avalonia.Controls/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewABPFactory creates a Source for the ABP Framework.
func NewABPFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/abp",
		Name:        "ABP Framework",
		Description: "Application framework for .NET",
		SourceURL:   "https://github.com/abpframework/abp",
		SourceDirs:  []string{"framework/src/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewServiceStackFactory creates a Source for ServiceStack.
func NewServiceStackFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/servicestack",
		Name:        "ServiceStack",
		Description: "Web services framework for .NET",
		SourceURL:   "https://github.com/ServiceStack/ServiceStack",
		// The repo nests the framework inside a ServiceStack/ top-level dir.
		SourceDirs:  []string{"ServiceStack/src/ServiceStack"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewMassTransitFactory creates a Source for MassTransit.
func NewMassTransitFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/masstransit",
		Name:        "MassTransit",
		Description: "Messaging framework for .NET",
		SourceURL:   "https://github.com/MassTransit/MassTransit",
		SourceDirs:  []string{"src/MassTransit/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewWolverineFactory creates a Source for Wolverine.
func NewWolverineFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/wolverine",
		Name:        "Wolverine",
		Description: "Messaging framework for .NET",
		SourceURL:   "https://github.com/JasperFx/wolverine",
		SourceDirs:  []string{"src/Wolverine/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewCarterFactory creates a Source for Carter.
func NewCarterFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "csharp/carter",
		Name:        "Carter",
		Description: "Minimal API framework for .NET",
		SourceURL:   "https://github.com/CarterCommunity/Carter",
		SourceDirs:  []string{"src/Carter/"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}
