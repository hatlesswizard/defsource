package cpp

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("cpp/libstdcpp", NewLibstdcppFactory)
	registry.Default.Register("cpp/asio", NewAsioFactory)
	registry.Default.Register("cpp/qt", NewQtFactory)
	registry.Default.Register("cpp/abseil", NewAbseilFactory)
	registry.Default.Register("cpp/googletest", NewGoogleTestFactory)
	registry.Default.Register("cpp/grpc", NewGRPCFactory)
	registry.Default.Register("cpp/opencv", NewOpenCVFactory)
	registry.Default.Register("cpp/poco", NewPocoFactory)
	registry.Default.Register("cpp/fmt", NewFmtFactory)
	registry.Default.Register("cpp/spdlog", NewSpdlogFactory)
	registry.Default.Register("cpp/nlohmann-json", NewNlohmannJsonFactory)
	registry.Default.Register("cpp/catch2", NewCatch2Factory)
	registry.Default.Register("cpp/eigen", NewEigenFactory)
	registry.Default.Register("cpp/wxwidgets", NewWxWidgetsFactory)
	registry.Default.Register("cpp/drogon", NewDrogonFactory)
	registry.Default.Register("cpp/crow", NewCrowFactory)
	registry.Default.Register("cpp/oatpp", NewOatppFactory)
	registry.Default.Register("cpp/juce", NewJUCEFactory)
	registry.Default.Register("cpp/wt", NewWtFactory)
	registry.Default.Register("cpp/cinder", NewCinderFactory)
	registry.Default.Register("cpp/pistache", NewPistacheFactory)
	registry.Default.Register("cpp/cppcms", NewCppCMSFactory)
	registry.Default.Register("cpp/treefrog", NewTreeFrogFactory)
}

