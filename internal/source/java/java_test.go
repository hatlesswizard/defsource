package java

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/source"
)

// testConfig returns a Config suitable for tests.
func testConfig() Config {
	return Config{
		Owner:       "test",
		Repo:        "test-repo",
		LibraryID:   "java/test",
		Name:        "Test Library",
		Description: "A test Java library",
		SourceURL:   "https://github.com/test/test-repo",
		SourceRoots: []string{"src/main/java"},
	}
}

func TestSourceID(t *testing.T) {
	s := New("", testConfig())
	if got := s.ID(); got != "java/test" {
		t.Errorf("ID() = %q, want %q", got, "java/test")
	}
}

func TestSourceMeta(t *testing.T) {
	s := New("", testConfig(), WithRef("v1.0.0"))
	meta := s.Meta()
	if meta.Language != "java" {
		t.Errorf("Meta().Language = %q, want %q", meta.Language, "java")
	}
	if meta.Version != "v1.0.0" {
		t.Errorf("Meta().Version = %q, want %q", meta.Version, "v1.0.0")
	}
	if meta.Name != "Test Library" {
		t.Errorf("Meta().Name = %q, want %q", meta.Name, "Test Library")
	}
}

const testClassSource = `package com.example;

/**
 * A simple service for managing users.
 *
 * @since 1.0
 */
public class UserService {
    private final UserRepository repository;

    /**
     * Creates a new UserService.
     *
     * @param repository the user repository
     */
    public UserService(UserRepository repository) {
        this.repository = repository;
    }

    /**
     * Finds a user by their ID.
     *
     * @param id the user identifier
     * @return the user, or null if not found
     * @throws IllegalArgumentException if id is negative
     * @since 1.0
     */
    public User findById(long id) throws IllegalArgumentException {
        return repository.findById(id);
    }

    /**
     * Saves a user.
     *
     * @param user the user to save
     */
    public void save(User user) {
        repository.save(user);
    }

    /**
     * Finds users by name.
     *
     * @param name the name to search
     * @param limit maximum results
     * @return list of matching users
     */
    public List<User> findByName(String name, int limit) {
        return repository.findByName(name, limit);
    }
}
`

func TestParseClassEntity(t *testing.T) {
	analysis := parseFile([]byte(testClassSource))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	cls := analysis.Types[0]
	if cls.Name != "UserService" {
		t.Errorf("class name = %q, want %q", cls.Name, "UserService")
	}
	if cls.Kind != source.KindClass {
		t.Errorf("class kind = %q, want %q", cls.Kind, source.KindClass)
	}
	if cls.Visibility != "public" {
		t.Errorf("visibility = %q, want %q", cls.Visibility, "public")
	}
	if cls.Doc == nil {
		t.Fatal("doc is nil")
	}
	if !strings.Contains(cls.Doc.Description, "managing users") {
		t.Errorf("doc description does not contain expected text: %q", cls.Doc.Description)
	}

	// Check methods.
	if len(cls.Methods) != 4 { // constructor + findById + save + findByName
		t.Errorf("method count = %d, want 4", len(cls.Methods))
	}

	// Check fields.
	if len(cls.Fields) != 1 {
		t.Errorf("field count = %d, want 1", len(cls.Fields))
	}
	if len(cls.Fields) > 0 && cls.Fields[0].Name != "repository" {
		t.Errorf("field name = %q, want %q", cls.Fields[0].Name, "repository")
	}
}

