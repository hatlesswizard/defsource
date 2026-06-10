//go:build sqlite_fts5 || fts5

package csharp

import (
	"context"
	"strings"
	"testing"

	"github.com/hatlesswizard/defsource/internal/source"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestSource() *CSharpSource {
	return New(".", Config{
		LibraryID:   "csharp/test",
		Name:        "Test Library",
		Description: "A test C# library",
		SourceURL:   "https://github.com/test/repo",
	})
}

// ---------------------------------------------------------------------------
// Test — Construction and identity
// ---------------------------------------------------------------------------

func TestNew_ConstructsWithConfig(t *testing.T) {
	s := newTestSource()
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if got := s.ID(); got != "csharp/test" {
		t.Errorf("ID() = %q, want %q", got, "csharp/test")
	}
	meta := s.Meta()
	if meta.Language != "csharp" {
		t.Errorf("Meta().Language = %q, want %q", meta.Language, "csharp")
	}
	if meta.Name != "Test Library" {
		t.Errorf("Meta().Name = %q, want %q", meta.Name, "Test Library")
	}
	if meta.TrustScore <= 0 || meta.TrustScore > 1 {
		t.Errorf("Meta().TrustScore = %v, want (0,1]", meta.TrustScore)
	}
}

func TestNew_ImplementsSourceInterface(t *testing.T) {
	var _ source.Source = newTestSource()
}

// ---------------------------------------------------------------------------
// Test — Class parsing
// ---------------------------------------------------------------------------

func TestParseFile_ClassDeclaration(t *testing.T) {
	src := []byte(`
namespace MyApp.Models;

/// <summary>
/// Represents a user in the system.
/// </summary>
[Serializable]
public class User : BaseEntity, IDisposable
{
    /// <summary>Gets or sets the user name.</summary>
    public string Name { get; set; }

    /// <summary>Gets the email address.</summary>
    public string Email { get; init; }

    private int _age;

    /// <summary>
    /// Creates a new user.
    /// </summary>
    /// <param name="name">The user name.</param>
    /// <param name="email">The email address.</param>
    public User(string name, string email)
    {
        Name = name;
        Email = email;
    }

    /// <summary>
    /// Gets the display name.
    /// </summary>
    /// <returns>The formatted display name.</returns>
    public string GetDisplayName()
    {
        return $"{Name} <{Email}>";
    }

    public void Dispose()
    {
        // cleanup
    }
}
`)

	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if td.Name != "User" {
		t.Errorf("Name = %q, want %q", td.Name, "User")
	}
	if td.Kind != "class" {
		t.Errorf("Kind = %q, want %q", td.Kind, "class")
	}
	if td.Namespace != "MyApp.Models" {
		t.Errorf("Namespace = %q, want %q", td.Namespace, "MyApp.Models")
	}
	if td.Visibility != "public" {
		t.Errorf("Visibility = %q, want %q", td.Visibility, "public")
	}
	if td.qualifiedName() != "MyApp.Models.User" {
		t.Errorf("qualifiedName() = %q, want %q", td.qualifiedName(), "MyApp.Models.User")
	}
	if td.DocComment == "" {
		t.Error("DocComment is empty, expected XML doc")
	}
	if !strings.Contains(td.DocComment, "Represents a user") {
		t.Errorf("DocComment missing expected content, got: %s", td.DocComment)
	}

	// Check properties
	if len(td.Properties) < 2 {
		t.Fatalf("expected at least 2 properties, got %d", len(td.Properties))
	}
	nameProp := td.Properties[0]
	if nameProp.Name != "Name" {
		t.Errorf("Property[0].Name = %q, want %q", nameProp.Name, "Name")
	}
	if nameProp.Type != "string" {
		t.Errorf("Property[0].Type = %q, want %q", nameProp.Type, "string")
	}
	if nameProp.Visibility != "public" {
		t.Errorf("Property[0].Visibility = %q, want %q", nameProp.Visibility, "public")
	}

	// Check methods (constructor + GetDisplayName + Dispose)
	if len(td.Methods) < 2 {
		t.Fatalf("expected at least 2 methods, got %d", len(td.Methods))
	}

	// Find GetDisplayName
	var getDisplayName *methodDef
	for i := range td.Methods {
		if td.Methods[i].Name == "GetDisplayName" {
			getDisplayName = &td.Methods[i]
			break
		}
	}
	if getDisplayName == nil {
		t.Fatal("GetDisplayName method not found")
	}
	if getDisplayName.ReturnType != "string" {
		t.Errorf("GetDisplayName.ReturnType = %q, want %q", getDisplayName.ReturnType, "string")
	}
	if getDisplayName.Visibility != "public" {
		t.Errorf("GetDisplayName.Visibility = %q, want %q", getDisplayName.Visibility, "public")
	}
}

// ---------------------------------------------------------------------------
// Test — Interface parsing
// ---------------------------------------------------------------------------

func TestParseFile_InterfaceDeclaration(t *testing.T) {
	src := []byte(`
namespace MyApp.Services;

/// <summary>
/// Defines a repository for managing entities.
/// </summary>
/// <typeparam name="T">The entity type.</typeparam>
public interface IRepository<T> where T : class
{
    /// <summary>Gets an entity by ID.</summary>
    /// <param name="id">The entity identifier.</param>
    /// <returns>The entity or null.</returns>
    Task<T?> GetByIdAsync(int id);

    /// <summary>Gets all entities.</summary>
    IEnumerable<T> GetAll();

    /// <summary>Adds a new entity.</summary>
    void Add(T entity);
}
`)

	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if td.Name != "IRepository" {
		t.Errorf("Name = %q, want %q", td.Name, "IRepository")
	}
	if td.Kind != "interface" {
		t.Errorf("Kind = %q, want %q", td.Kind, "interface")
	}
	if td.Generics == "" {
		t.Error("Generics is empty, expected type parameters")
	}
	if td.Visibility != "public" {
		t.Errorf("Visibility = %q, want %q", td.Visibility, "public")
	}
	if len(td.Methods) < 3 {
		t.Errorf("expected at least 3 methods, got %d", len(td.Methods))
	}
}

// ---------------------------------------------------------------------------
// Test — Struct parsing
// ---------------------------------------------------------------------------

func TestParseFile_StructDeclaration(t *testing.T) {
	src := []byte(`
namespace MyApp.ValueTypes;

/// <summary>
/// Represents a 2D point.
/// </summary>
public readonly struct Point
{
    public double X { get; }
    public double Y { get; }

    public Point(double x, double y)
    {
        X = x;
        Y = y;
    }

    public double DistanceTo(Point other)
    {
        var dx = X - other.X;
        var dy = Y - other.Y;
        return Math.Sqrt(dx * dx + dy * dy);
    }
}
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if td.Name != "Point" {
		t.Errorf("Name = %q, want %q", td.Name, "Point")
	}
	if td.Kind != "struct" {
		t.Errorf("Kind = %q, want %q", td.Kind, "struct")
	}
	if td.Visibility != "public" {
		t.Errorf("Visibility = %q, want %q", td.Visibility, "public")
	}
}

// ---------------------------------------------------------------------------
// Test — Record parsing
// ---------------------------------------------------------------------------

func TestParseFile_RecordDeclaration(t *testing.T) {
	src := []byte(`
namespace MyApp.Models;

/// <summary>
/// Represents an address with positional parameters.
/// </summary>
public record Address(string Street, string City, string State, string Zip);
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if td.Name != "Address" {
		t.Errorf("Name = %q, want %q", td.Name, "Address")
	}
	if td.Kind != "record" {
		t.Errorf("Kind = %q, want %q", td.Kind, "record")
	}
}

// ---------------------------------------------------------------------------
// Test — Enum parsing
// ---------------------------------------------------------------------------

func TestParseFile_EnumDeclaration(t *testing.T) {
	src := []byte(`
namespace MyApp.Types;

/// <summary>
/// Represents the status of an order.
/// </summary>
public enum OrderStatus
{
    /// <summary>Order is pending.</summary>
    Pending = 0,
    /// <summary>Order is confirmed.</summary>
    Confirmed = 1,
    /// <summary>Order is shipped.</summary>
    Shipped = 2,
    /// <summary>Order is delivered.</summary>
    Delivered = 3,
    /// <summary>Order is cancelled.</summary>
    Cancelled = 4
}
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if td.Name != "OrderStatus" {
		t.Errorf("Name = %q, want %q", td.Name, "OrderStatus")
	}
	if td.Kind != "enum" {
		t.Errorf("Kind = %q, want %q", td.Kind, "enum")
	}
	if len(td.Members) < 5 {
		t.Errorf("expected 5 enum members, got %d", len(td.Members))
	}
}

// ---------------------------------------------------------------------------
// Test — Extension method detection
// ---------------------------------------------------------------------------

func TestParseFile_ExtensionMethod(t *testing.T) {
	src := []byte(`
namespace MyApp.Extensions;

/// <summary>
/// String extension methods.
/// </summary>
public static class StringExtensions
{
    /// <summary>
    /// Truncates a string to the specified length.
    /// </summary>
    /// <param name="str">The string to truncate.</param>
    /// <param name="maxLength">Maximum length.</param>
    /// <returns>The truncated string.</returns>
    public static string Truncate(this string str, int maxLength)
    {
        if (string.IsNullOrEmpty(str)) return str;
        return str.Length <= maxLength ? str : str[..maxLength];
    }
}
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if td.Name != "StringExtensions" {
		t.Errorf("Name = %q, want %q", td.Name, "StringExtensions")
	}
	if !td.IsStatic {
		t.Error("expected static class")
	}
	if len(td.Methods) == 0 {
		t.Fatal("no methods found")
	}

	m := td.Methods[0]
	if m.Name != "Truncate" {
		t.Errorf("Method.Name = %q, want %q", m.Name, "Truncate")
	}
	if !m.IsExtension {
		t.Error("expected extension method")
	}
	if !m.IsStatic {
		t.Error("expected static method")
	}
	if len(m.Params) < 2 {
		t.Fatalf("expected at least 2 params, got %d", len(m.Params))
	}
	if !m.Params[0].IsThis {
		t.Error("first param should have IsThis=true")
	}
}

// ---------------------------------------------------------------------------
// Test — XML doc extraction
// ---------------------------------------------------------------------------

func TestParseFile_XMLDocExtraction(t *testing.T) {
	src := []byte(`
namespace MyApp;

/// <summary>
/// Provides utility methods for calculations.
/// </summary>
/// <remarks>
/// This class is thread-safe.
/// </remarks>
public class Calculator
{
    /// <summary>
    /// Adds two numbers.
    /// </summary>
    /// <param name="a">First number.</param>
    /// <param name="b">Second number.</param>
    /// <returns>The sum of a and b.</returns>
    /// <exception cref="OverflowException">If the result overflows.</exception>
    public int Add(int a, int b)
    {
        return checked(a + b);
    }
}
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if td.DocComment == "" {
		t.Fatal("type DocComment is empty")
	}
	if !strings.Contains(td.DocComment, "Provides utility methods") {
		t.Errorf("type DocComment missing summary, got: %q", td.DocComment)
	}

	if len(td.Methods) == 0 {
		t.Fatal("no methods found")
	}
	m := td.Methods[0]
	if m.DocComment == "" {
		t.Fatal("method DocComment is empty")
	}
	if !strings.Contains(m.DocComment, "Adds two numbers") {
		t.Errorf("method DocComment missing summary, got: %q", m.DocComment)
	}
	if !strings.Contains(m.DocComment, "<param") {
		t.Error("method DocComment missing <param> tags")
	}
	if !strings.Contains(m.DocComment, "<returns>") {
		t.Error("method DocComment missing <returns> tag")
	}
}

// ---------------------------------------------------------------------------
// Test — Nullable type handling
// ---------------------------------------------------------------------------

func TestParseFile_NullableTypes(t *testing.T) {
	src := []byte(`
namespace MyApp;

public class NullableExample
{
    public string? NullableName { get; set; }
    public int? NullableAge { get; set; }

    public string? FindUser(string? name)
    {
        return null;
    }
}
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]

	// Check nullable property types
	if len(td.Properties) < 1 {
		t.Fatal("no properties found")
	}
	found := false
	for _, p := range td.Properties {
		if p.Name == "NullableName" && strings.Contains(p.Type, "string?") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected NullableName property with nullable string type")
	}

	// Check method with nullable params and return
	if len(td.Methods) == 0 {
		t.Fatal("no methods found")
	}
	m := td.Methods[0]
	if !strings.Contains(m.ReturnType, "string?") {
		t.Errorf("expected nullable return type, got: %q", m.ReturnType)
	}
}

// ---------------------------------------------------------------------------
// Test — Partial class merging
// ---------------------------------------------------------------------------

func TestParseFile_PartialClass(t *testing.T) {
	src := []byte(`
namespace MyApp;

public partial class MyService
{
    public void MethodA() { }
}
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if !td.IsPartial {
		t.Error("expected partial class")
	}
	if td.Name != "MyService" {
		t.Errorf("Name = %q, want %q", td.Name, "MyService")
	}
}

// ---------------------------------------------------------------------------
// Test — Wrapper detection
// ---------------------------------------------------------------------------

func TestDetectWrapper_DirectDelegation(t *testing.T) {
	code := `public string GetName()
{
    return _inner.GetName();
}`
	isWrapper, target, kind := detectWrapper(code, nil)
	if !isWrapper {
		t.Error("expected wrapper detection")
	}
	if target != "GetName" {
		t.Errorf("target = %q, want %q", target, "GetName")
	}
	// _inner.Method() is interface forwarding/decoration, kind is "method"
	if kind != "method" {
		t.Errorf("kind = %q, want %q", kind, "method")
	}
}

func TestDetectWrapper_StaticDelegation(t *testing.T) {
	code := `public int Calculate(int x)
{
    return MathHelper.Calculate(x);
}`
	isWrapper, target, kind := detectWrapper(code, nil)
	if !isWrapper {
		t.Error("expected wrapper detection")
	}
	if target != "MathHelper.Calculate" {
		t.Errorf("target = %q, want %q", target, "MathHelper.Calculate")
	}
	if kind != "function" {
		t.Errorf("kind = %q, want %q", kind, "function")
	}
}

func TestDetectWrapper_ExpressionBodied(t *testing.T) {
	code := `public string GetValue() => _cache.GetValue();`
	isWrapper, target, kind := detectWrapper(code, nil)
	if !isWrapper {
		t.Error("expected wrapper detection")
	}
	if target != "GetValue" {
		t.Errorf("target = %q, want %q", target, "GetValue")
	}
	if kind != "self_method" {
		t.Errorf("kind = %q, want %q", kind, "self_method")
	}
}

func TestDetectWrapper_NotAWrapper(t *testing.T) {
	code := `public string Process(string input)
{
    var result = Transform(input);
    var validated = Validate(result);
    return Format(validated);
}`
	isWrapper, _, _ := detectWrapper(code, nil)
	if isWrapper {
		t.Error("expected non-wrapper detection (too many statements)")
	}
}

// ---------------------------------------------------------------------------
// Test — Property accessor patterns
// ---------------------------------------------------------------------------

func TestParseFile_PropertyAccessors(t *testing.T) {
	src := []byte(`
namespace MyApp;

public class PropExample
{
    // Auto-property with get and set
    public string Name { get; set; }

    // Get-only auto-property
    public int Id { get; }

    // Init-only property
    public string Code { get; init; }

    // Full property
    public string FullName
    {
        get { return _fullName; }
        set { _fullName = value; }
    }

    private string _fullName;
}
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if len(td.Properties) < 3 {
		t.Fatalf("expected at least 3 properties, got %d", len(td.Properties))
	}

	// Verify properties exist
	propMap := make(map[string]propertyDef)
	for _, p := range td.Properties {
		propMap[p.Name] = p
	}

	if p, ok := propMap["Name"]; ok {
		if !p.HasGetter {
			t.Error("Name property should have getter")
		}
		if !p.HasSetter {
			t.Error("Name property should have setter")
		}
	} else {
		t.Error("Name property not found")
	}

	if p, ok := propMap["Id"]; ok {
		if !p.HasGetter {
			t.Error("Id property should have getter")
		}
		if p.HasSetter {
			t.Error("Id property should not have setter")
		}
	} else {
		t.Error("Id property not found")
	}
}

// ---------------------------------------------------------------------------
// Test — Generics and attributes
// ---------------------------------------------------------------------------

func TestParseFile_GenericsAndAttributes(t *testing.T) {
	src := []byte(`
namespace MyApp;

/// <summary>A generic service.</summary>
[ServiceLifetime(Lifetime.Scoped)]
public class GenericService<TEntity, TKey> where TEntity : class where TKey : struct
{
    public TEntity? Find(TKey id)
    {
        return default;
    }
}
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if td.Name != "GenericService" {
		t.Errorf("Name = %q, want %q", td.Name, "GenericService")
	}
	if td.Generics == "" {
		t.Error("Generics is empty")
	}
	if !strings.Contains(td.Generics, "TEntity") {
		t.Errorf("Generics should contain TEntity, got: %q", td.Generics)
	}
	if len(td.Attributes) == 0 {
		t.Error("Attributes is empty, expected at least one")
	}
}

// ---------------------------------------------------------------------------
// Test — ParseEntity integration
// ---------------------------------------------------------------------------

func TestParseEntity_Integration(t *testing.T) {
	src := []byte(`
namespace MyApp.Services;

/// <summary>
/// Provides caching functionality.
/// </summary>
public class CacheService
{
    /// <summary>Gets a cached value.</summary>
    /// <param name="key">The cache key.</param>
    /// <returns>The cached value or null.</returns>
    public string? Get(string key)
    {
        return null;
    }

    /// <summary>Sets a cached value.</summary>
    /// <param name="key">The cache key.</param>
    /// <param name="value">The value to cache.</param>
    public void Set(string key, string value)
    {
    }
}
`)

	s := newTestSource()
	entityID := "/fake/path.cs#MyApp.Services.CacheService"
	entity, methodIDs, err := s.ParseEntity(context.Background(), entityID, src)
	if err != nil {
		t.Fatalf("ParseEntity error: %v", err)
	}
	if entity == nil {
		t.Fatal("ParseEntity returned nil entity")
	}
	if entity.Name != "CacheService" {
		t.Errorf("entity.Name = %q, want %q", entity.Name, "CacheService")
	}
	if entity.Kind != "class" {
		t.Errorf("entity.Kind = %q, want %q", entity.Kind, "class")
	}
	if !strings.Contains(entity.Description, "Provides caching") {
		t.Errorf("entity.Description = %q, missing expected content", entity.Description)
	}
	if len(methodIDs) != 2 {
		t.Errorf("expected 2 method IDs, got %d: %v", len(methodIDs), methodIDs)
	}
}

// ---------------------------------------------------------------------------
// Test — ParseMethod integration
// ---------------------------------------------------------------------------

func TestParseMethod_Integration(t *testing.T) {
	src := []byte(`
namespace MyApp.Services;

public class CacheService
{
    /// <summary>Gets a cached value by key.</summary>
    /// <param name="key">The cache key.</param>
    /// <returns>The cached value or null.</returns>
    public string? Get(string key)
    {
        return null;
    }
}
`)

	s := newTestSource()
	methodID := "/fake/path.cs#MyApp.Services.CacheService.Get"
	method, err := s.ParseMethod(context.Background(), methodID, src)
	if err != nil {
		t.Fatalf("ParseMethod error: %v", err)
	}
	if method == nil {
		t.Fatal("ParseMethod returned nil")
	}
	if method.Name != "Get" {
		t.Errorf("method.Name = %q, want %q", method.Name, "Get")
	}
	if !strings.Contains(method.Description, "Gets a cached value") {
		t.Errorf("method.Description = %q, missing expected content", method.Description)
	}
	if len(method.Parameters) == 0 {
		t.Error("expected at least 1 parameter")
	} else {
		if method.Parameters[0].Name != "key" {
			t.Errorf("param[0].Name = %q, want %q", method.Parameters[0].Name, "key")
		}
	}
}

// ---------------------------------------------------------------------------
// Test — Discovery filters
// ---------------------------------------------------------------------------

func TestIsTestPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"src/MyApp/MyClass.cs", false},
		{"src/MyApp.Tests/MyClassTests.cs", true},
		{"tests/UnitTest1.cs", true},
		{"src/MyApp/Services/Service.cs", false},
		{"test/Integration/Test.cs", true},
		{"benchmarks/Bench.cs", true},
	}
	for _, tt := range tests {
		got := isTestPath(tt.path)
		if got != tt.want {
			t.Errorf("isTestPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsGeneratedFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"normal file", "using System;\npublic class Foo {}", false},
		{"auto-generated", "// <auto-generated>\n// This file was generated\n// </auto-generated>\npublic class Foo {}", true},
	}
	for _, tt := range tests {
		got := isGeneratedFile([]byte(tt.content))
		if got != tt.want {
			t.Errorf("isGeneratedFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Test — Delegate parsing
// ---------------------------------------------------------------------------

func TestParseFile_DelegateDeclaration(t *testing.T) {
	src := []byte(`
namespace MyApp.Events;

/// <summary>Handler for custom events.</summary>
public delegate void EventHandler<TEventArgs>(object sender, TEventArgs e);
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if td.Name != "EventHandler" {
		t.Errorf("Name = %q, want %q", td.Name, "EventHandler")
	}
	if td.Kind != "delegate" {
		t.Errorf("Kind = %q, want %q", td.Kind, "delegate")
	}
}

// ---------------------------------------------------------------------------
// Test — Async method detection
// ---------------------------------------------------------------------------

func TestParseFile_AsyncMethod(t *testing.T) {
	src := []byte(`
namespace MyApp;

public class AsyncService
{
    public async Task<string> FetchDataAsync(string url)
    {
        return await _client.GetStringAsync(url);
    }
}
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if len(td.Methods) == 0 {
		t.Fatal("no methods found")
	}

	m := td.Methods[0]
	if m.Name != "FetchDataAsync" {
		t.Errorf("Method.Name = %q, want %q", m.Name, "FetchDataAsync")
	}
	if !m.IsAsync {
		t.Error("expected async method")
	}
}

// ---------------------------------------------------------------------------
// Test — File-scoped namespace
// ---------------------------------------------------------------------------

func TestParseFile_FileScopedNamespace(t *testing.T) {
	src := []byte(`
namespace MyApp.Services;

public class MyService
{
    public void DoWork() { }
}

public interface IMyService
{
    void DoWork();
}
`)

	analysis := parseFile(src)
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Types) < 2 {
		t.Fatalf("expected at least 2 types, got %d", len(analysis.Types))
	}

	// Both should have the file-scoped namespace
	for _, td := range analysis.Types {
		if td.Namespace != "MyApp.Services" {
			t.Errorf("type %s: Namespace = %q, want %q", td.Name, td.Namespace, "MyApp.Services")
		}
	}
}

// ---------------------------------------------------------------------------
// Test — Traditional namespace
// ---------------------------------------------------------------------------

func TestParseFile_TraditionalNamespace(t *testing.T) {
	src := []byte(`
namespace MyApp.Models
{
    public class Order
    {
        public int Id { get; set; }
    }
}
`)

	analysis := parseFile(src)
	if analysis == nil || len(analysis.Types) == 0 {
		t.Fatal("no types found")
	}

	td := analysis.Types[0]
	if td.Namespace != "MyApp.Models" {
		t.Errorf("Namespace = %q, want %q", td.Namespace, "MyApp.Models")
	}
	if td.Name != "Order" {
		t.Errorf("Name = %q, want %q", td.Name, "Order")
	}
}
