package clang

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("c/glibc", NewGlibcFactory)
	registry.Default.Register("c/sqlite", NewSQLiteFactory)
	registry.Default.Register("c/openssl", NewOpenSSLFactory)
	registry.Default.Register("c/libcurl", NewLibcurlFactory)
	registry.Default.Register("c/zlib", NewZlibFactory)
	registry.Default.Register("c/libuv", NewLibuvFactory)
	registry.Default.Register("c/jansson", NewJanssonFactory)
	registry.Default.Register("c/glib", NewGLibFactory)
	registry.Default.Register("c/sdl2", NewSDL2Factory)
	registry.Default.Register("c/libpng", NewLibpngFactory)
	registry.Default.Register("c/check", NewCheckFactory)
	registry.Default.Register("c/linux-uapi", NewLinuxUAPIFactory)
	registry.Default.Register("c/gtk", NewGTKFactory)
	registry.Default.Register("c/freertos", NewFreeRTOSFactory)
	registry.Default.Register("c/esp-idf", NewESPIDFFactory)
	registry.Default.Register("c/zephyr", NewZephyrFactory)
	registry.Default.Register("c/dpdk", NewDPDKFactory)
	registry.Default.Register("c/mongoose-web", NewMongooseFactory)
	registry.Default.Register("c/kore", NewKoreFactory)
	registry.Default.Register("c/facilio", NewFacilioFactory)
	registry.Default.Register("c/h2o", NewH2OFactory)
}