func TestParseMethodWithParams(t *testing.T) {
	analysis := parseFile([]byte(testClassSource))
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("parse failed")
	}

	cls := &analysis.Types[0]
	var findById *javaMethod
	for i := range cls.Methods {
		if cls.Methods[i].Name == "findById" {
			findById = &cls.Methods[i]
			break
		}
	}
	if findById == nil {
		t.Fatal("findById method not found")
	}

	if findById.ReturnType != "User" {
		t.Errorf("returnType = %q, want %q", findById.ReturnType, "User")
	}
	if len(findById.Params) != 1 {
		t.Fatalf("param count = %d, want 1", len(findById.Params))
	}
	if findById.Params[0].Name != "id" {
		t.Errorf("param name = %q, want %q", findById.Params[0].Name, "id")
	}
	if findById.Params[0].Type != "long" {
		t.Errorf("param type = %q, want %q", findById.Params[0].Type, "long")
	}
	if len(findById.Throws) == 0 {
		t.Error("expected throws clause")
	}
	if findById.Doc == nil {
		t.Fatal("method doc is nil")
	}
	if findById.Doc.Since != "1.0" {
		t.Errorf("since = %q, want %q", findById.Doc.Since, "1.0")
	}
}

const testInterfaceSource = `package com.example;

/**
 * Repository for user entities.
 */
public interface UserRepository {
    /**
     * Finds a user by ID.
     *
     * @param id the user ID
     * @return the user or null
     */
    User findById(long id);

    /**
     * Default method for checking existence.
     *
     * @param id the user ID
     * @return true if user exists
     */
    default boolean exists(long id) {
        return findById(id) != null;
    }

    /**
     * Static factory method.
     *
     * @return a new in-memory repository
     */
    static UserRepository inMemory() {
        return new InMemoryUserRepository();
    }
}
`

func TestParseInterface(t *testing.T) {
	analysis := parseFile([]byte(testInterfaceSource))
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("parse failed")
	}

	iface := analysis.Types[0]
	if iface.Name != "UserRepository" {
		t.Errorf("name = %q, want %q", iface.Name, "UserRepository")
	}
	if iface.Kind != source.KindInterface {
		t.Errorf("kind = %q, want %q", iface.Kind, source.KindInterface)
	}
	if len(iface.Methods) != 3 {
		t.Errorf("method count = %d, want 3", len(iface.Methods))
	}

	// Check default method.
	var existsMethod *javaMethod
	for i := range iface.Methods {
		if iface.Methods[i].Name == "exists" {
			existsMethod = &iface.Methods[i]
			break
		}
	}
	if existsMethod == nil {
		t.Fatal("exists method not found")
	}
	if !existsMethod.Default {
		t.Error("exists should be marked as default")
	}

	// Check static method.
	var staticMethod *javaMethod
	for i := range iface.Methods {
		if iface.Methods[i].Name == "inMemory" {
			staticMethod = &iface.Methods[i]
			break
		}
	}
	if staticMethod == nil {
		t.Fatal("inMemory method not found")
	}
	if !staticMethod.Static {
		t.Error("inMemory should be marked as static")
	}
}

const testEnumSource = `package com.example;

/**
 * Represents HTTP status codes.
 */
public enum HttpStatus {
    OK(200, "OK"),
    NOT_FOUND(404, "Not Found"),
    INTERNAL_ERROR(500, "Internal Server Error");

    private final int code;
    private final String message;

    HttpStatus(int code, String message) {
        this.code = code;
        this.message = message;
    }

    /**
     * Gets the numeric status code.
     *
     * @return the HTTP status code
     */
    public int getCode() {
        return code;
    }

    /**
     * Gets the reason phrase.
     *
     * @return the reason phrase
     */
    public String getMessage() {
        return message;
    }
}
`

func TestParseEnum(t *testing.T) {
	analysis := parseFile([]byte(testEnumSource))
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("parse failed")
	}

	enum := analysis.Types[0]
	if enum.Name != "HttpStatus" {
		t.Errorf("name = %q, want %q", enum.Name, "HttpStatus")
	}
	if enum.Kind != source.KindEnum {
		t.Errorf("kind = %q, want %q", enum.Kind, source.KindEnum)
	}
	if enum.Visibility != "public" {
		t.Errorf("visibility = %q, want %q", enum.Visibility, "public")
	}

	// Should have constructor + getCode + getMessage.
	methodNames := make(map[string]bool)
	for _, m := range enum.Methods {
		methodNames[m.Name] = true
	}
	if !methodNames["getCode"] {
		t.Error("getCode method not found")
	}
	if !methodNames["getMessage"] {
		t.Error("getMessage method not found")
	}
}