// NewLibstdcppFactory creates a Source for the GNU libstdc++ standard library.
func NewLibstdcppFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/libstdcpp",
		Name:        "libstdc++",
		Description: "GNU C++ Standard Library implementation",
		SourceURL:   "https://github.com/gcc-mirror/gcc",
		Version:     opts.Ref,
		IncludeDirs: []string{"libstdc++-v3/include"},
		SkipDirs:    []string{"testsuite"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewAsioFactory creates a Source for the Boost.Asio networking library.
// Replaces the boostorg/boost super-project, which is an empty shell of
// git submodules and contains no crawlable headers.
func NewAsioFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/asio",
		Name:        "Asio",
		Description: "Cross-platform C++ library for network and low-level I/O programming (Boost.Asio)",
		SourceURL:   "https://github.com/boostorg/asio",
		Version:     opts.Ref,
		IncludeDirs: []string{"include/boost/asio"},
		SkipDirs:    []string{"test", "example", "doc"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewQtFactory creates a Source for the Qt framework.
func NewQtFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/qt",
		Name:        "Qt",
		Description: "Cross-platform application development framework for C++",
		SourceURL:   "https://github.com/qt/qtbase",
		Version:     opts.Ref,
		IncludeDirs: []string{"src/corelib", "src/gui", "src/widgets", "src/network"},
		SkipDirs:    []string{"tests", "examples"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewAbseilFactory creates a Source for the Abseil C++ library.
func NewAbseilFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/abseil",
		Name:        "Abseil",
		Description: "Collection of C++ libraries drawn from Google internal codebase",
		SourceURL:   "https://github.com/abseil/abseil-cpp",
		Version:     opts.Ref,
		IncludeDirs: []string{"absl"},
		SkipDirs:    []string{"testdata"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewGoogleTestFactory creates a Source for Google Test.
func NewGoogleTestFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/googletest",
		Name:        "Google Test",
		Description: "Google Testing and Mocking Framework for C++",
		SourceURL:   "https://github.com/google/googletest",
		Version:     opts.Ref,
		IncludeDirs: []string{"googletest/include", "googlemock/include"},
		SkipDirs:    []string{"test", "samples"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewGRPCFactory creates a Source for the gRPC C++ library.
func NewGRPCFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/grpc",
		Name:        "gRPC",
		Description: "High-performance, open-source universal RPC framework",
		SourceURL:   "https://github.com/grpc/grpc",
		Version:     opts.Ref,
		IncludeDirs: []string{"include/grpc", "include/grpcpp"},
		SkipDirs:    []string{"test", "examples"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewOpenCVFactory creates a Source for the OpenCV computer vision library.
func NewOpenCVFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/opencv",
		Name:        "OpenCV",
		Description: "Open-source computer vision and machine learning software library",
		SourceURL:   "https://github.com/opencv/opencv",
		Version:     opts.Ref,
		IncludeDirs: []string{"modules/core/include", "modules/imgproc/include", "modules/highgui/include"},
		SkipDirs:    []string{"test", "samples", "doc"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewPocoFactory creates a Source for the Poco C++ Libraries.
func NewPocoFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/poco",
		Name:        "Poco",
		Description: "Cross-platform C++ libraries for building network and internet applications",
		SourceURL:   "https://github.com/pocoproject/poco",
		Version:     opts.Ref,
		IncludeDirs: []string{"Foundation/include", "Net/include", "Util/include"},
		SkipDirs:    []string{"testsuite", "samples"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewFmtFactory creates a Source for the fmt formatting library.
func NewFmtFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/fmt",
		Name:        "fmt",
		Description: "Modern formatting library for C++ with safe and fast alternatives to printf",
		SourceURL:   "https://github.com/fmtlib/fmt",
		Version:     opts.Ref,
		IncludeDirs: []string{"include/fmt"},
		SkipDirs:    []string{"test"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewSpdlogFactory creates a Source for the spdlog logging library.
func NewSpdlogFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/spdlog",
		Name:        "spdlog",
		Description: "Fast C++ logging library with header-only and compiled modes",
		SourceURL:   "https://github.com/gabime/spdlog",
		Version:     opts.Ref,
		IncludeDirs: []string{"include/spdlog"},
		SkipDirs:    []string{"tests", "bench"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewNlohmannJsonFactory creates a Source for the nlohmann/json library.
func NewNlohmannJsonFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/nlohmann-json",
		Name:        "nlohmann/json",
		Description: "JSON for Modern C++ with intuitive syntax",
		SourceURL:   "https://github.com/nlohmann/json",
		Version:     opts.Ref,
		IncludeDirs: []string{"include/nlohmann"},
		SkipDirs:    []string{"tests", "benchmarks"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewCatch2Factory creates a Source for the Catch2 testing framework.
func NewCatch2Factory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/catch2",
		Name:        "Catch2",
		Description: "Modern C++ test framework for unit tests and BDD",
		SourceURL:   "https://github.com/catchorg/Catch2",
		Version:     opts.Ref,
		IncludeDirs: []string{"src/catch2"},
		SkipDirs:    []string{"tests", "examples"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewEigenFactory creates a Source for the Eigen linear algebra library.
func NewEigenFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/eigen",
		Name:        "Eigen",
		Description: "C++ template library for linear algebra, matrices, and numerical solvers",
		SourceURL:   "https://github.com/eigenteam/eigen-git-mirror",
		Version:     opts.Ref,
		IncludeDirs: []string{"Eigen"},
		SkipDirs:    []string{"test", "bench", "doc"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewWxWidgetsFactory creates a Source for the wxWidgets GUI framework.
func NewWxWidgetsFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/wxwidgets",
		Name:        "wxWidgets",
		Description: "GUI framework for C++",
		SourceURL:   "https://github.com/wxWidgets/wxWidgets",
		Version:     opts.Ref,
		IncludeDirs: []string{"include/wx/"},
		SkipDirs:    []string{"tests", "samples"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewDrogonFactory creates a Source for the Drogon async web framework.
func NewDrogonFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/drogon",
		Name:        "Drogon",
		Description: "Async web framework for C++",
		SourceURL:   "https://github.com/drogonframework/drogon",
		Version:     opts.Ref,
		IncludeDirs: []string{"lib/inc/drogon/"},
		SkipDirs:    []string{"tests", "examples"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewCrowFactory creates a Source for the Crow micro web framework.
func NewCrowFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/crow",
		Name:        "Crow",
		Description: "Micro web framework for C++",
		SourceURL:   "https://github.com/CrowCpp/Crow",
		Version:     opts.Ref,
		IncludeDirs: []string{"include/crow/"},
		SkipDirs:    []string{"tests", "examples"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewOatppFactory creates a Source for the Oat++ web framework.
func NewOatppFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/oatpp",
		Name:        "Oat++",
		Description: "Web framework for C++",
		SourceURL:   "https://github.com/oatpp/oatpp",
		Version:     opts.Ref,
		IncludeDirs: []string{"src/oatpp/"},
		SkipDirs:    []string{"test"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewJUCEFactory creates a Source for the JUCE audio/app framework.
func NewJUCEFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/juce",
		Name:        "JUCE",
		Description: "Audio/app framework for C++",
		SourceURL:   "https://github.com/juce-framework/JUCE",
		Version:     opts.Ref,
		IncludeDirs: []string{"modules/"},
		SkipDirs:    []string{"tests", "examples"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewWtFactory creates a Source for the Wt web toolkit framework.
func NewWtFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/wt",
		Name:        "Wt",
		Description: "Web toolkit framework for C++",
		SourceURL:   "https://github.com/emweb/wt",
		Version:     opts.Ref,
		IncludeDirs: []string{"src/Wt/"},
		SkipDirs:    []string{"test", "examples"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewCinderFactory creates a Source for the Cinder creative framework.
func NewCinderFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/cinder",
		Name:        "Cinder",
		Description: "Creative framework for C++",
		SourceURL:   "https://github.com/cinder/Cinder",
		Version:     opts.Ref,
		IncludeDirs: []string{"include/cinder/"},
		SkipDirs:    []string{"test", "samples"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewPistacheFactory creates a Source for the Pistache REST framework.
func NewPistacheFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/pistache",
		Name:        "Pistache",
		Description: "REST framework for C++",
		SourceURL:   "https://github.com/pistacheio/pistache",
		Version:     opts.Ref,
		IncludeDirs: []string{"include/pistache/"},
		SkipDirs:    []string{"tests"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewCppCMSFactory creates a Source for the CppCMS web framework.
func NewCppCMSFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/cppcms",
		Name:        "CppCMS",
		Description: "Web framework for C++",
		SourceURL:   "https://github.com/artyom-beilis/cppcms",
		Version:     opts.Ref,
		IncludeDirs: []string{"cppcms/"},
		SkipDirs:    []string{"tests"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}

// NewTreeFrogFactory creates a Source for the TreeFrog MVC web framework.
func NewTreeFrogFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		RepoPath:    repoPath,
		LibraryID:   "cpp/treefrog",
		Name:        "TreeFrog",
		Description: "MVC web framework for C++",
		SourceURL:   "https://github.com/treefrogframework/treefrog-framework",
		Version:     opts.Ref,
		IncludeDirs: []string{"src/"},
		SkipDirs:    []string{"test"},
		Ref:         opts.Ref,
	}
	return New(cfg), nil
}
