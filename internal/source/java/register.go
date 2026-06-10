package java

import (
	"github.com/hatlesswizard/defsource/internal/source"
	"github.com/hatlesswizard/defsource/internal/source/registry"
)

func init() {
	registry.Default.Register("java/openjdk", NewOpenJDKFactory)
	registry.Default.Register("java/spring-framework", NewSpringFrameworkFactory)
	registry.Default.Register("java/spring-boot", NewSpringBootFactory)
	registry.Default.Register("java/hibernate", NewHibernateFactory)
	registry.Default.Register("java/junit5", NewJUnit5Factory)
	registry.Default.Register("java/guava", NewGuavaFactory)
	registry.Default.Register("java/jackson", NewJacksonFactory)
	registry.Default.Register("java/netty", NewNettyFactory)
	registry.Default.Register("java/mockito", NewMockitoFactory)
	registry.Default.Register("java/logback", NewLogbackFactory)
	registry.Default.Register("java/kafka", NewKafkaFactory)
	registry.Default.Register("java/commons-lang", NewCommonsLangFactory)
	registry.Default.Register("java/micronaut", NewMicronautFactory)
	registry.Default.Register("java/quarkus", NewQuarkusFactory)
	registry.Default.Register("java/vertx", NewVertxFactory)
	registry.Default.Register("java/dropwizard", NewDropwizardFactory)
	registry.Default.Register("java/play", NewPlayFactory)
	registry.Default.Register("java/struts", NewStrutsFactory)
	registry.Default.Register("java/grails", NewGrailsFactory)
	registry.Default.Register("java/javalin", NewJavalinFactory)
	registry.Default.Register("java/spark", NewSparkFactory)
	registry.Default.Register("java/ratpack", NewRatpackFactory)
}