const testRecordSource = `package com.example;

/**
 * Represents a point in 2D space.
 *
 * @param x the x-coordinate
 * @param y the y-coordinate
 * @since 16
 */
public record Point(double x, double y) {
    /**
     * Calculates distance to another point.
     *
     * @param other the other point
     * @return the distance
     */
    public double distanceTo(Point other) {
        double dx = this.x - other.x();
        double dy = this.y - other.y();
        return Math.sqrt(dx * dx + dy * dy);
    }
}
`

func TestParseRecord(t *testing.T) {
	analysis := parseFile([]byte(testRecordSource))
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("parse failed")
	}

	rec := analysis.Types[0]
	if rec.Name != "Point" {
		t.Errorf("name = %q, want %q", rec.Name, "Point")
	}
	if rec.Kind != source.KindRecord {
		t.Errorf("kind = %q, want %q", rec.Kind, source.KindRecord)
	}

	// Should have fields from record components.
	if len(rec.Fields) < 2 {
		t.Errorf("field count = %d, want >= 2", len(rec.Fields))
	}

	// Should have the distanceTo method.
	if len(rec.Methods) == 0 {
		t.Error("no methods found")
	}
}

const testAnnotationSource = `package com.example;

import java.lang.annotation.*;

/**
 * Marks a method as requiring authentication.
 *
 * @since 2.0
 */
@Retention(RetentionPolicy.RUNTIME)
@Target(ElementType.METHOD)
public @interface RequiresAuth {
    /**
     * The required role.
     *
     * @return the role name
     */
    String role() default "USER";

    /**
     * Whether to check permissions.
     *
     * @return true if permissions should be checked
     */
    boolean checkPermissions() default true;
}
`

func TestParseAnnotation(t *testing.T) {
	analysis := parseFile([]byte(testAnnotationSource))
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("parse failed")
	}

	ann := analysis.Types[0]
	if ann.Name != "RequiresAuth" {
		t.Errorf("name = %q, want %q", ann.Name, "RequiresAuth")
	}
	if ann.Kind != source.KindAnnotation {
		t.Errorf("kind = %q, want %q", ann.Kind, source.KindAnnotation)
	}

	// Should have annotation elements as methods.
	if len(ann.Methods) < 2 {
		t.Errorf("method count = %d, want >= 2", len(ann.Methods))
	}
}

const testGenericsSource = `package com.example;

/**
 * A generic container with bounded type parameter.
 *
 * @param <T> the element type
 */
public class Container<T extends Comparable<T>> implements Iterable<T> {
    private T value;

    /**
     * Gets the value.
     *
     * @return the contained value
     */
    public T getValue() {
        return value;
    }

    /**
     * Transforms the value.
     *
     * @param <R> the result type
     * @param mapper the transformation function
     * @return a new container with the mapped value
     */
    public <R extends Comparable<R>> Container<R> map(Function<T, R> mapper) {
        return new Container<>(mapper.apply(value));
    }
}
`

func TestParseGenerics(t *testing.T) {
	analysis := parseFile([]byte(testGenericsSource))
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("parse failed")
	}

	cls := analysis.Types[0]
	if cls.Name != "Container" {
		t.Errorf("name = %q, want %q", cls.Name, "Container")
	}
	if cls.TypeParams == "" {
		t.Error("TypeParams is empty, expected generic type parameters")
	}
	if !strings.Contains(cls.TypeParams, "T") {
		t.Errorf("TypeParams = %q, expected to contain 'T'", cls.TypeParams)
	}
	if len(cls.Implements) == 0 {
		t.Error("implements list is empty, expected Iterable<T>")
	}

	// Check method-level generics.
	var mapMethod *javaMethod
	for i := range cls.Methods {
		if cls.Methods[i].Name == "map" {
			mapMethod = &cls.Methods[i]
			break
		}
	}
	if mapMethod == nil {
		t.Fatal("map method not found")
	}
	if mapMethod.TypeParams == "" {
		t.Error("map method TypeParams is empty")
	}
}