// NewGlibcFactory creates a Source for the GNU C Library.
func NewGlibcFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/glibc",
		Name:        "glibc",
		Description: "GNU C Library providing core system call wrappers and standard C functions",
		SourceURL:   "https://github.com/bminor/glibc",
		TrustScore:  0.95,
		HeaderDirs:  []string{"include", "stdlib", "string", "stdio-common"},
		ExcludePatterns: []string{"test", "benchtests"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewSQLiteFactory creates a Source for the SQLite library.
func NewSQLiteFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/sqlite",
		Name:        "SQLite",
		Description: "Self-contained, serverless, zero-configuration SQL database engine",
		SourceURL:   "https://github.com/sqlite/sqlite",
		TrustScore:  0.95,
		HeaderDirs:  []string{"src"},
		ExcludePatterns: []string{"test", "ext"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewOpenSSLFactory creates a Source for the OpenSSL library.
func NewOpenSSLFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/openssl",
		Name:        "OpenSSL",
		Description: "Full-featured toolkit for TLS/SSL protocols and general-purpose cryptography",
		SourceURL:   "https://github.com/openssl/openssl",
		TrustScore:  0.92,
		HeaderDirs:  []string{"include/openssl"},
		ExcludePatterns: []string{"test", "fuzz"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewLibcurlFactory creates a Source for the libcurl library.
func NewLibcurlFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/libcurl",
		Name:        "libcurl",
		Description: "Multiprotocol file transfer library supporting HTTP, FTP, and more",
		SourceURL:   "https://github.com/curl/curl",
		TrustScore:  0.92,
		HeaderDirs:  []string{"include/curl"},
		ExcludePatterns: []string{"tests"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewZlibFactory creates a Source for the zlib compression library.
func NewZlibFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/zlib",
		Name:        "zlib",
		Description: "General-purpose lossless data compression library",
		SourceURL:   "https://github.com/madler/zlib",
		TrustScore:  0.92,
		HeaderDirs:  []string{""},
		ExcludePatterns: []string{"test", "examples"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewLibuvFactory creates a Source for the libuv async I/O library.
func NewLibuvFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/libuv",
		Name:        "libuv",
		Description: "Cross-platform asynchronous I/O library",
		SourceURL:   "https://github.com/libuv/libuv",
		TrustScore:  0.90,
		HeaderDirs:  []string{"include"},
		ExcludePatterns: []string{"test", "docs"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewJanssonFactory creates a Source for the Jansson JSON library.
func NewJanssonFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/jansson",
		Name:        "Jansson",
		Description: "C library for encoding, decoding, and manipulating JSON data",
		SourceURL:   "https://github.com/akheron/jansson",
		TrustScore:  0.88,
		HeaderDirs:  []string{"src"},
		ExcludePatterns: []string{"test"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewGLibFactory creates a Source for the GLib utility library.
func NewGLibFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/glib",
		Name:        "GLib",
		Description: "Low-level core library for GNOME providing data structures and utilities",
		SourceURL:   "https://github.com/GNOME/glib",
		TrustScore:  0.90,
		HeaderDirs:  []string{"glib", "gio"},
		ExcludePatterns: []string{"tests", "fuzzing"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewSDL2Factory creates a Source for the SDL2 multimedia library.
func NewSDL2Factory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/sdl2",
		Name:        "SDL2",
		Description: "Cross-platform multimedia library for audio, video, and input",
		SourceURL:   "https://github.com/libsdl-org/SDL",
		TrustScore:  0.90,
		HeaderDirs:  []string{"include"},
		ExcludePatterns: []string{"test"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewLibpngFactory creates a Source for the libpng library.
func NewLibpngFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/libpng",
		Name:        "libpng",
		Description: "Reference library for reading and writing PNG image files",
		SourceURL:   "https://github.com/pnggroup/libpng",
		TrustScore:  0.88,
		HeaderDirs:  []string{""},
		ExcludePatterns: []string{"tests", "contrib"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewCheckFactory creates a Source for the Check unit testing framework.
func NewCheckFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/check",
		Name:        "Check",
		Description: "Unit testing framework for C",
		SourceURL:   "https://github.com/libcheck/check",
		TrustScore:  0.85,
		HeaderDirs:  []string{"src"},
		ExcludePatterns: []string{"tests"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewLinuxUAPIFactory creates a Source for the Linux userspace API headers.
func NewLinuxUAPIFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/linux-uapi",
		Name:        "Linux UAPI",
		Description: "Linux kernel userspace API headers",
		SourceURL:   "https://github.com/torvalds/linux",
		TrustScore:  0.95,
		HeaderDirs:  []string{"include/uapi"},
		ExcludePatterns: []string{"tools", "Documentation"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewGTKFactory creates a Source for the GTK GUI framework.
func NewGTKFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/gtk",
		Name:        "GTK",
		Description: "GUI application framework",
		SourceURL:   "https://github.com/GNOME/gtk",
		TrustScore:  0.90,
		HeaderDirs:  []string{"gtk/"},
		ExcludePatterns: []string{"tests", "examples"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewFreeRTOSFactory creates a Source for the FreeRTOS framework.
func NewFreeRTOSFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/freertos",
		Name:        "FreeRTOS",
		Description: "RTOS framework",
		SourceURL:   "https://github.com/FreeRTOS/FreeRTOS-Kernel",
		TrustScore:  0.92,
		HeaderDirs:  []string{"include/"},
		ExcludePatterns: []string{"tests", "examples"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewESPIDFFactory creates a Source for the ESP-IDF IoT framework.
func NewESPIDFFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/esp-idf",
		Name:        "ESP-IDF",
		Description: "IoT framework",
		SourceURL:   "https://github.com/espressif/esp-idf",
		TrustScore:  0.90,
		HeaderDirs:  []string{"components/"},
		ExcludePatterns: []string{"tests", "examples"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewZephyrFactory creates a Source for the Zephyr RTOS framework.
func NewZephyrFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/zephyr",
		Name:        "Zephyr",
		Description: "RTOS framework",
		SourceURL:   "https://github.com/zephyrproject-rtos/zephyr",
		TrustScore:  0.90,
		HeaderDirs:  []string{"include/zephyr/"},
		ExcludePatterns: []string{"tests", "samples"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewDPDKFactory creates a Source for the DPDK networking framework.
func NewDPDKFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/dpdk",
		Name:        "DPDK",
		Description: "Networking framework",
		SourceURL:   "https://github.com/DPDK/dpdk",
		TrustScore:  0.88,
		HeaderDirs:  []string{"lib/"},
		ExcludePatterns: []string{"test", "examples"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewMongooseFactory creates a Source for the Mongoose embedded web framework.
func NewMongooseFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/mongoose-web",
		Name:        "Mongoose",
		Description: "Embedded web framework",
		SourceURL:   "https://github.com/cesanta/mongoose",
		TrustScore:  0.85,
		HeaderDirs:  []string{""},
		ExcludePatterns: []string{"test", "examples"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewKoreFactory creates a Source for the Kore web application framework.
func NewKoreFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/kore",
		Name:        "Kore",
		Description: "Web application framework",
		SourceURL:   "https://github.com/jorisvink/kore",
		TrustScore:  0.85,
		HeaderDirs:  []string{"include/kore"},
		ExcludePatterns: []string{"test", "examples"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewFacilioFactory creates a Source for the facil.io web framework.
func NewFacilioFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/facilio",
		Name:        "facil.io",
		Description: "Web framework for C",
		SourceURL:   "https://github.com/boazsegev/facil.io",
		TrustScore:  0.85,
		HeaderDirs:  []string{"lib/facil/"},
		ExcludePatterns: []string{"tests", "examples"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewH2OFactory creates a Source for the H2O HTTP/2 web server framework.
func NewH2OFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		LibraryID:   "c/h2o",
		Name:        "H2O",
		Description: "HTTP/2 web server framework",
		SourceURL:   "https://github.com/h2o/h2o",
		TrustScore:  0.85,
		HeaderDirs:  []string{"include/"},
		ExcludePatterns: []string{"t", "examples"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}