// NewOpenJDKFactory creates a Source for the OpenJDK standard library.
func NewOpenJDKFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "openjdk",
		Repo:        "jdk",
		LibraryID:   "java/openjdk",
		Name:        "OpenJDK Standard Library",
		Description: "Complete reference for the Java standard library",
		SourceURL:   "https://github.com/openjdk/jdk",
		SourceRoots: []string{"src/java.base/share/classes"},
		ExcludePatterns: []string{"**/test/**", "**/internal/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewSpringFrameworkFactory creates a Source for the Spring Framework.
func NewSpringFrameworkFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "spring-projects",
		Repo:        "spring-framework",
		LibraryID:   "java/spring-framework",
		Name:        "Spring Framework",
		Description: "Core support for dependency injection, transaction management, and web applications",
		SourceURL:   "https://github.com/spring-projects/spring-framework",
		SourceRoots: []string{"spring-core/src/main/java", "spring-beans/src/main/java", "spring-context/src/main/java", "spring-web/src/main/java", "spring-webmvc/src/main/java"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewSpringBootFactory creates a Source for Spring Boot.
func NewSpringBootFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "spring-projects",
		Repo:        "spring-boot",
		LibraryID:   "java/spring-boot",
		Name:        "Spring Boot",
		Description: "Convention-over-configuration framework for production-ready Spring applications",
		SourceURL:   "https://github.com/spring-projects/spring-boot",
		// Spring Boot 4 moved modules from spring-boot-project/ to core/;
		// both layouts are listed and missing roots are skipped.
		SourceRoots: []string{
			"core/spring-boot/src/main/java",
			"core/spring-boot-autoconfigure/src/main/java",
			"spring-boot-project/spring-boot/src/main/java",
			"spring-boot-project/spring-boot-autoconfigure/src/main/java",
		},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewHibernateFactory creates a Source for Hibernate ORM.
func NewHibernateFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "hibernate",
		Repo:        "hibernate-orm",
		LibraryID:   "java/hibernate",
		Name:        "Hibernate ORM",
		Description: "Object/Relational Mapping framework for Java",
		SourceURL:   "https://github.com/hibernate/hibernate-orm",
		SourceRoots: []string{"hibernate-core/src/main/java"},
		ExcludePatterns: []string{"**/test/**", "**/internal/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewJUnit5Factory creates a Source for JUnit 5.
func NewJUnit5Factory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "junit-team",
		Repo:        "junit5",
		LibraryID:   "java/junit5",
		Name:        "JUnit 5",
		Description: "Next generation testing framework for Java",
		SourceURL:   "https://github.com/junit-team/junit5",
		SourceRoots: []string{"junit-jupiter-api/src/main/java", "junit-jupiter-engine/src/main/java"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewGuavaFactory creates a Source for Google Guava.
func NewGuavaFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "google",
		Repo:        "guava",
		LibraryID:   "java/guava",
		Name:        "Guava",
		Description: "Google core libraries for Java with collections, caching, and utilities",
		SourceURL:   "https://github.com/google/guava",
		SourceRoots: []string{"guava/src"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewJacksonFactory creates a Source for the Jackson JSON library.
func NewJacksonFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "FasterXML",
		Repo:        "jackson-databind",
		LibraryID:   "java/jackson",
		Name:        "Jackson",
		Description: "High-performance JSON processor for Java",
		SourceURL:   "https://github.com/FasterXML/jackson-databind",
		SourceRoots: []string{"src/main/java"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewNettyFactory creates a Source for Netty.
func NewNettyFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "netty",
		Repo:        "netty",
		LibraryID:   "java/netty",
		Name:        "Netty",
		Description: "Asynchronous event-driven network application framework",
		SourceURL:   "https://github.com/netty/netty",
		SourceRoots: []string{"transport/src/main/java", "common/src/main/java", "buffer/src/main/java", "handler/src/main/java"},
		ExcludePatterns: []string{"**/test/**", "**/internal/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewMockitoFactory creates a Source for Mockito.
func NewMockitoFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "mockito",
		Repo:        "mockito",
		LibraryID:   "java/mockito",
		Name:        "Mockito",
		Description: "Tasty mocking framework for unit tests in Java",
		SourceURL:   "https://github.com/mockito/mockito",
		// v5.x is a Gradle multi-module build (mockito-core); old tags
		// kept sources at the repo root. Missing roots are skipped.
		SourceRoots: []string{"mockito-core/src/main/java", "src"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewLogbackFactory creates a Source for Logback.
func NewLogbackFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "qos-ch",
		Repo:        "logback",
		LibraryID:   "java/logback",
		Name:        "Logback",
		Description: "Reliable, generic, fast and flexible logging framework for Java",
		SourceURL:   "https://github.com/qos-ch/logback",
		SourceRoots: []string{"logback-core/src/main/java", "logback-classic/src/main/java"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewKafkaFactory creates a Source for Apache Kafka.
func NewKafkaFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "apache",
		Repo:        "kafka",
		LibraryID:   "java/kafka",
		Name:        "Apache Kafka",
		Description: "Distributed event streaming platform for high-performance data pipelines",
		SourceURL:   "https://github.com/apache/kafka",
		SourceRoots: []string{"clients/src/main/java"},
		ExcludePatterns: []string{"**/test/**", "**/internal/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewCommonsLangFactory creates a Source for Apache Commons Lang.
func NewCommonsLangFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "apache",
		Repo:        "commons-lang",
		LibraryID:   "java/commons-lang",
		Name:        "Apache Commons Lang",
		Description: "Helper utilities for the java.lang API with string manipulation and reflection",
		SourceURL:   "https://github.com/apache/commons-lang",
		SourceRoots: []string{"src/main/java"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewMicronautFactory creates a Source for the Micronaut framework.
func NewMicronautFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "micronaut-projects",
		Repo:        "micronaut-core",
		LibraryID:   "java/micronaut",
		Name:        "Micronaut",
		Description: "Cloud-native framework for Java",
		SourceURL:   "https://github.com/micronaut-projects/micronaut-core",
		SourceRoots: []string{"core/src/main/java/"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewQuarkusFactory creates a Source for the Quarkus framework.
func NewQuarkusFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "quarkusio",
		Repo:        "quarkus",
		LibraryID:   "java/quarkus",
		Name:        "Quarkus",
		Description: "Kubernetes-native framework for Java",
		SourceURL:   "https://github.com/quarkusio/quarkus",
		SourceRoots: []string{"core/runtime/src/main/java/"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewVertxFactory creates a Source for the Vert.x toolkit.
func NewVertxFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "eclipse-vertx",
		Repo:        "vert.x",
		LibraryID:   "java/vertx",
		Name:        "Vert.x",
		Description: "Reactive toolkit for Java",
		SourceURL:   "https://github.com/eclipse-vertx/vert.x",
		// Vert.x 5 moved core sources into the vertx-core module.
		SourceRoots: []string{"vertx-core/src/main/java", "src/main/java"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewDropwizardFactory creates a Source for the Dropwizard framework.
func NewDropwizardFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "dropwizard",
		Repo:        "dropwizard",
		LibraryID:   "java/dropwizard",
		Name:        "Dropwizard",
		Description: "REST framework for Java",
		SourceURL:   "https://github.com/dropwizard/dropwizard",
		SourceRoots: []string{"dropwizard-core/src/main/java/"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewPlayFactory creates a Source for the Play Framework.
func NewPlayFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "playframework",
		Repo:        "playframework",
		LibraryID:   "java/play",
		Name:        "Play Framework",
		Description: "Reactive web framework for Java",
		SourceURL:   "https://github.com/playframework/playframework",
		SourceRoots: []string{"core/play/src/main/java/"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewStrutsFactory creates a Source for the Apache Struts framework.
func NewStrutsFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "apache",
		Repo:        "struts",
		LibraryID:   "java/struts",
		Name:        "Struts",
		Description: "MVC framework for Java",
		SourceURL:   "https://github.com/apache/struts",
		SourceRoots: []string{"core/src/main/java/"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewGrailsFactory creates a Source for the Grails framework.
func NewGrailsFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "grails",
		Repo:        "grails-core",
		LibraryID:   "java/grails",
		Name:        "Grails",
		Description: "Groovy web framework for Java",
		SourceURL:   "https://github.com/grails/grails-core",
		SourceRoots: []string{"grails-core/src/main/groovy/"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewJavalinFactory creates a Source for the Javalin framework.
func NewJavalinFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "javalin",
		Repo:        "javalin",
		LibraryID:   "java/javalin",
		Name:        "Javalin",
		Description: "Lightweight web framework for Java",
		SourceURL:   "https://github.com/javalin/javalin",
		SourceRoots: []string{"javalin/src/main/java/"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewSparkFactory creates a Source for the Spark Java framework.
func NewSparkFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "perwendel",
		Repo:        "spark",
		LibraryID:   "java/spark",
		Name:        "Spark Java",
		Description: "Micro web framework for Java",
		SourceURL:   "https://github.com/perwendel/spark",
		SourceRoots: []string{"src/main/java/"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}

// NewRatpackFactory creates a Source for the Ratpack framework.
func NewRatpackFactory(repoPath string, opts registry.FactoryOptions) (source.Source, error) {
	cfg := Config{
		Owner:       "ratpack",
		Repo:        "ratpack",
		LibraryID:   "java/ratpack",
		Name:        "Ratpack",
		Description: "Async HTTP framework for Java",
		SourceURL:   "https://github.com/ratpack/ratpack",
		SourceRoots: []string{"ratpack-core/src/main/java/"},
		ExcludePatterns: []string{"**/test/**"},
	}
	var sopts []Option
	if opts.Ref != "" {
		sopts = append(sopts, WithRef(opts.Ref))
	}
	return New(repoPath, cfg, sopts...), nil
}