const testInnerClassSource = `package com.example;

/**
 * Builder pattern example.
 */
public class Config {
    private final String host;
    private final int port;

    private Config(Builder builder) {
        this.host = builder.host;
        this.port = builder.port;
    }

    /**
     * Builder for Config.
     */
    public static class Builder {
        private String host = "localhost";
        private int port = 8080;

        /**
         * Sets the host.
         *
         * @param host the hostname
         * @return this builder
         */
        public Builder host(String host) {
            this.host = host;
            return this;
        }

        /**
         * Builds the config.
         *
         * @return the config instance
         */
        public Config build() {
            return new Config(this);
        }
    }
}
`

func TestParseInnerClass(t *testing.T) {
	analysis := parseFile([]byte(testInnerClassSource))
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("parse failed")
	}

	outer := analysis.Types[0]
	if outer.Name != "Config" {
		t.Errorf("outer name = %q, want %q", outer.Name, "Config")
	}
	if len(outer.InnerTypes) == 0 {
		t.Fatal("no inner types found")
	}

	inner := outer.InnerTypes[0]
	if inner.Name != "Builder" {
		t.Errorf("inner name = %q, want %q", inner.Name, "Builder")
	}
	if inner.Kind != source.KindClass {
		t.Errorf("inner kind = %q, want %q", inner.Kind, source.KindClass)
	}

	// Test findType with dotted notation.
	found := analysis.findType("Config.Builder")
	if found == nil {
		t.Fatal("findType(Config.Builder) returned nil")
	}
	if found.Name != "Builder" {
		t.Errorf("findType result name = %q, want %q", found.Name, "Builder")
	}
}

const testOverloadSource = `package com.example;

public class Printer {
    public void print(String msg) {
        System.out.println(msg);
    }

    public void print(String msg, int count) {
        for (int i = 0; i < count; i++) {
            System.out.println(msg);
        }
    }

    public void print(Object obj) {
        System.out.println(obj.toString());
    }
}
`

func TestOverloadedMethods(t *testing.T) {
	analysis := parseFile([]byte(testOverloadSource))
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("parse failed")
	}

	cls := analysis.Types[0]
	printMethods := 0
	sigs := make(map[string]bool)
	for _, m := range cls.Methods {
		if m.Name == "print" {
			printMethods++
			sigs[m.paramSignature()] = true
		}
	}

	if printMethods != 3 {
		t.Errorf("print method count = %d, want 3", printMethods)
	}
	if len(sigs) != 3 {
		t.Errorf("unique signatures = %d, want 3", len(sigs))
	}
}

const testWrapperSource = `{
    return delegate.execute(request);
}`

const testSelfWrapperSource = `{
    return this.doExecute(request);
}`

const testStaticWrapperSource = `{
    return Collections.unmodifiableList(items);
}`

const testNonWrapperSource = `{
    List<User> users = repository.findAll();
    for (User u : users) {
        u.setActive(true);
    }
    repository.saveAll(users);
    return users;
}`

func TestWrapperDetection(t *testing.T) {
	idx := emptyIndex()
	idx.definedClasses["Collections"] = "/some/path.java"

	tests := []struct {
		name     string
		src      string
		wantWrap bool
		wantKind string
	}{
		{"delegate pattern", testWrapperSource, true, "method"},
		{"self method", testSelfWrapperSource, true, "self_method"},
		{"static method", testStaticWrapperSource, true, "static_method"},
		{"non-wrapper", testNonWrapperSource, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isWrapper, _, kind := detectWrapper([]byte(tt.src), idx)
			if isWrapper != tt.wantWrap {
				t.Errorf("isWrapper = %v, want %v", isWrapper, tt.wantWrap)
			}
			if isWrapper && kind != tt.wantKind {
				t.Errorf("kind = %q, want %q", kind, tt.wantKind)
			}
		})
	}
}

func TestDiscovery(t *testing.T) {
	// Create a temporary directory structure for testing.
	tmp := t.TempDir()

	// Create src/main/java structure.
	javaDir := filepath.Join(tmp, "src", "main", "java", "com", "example")
	if err := os.MkdirAll(javaDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a public class.
	publicClass := `package com.example;

/**
 * A public service class.
 */
public class MyService {
    public void doWork() {}
}
`
	if err := os.WriteFile(filepath.Join(javaDir, "MyService.java"), []byte(publicClass), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a package-private class (should still be discovered since it has no modifier).
	pkgClass := `package com.example;

class InternalHelper {
    void help() {}
}
`
	if err := os.WriteFile(filepath.Join(javaDir, "InternalHelper.java"), []byte(pkgClass), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a test file (should be skipped).
	testDir := filepath.Join(tmp, "src", "test", "java", "com", "example")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testFile := `package com.example;

public class MyServiceTest {
    public void testDoWork() {}
}
`
	if err := os.WriteFile(filepath.Join(testDir, "MyServiceTest.java"), []byte(testFile), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		LibraryID:   "java/test",
		Name:        "Test",
		SourceURL:   "https://github.com/test/test",
		SourceRoots: []string{"src/main/java"},
	}

	s := New(tmp, cfg)
	entities, err := s.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) == 0 {
		t.Fatal("no entities discovered")
	}

	// MyService should be discovered.
	found := false
	for _, e := range entities {
		if strings.Contains(e, "MyService") {
			found = true
			break
		}
	}
	if !found {
		t.Error("MyService not found in discovered entities")
	}

	// Test files should NOT be discovered (check fragment part only).
	for _, e := range entities {
		_, fragment := splitFragment(e)
		if strings.Contains(fragment, "Test") {
			t.Errorf("test class %q should not be discovered", e)
		}
	}
}

func TestParseEntityFullFlow(t *testing.T) {
	tmp := t.TempDir()
	javaDir := filepath.Join(tmp, "src", "main", "java", "com", "example")
	if err := os.MkdirAll(javaDir, 0o755); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(javaDir, "UserService.java")
	if err := os.WriteFile(filePath, []byte(testClassSource), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		LibraryID:   "java/test",
		Name:        "Test",
		SourceURL:   "https://github.com/test/test",
		SourceRoots: []string{"src/main/java"},
	}

	s := New(tmp, cfg, WithRef("v1.0.0"))

	// Discover first.
	_, err := s.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Parse the entity.
	entityID := filePath + "#UserService"
	entity, methodURLs, err := s.ParseEntity(context.Background(), entityID, []byte(testClassSource))
	if err != nil {
		t.Fatal(err)
	}

	if entity.Name != "UserService" {
		t.Errorf("entity name = %q, want %q", entity.Name, "UserService")
	}
	if entity.Kind != source.KindClass {
		t.Errorf("entity kind = %q, want %q", entity.Kind, source.KindClass)
	}
	if len(methodURLs) == 0 {
		t.Error("no method URLs returned")
	}

	// Parse a method.
	for _, mURL := range methodURLs {
		if strings.Contains(mURL, "findById") {
			method, err := s.ParseMethod(context.Background(), mURL, []byte(testClassSource))
			if err != nil {
				t.Fatal(err)
			}
			if method.Name != "findById" {
				t.Errorf("method name = %q, want %q", method.Name, "findById")
			}
			if method.ReturnType != "User" {
				t.Errorf("return type = %q, want %q", method.ReturnType, "User")
			}
			if len(method.Parameters) != 1 {
				t.Errorf("param count = %d, want 1", len(method.Parameters))
			}
			break
		}
	}
}

func TestJavaDocExtraction(t *testing.T) {
	src := `package com.example;

/**
 * Handles HTTP requests.
 *
 * <p>This class provides a fluent API for building HTTP handlers.
 *
 * @since 2.0
 * @see HttpResponse
 * @deprecated Use NewHandler instead
 */
public class HttpHandler {
}
`
	analysis := parseFile([]byte(src))
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("parse failed")
	}

	cls := analysis.Types[0]
	if cls.Doc == nil {
		t.Fatal("doc is nil")
	}
	if cls.Doc.Since != "2.0" {
		t.Errorf("since = %q, want %q", cls.Doc.Since, "2.0")
	}
	if cls.Doc.Deprecated == "" {
		t.Error("expected deprecated to be set")
	}
	if len(cls.Doc.See) == 0 {
		t.Error("expected @see entries")
	}
}

func TestParamSignature(t *testing.T) {
	tests := []struct {
		params []javaParam
		want   string
	}{
		{nil, "()"},
		{[]javaParam{{Name: "a", Type: "int"}}, "(int)"},
		{[]javaParam{
			{Name: "a", Type: "String"},
			{Name: "b", Type: "int"},
		}, "(String,int)"},
		{[]javaParam{
			{Name: "args", Type: "String...", Variadic: true},
		}, "(String...)"},
	}

	for _, tt := range tests {
		m := &javaMethod{Params: tt.params}
		got := m.paramSignature()
		if got != tt.want {
			t.Errorf("paramSignature(%v) = %q, want %q", tt.params, got, tt.want)
		}
	}
}

const testVarArgsSource = `package com.example;

public class VarArgExample {
    public void log(String format, Object... args) {
        System.out.printf(format, args);
    }
}
`

func TestVarArgs(t *testing.T) {
	analysis := parseFile([]byte(testVarArgsSource))
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("parse failed")
	}

	cls := analysis.Types[0]
	if len(cls.Methods) == 0 {
		t.Fatal("no methods found")
	}

	logMethod := cls.Methods[0]
	if len(logMethod.Params) != 2 {
		t.Fatalf("param count = %d, want 2", len(logMethod.Params))
	}
	lastParam := logMethod.Params[1]
	if !lastParam.Variadic {
		t.Error("last param should be variadic")
	}
	if !strings.Contains(lastParam.Type, "...") {
		t.Errorf("variadic type = %q, expected to contain '...'", lastParam.Type)
	}
}

func TestSplitDot(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"Foo", []string{"Foo"}},
		{"Foo.Bar", []string{"Foo", "Bar"}},
		{"Foo.Bar.Baz", []string{"Foo", "Bar", "Baz"}},
		{"Map<String, Integer>.Entry", []string{"Map<String, Integer>", "Entry"}},
		{"", nil},
	}

	for _, tt := range tests {
		got := splitDot(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitDot(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitDot(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

const testDeprecatedSource = `package com.example;

public class Legacy {
    /**
     * @deprecated Use newMethod instead
     */
    @Deprecated
    public void oldMethod() {}

    public void newMethod() {}
}
`

func TestDeprecatedDetection(t *testing.T) {
	analysis := parseFile([]byte(testDeprecatedSource))
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("parse failed")
	}

	cls := analysis.Types[0]
	var oldMethod *javaMethod
	for i := range cls.Methods {
		if cls.Methods[i].Name == "oldMethod" {
			oldMethod = &cls.Methods[i]
			break
		}
	}
	if oldMethod == nil {
		t.Fatal("oldMethod not found")
	}
	if !oldMethod.Deprecated {
		t.Error("oldMethod should be deprecated")
	}
}
