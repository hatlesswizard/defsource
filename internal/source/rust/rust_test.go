package rust

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testFixtureDir creates a temporary directory with a Rust source structure.
func testFixtureDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(srcDir, name)
		parent := filepath.Dir(path)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const sampleStruct = `/// A connection pool for database connections.
///
/// Manages a set of reusable connections.
///
/// # Arguments
///
/// * ` + "`max_size`" + ` - Maximum pool size
#[derive(Debug, Clone)]
pub struct Pool<T: Connection> {
    /// The maximum number of connections.
    pub max_size: usize,
    /// Internal connection storage.
    connections: Vec<T>,
    /// Whether the pool is active.
    pub active: bool,
}
`

const sampleEnum = `/// Represents the state of a task.
pub enum TaskState {
    /// Task is waiting to be scheduled.
    Pending,
    /// Task is currently running.
    Running(u64),
    /// Task completed successfully.
    Done { result: String, elapsed: u64 },
}
`

const sampleTrait = `/// A trait for types that can be serialized.
pub trait Serialize {
    /// The output type.
    type Output;

    /// Serializes self into bytes.
    fn serialize(&self) -> Vec<u8>;

    /// Deserializes from bytes with a default implementation.
    fn deserialize(data: &[u8]) -> Self where Self: Sized {
        unimplemented!()
    }
}
`

const sampleFunctions = `/// Creates a new runtime with the specified configuration.
///
/// # Arguments
///
/// * ` + "`threads`" + ` - Number of worker threads
///
/// # Returns
///
/// A configured Runtime instance.
///
/// # Errors
///
/// Returns an error if thread creation fails.
pub fn create_runtime(threads: usize) -> Result<Runtime, Error> {
    Runtime::builder()
        .worker_threads(threads)
        .build()
}

/// An async function that fetches data.
pub async fn fetch_data(url: &str) -> Result<Vec<u8>, Error> {
    client::get(url).await
}

/// An unsafe function for raw memory access.
pub unsafe fn read_raw(ptr: *const u8, len: usize) -> &[u8] {
    std::slice::from_raw_parts(ptr, len)
}

// This is private, should not be discovered.
fn internal_helper() -> bool {
    true
}
`

const sampleImpl = `pub struct Server {
    pub addr: String,
    port: u16,
}

impl Server {
    /// Creates a new server.
    pub fn new(addr: String, port: u16) -> Self {
        Server { addr, port }
    }

    /// Starts the server.
    pub async fn start(&self) -> Result<(), Error> {
        listen(self.addr.clone(), self.port).await
    }

    // Private method, should still be tracked
    fn validate(&self) -> bool {
        self.port > 0
    }
}

impl Display for Server {
    fn fmt(&self, f: &mut Formatter) -> fmt::Result {
        write!(f, "{}:{}", self.addr, self.port)
    }
}
`

const sampleTypeAlias = `/// A boxed future that is Send.
pub type BoxFuture<'a, T> = Pin<Box<dyn Future<Output = T> + Send + 'a>>;
`

const sampleMacro = `/// Creates a vector with the given elements.
///
/// # Examples
///
/// ` + "```" + `
/// let v = vec![1, 2, 3];
/// ` + "```" + `
macro_rules! vec {
    ($($x:expr),*) => {
        {
            let mut v = Vec::new();
            $(v.push($x);)*
            v
        }
    };
}
`

const sampleConstant = `/// The maximum buffer size in bytes.
pub const MAX_BUFFER_SIZE: usize = 8192;

/// The global allocator instance.
pub static ALLOCATOR: MyAllocator = MyAllocator::new();
`

const sampleDocHidden = `/// This is public but doc-hidden.
#[doc(hidden)]
pub fn hidden_function() -> bool {
    true
}

/// This is visible.
pub fn visible_function() -> bool {
    false
}
`

const sampleReExport = `pub use crate::runtime::Runtime;
pub use crate::io::AsyncRead as Read;
`

const sampleTestModule = `pub fn public_fn() -> i32 { 42 }

#[cfg(test)]
mod tests {
    pub fn test_helper() -> bool { true }
}
`

func TestDiscoverPubItems(t *testing.T) {
	dir := testFixtureDir(t, map[string]string{
		"lib.rs": sampleStruct + "\n" + sampleFunctions,
	})

	src := New(dir, Config{
		Owner:     "test",
		Repo:      "test",
		CrateName: "test",
	})

	ids, err := src.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(ids) == 0 {
		t.Fatal("expected discovered entities, got 0")
	}

	// Should find Pool struct and public functions
	foundPool := false
	foundRuntime := false
	foundFetch := false
	foundInternal := false
	for _, id := range ids {
		_, frag := splitFragment(id)
		switch frag {
		case "Pool":
			foundPool = true
		case "create_runtime":
			foundRuntime = true
		case "fetch_data":
			foundFetch = true
		case "internal_helper":
			foundInternal = true
		}
	}

	if !foundPool {
		t.Error("expected to discover Pool struct")
	}
	if !foundRuntime {
		t.Error("expected to discover create_runtime function")
	}
	if !foundFetch {
		t.Error("expected to discover fetch_data function")
	}
	if foundInternal {
		t.Error("should not discover private internal_helper function")
	}
}

func TestStructParsing(t *testing.T) {
	analysis := parseFile([]byte(sampleStruct))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(analysis.Structs))
	}

	st := analysis.Structs[0]
	if st.Name != "Pool" {
		t.Errorf("expected name Pool, got %q", st.Name)
	}

	if st.Generics == "" {
		t.Error("expected generics to be non-empty")
	}

	// Should have pub fields only
	pubFields := 0
	for _, f := range st.Fields {
		if f.Visibility == "public" {
			pubFields++
		}
	}
	// All fields are extracted, visibility is tracked
	if len(st.Fields) < 2 {
		t.Errorf("expected at least 2 fields, got %d", len(st.Fields))
	}

	// Check derives
	if len(st.Derives) == 0 {
		t.Error("expected derives to be non-empty")
	}

	// Check doc comment
	if st.DocComment == "" {
		t.Error("expected doc comment to be non-empty")
	}
	if !strings.Contains(st.DocComment, "connection pool") {
		t.Errorf("unexpected doc comment: %q", st.DocComment)
	}
}

func TestEnumParsing(t *testing.T) {
	analysis := parseFile([]byte(sampleEnum))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(analysis.Enums))
	}

	en := analysis.Enums[0]
	if en.Name != "TaskState" {
		t.Errorf("expected name TaskState, got %q", en.Name)
	}

	if len(en.Variants) < 3 {
		t.Fatalf("expected at least 3 variants, got %d", len(en.Variants))
	}

	// Check variant names
	names := make(map[string]bool)
	for _, v := range en.Variants {
		names[v.Name] = true
	}
	for _, expected := range []string{"Pending", "Running", "Done"} {
		if !names[expected] {
			t.Errorf("missing variant %q", expected)
		}
	}
}

func TestTraitParsing(t *testing.T) {
	analysis := parseFile([]byte(sampleTrait))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Traits) != 1 {
		t.Fatalf("expected 1 trait, got %d", len(analysis.Traits))
	}

	tr := analysis.Traits[0]
	if tr.Name != "Serialize" {
		t.Errorf("expected name Serialize, got %q", tr.Name)
	}

	// Should have methods
	if len(tr.Methods) < 1 {
		t.Error("expected at least 1 method in trait")
	}

	// Should have associated type
	if len(tr.AssocTypes) < 1 {
		t.Error("expected at least 1 associated type")
	}
	if tr.AssocTypes[0].Name != "Output" {
		t.Errorf("expected assoc type Output, got %q", tr.AssocTypes[0].Name)
	}
}

func TestImplBlockAssociation(t *testing.T) {
	analysis := parseFile([]byte(sampleImpl))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.ImplBlocks) < 2 {
		t.Fatalf("expected at least 2 impl blocks, got %d", len(analysis.ImplBlocks))
	}

	// Find the inherent impl
	var inherentImpl *implBlockDef
	var traitImpl *implBlockDef
	for i := range analysis.ImplBlocks {
		ib := &analysis.ImplBlocks[i]
		if ib.TypeName == "Server" {
			if ib.TraitName == "" {
				inherentImpl = ib
			} else {
				traitImpl = ib
			}
		}
	}

	if inherentImpl == nil {
		t.Fatal("expected inherent impl for Server")
	}
	if traitImpl == nil {
		t.Fatal("expected trait impl for Server")
	}

	// Inherent impl should have new and start methods
	methodNames := make(map[string]bool)
	for _, m := range inherentImpl.Methods {
		methodNames[m.Name] = true
	}
	if !methodNames["new"] {
		t.Error("expected method 'new'")
	}
	if !methodNames["start"] {
		t.Error("expected method 'start'")
	}
}

func TestFunctionParsing(t *testing.T) {
	analysis := parseFile([]byte(sampleFunctions))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	// Should find public functions only
	if len(analysis.Functions) < 3 {
		t.Fatalf("expected at least 3 public functions, got %d", len(analysis.Functions))
	}

	funcMap := make(map[string]*functionDef)
	for i := range analysis.Functions {
		funcMap[analysis.Functions[i].Name] = &analysis.Functions[i]
	}

	// Check create_runtime
	if fn, ok := funcMap["create_runtime"]; ok {
		if fn.ReturnType == "" {
			t.Error("create_runtime should have a return type")
		}
		if len(fn.Params) < 1 {
			t.Error("create_runtime should have params")
		}
	} else {
		t.Error("missing create_runtime function")
	}

	// Check async function
	if fn, ok := funcMap["fetch_data"]; ok {
		if !fn.IsAsync {
			t.Error("fetch_data should be async")
		}
	} else {
		t.Error("missing fetch_data function")
	}

	// Check unsafe function
	if fn, ok := funcMap["read_raw"]; ok {
		if !fn.IsUnsafe {
			t.Error("read_raw should be unsafe")
		}
	} else {
		t.Error("missing read_raw function")
	}

	// internal_helper should NOT be present (private)
	if _, ok := funcMap["internal_helper"]; ok {
		t.Error("internal_helper should not be present (it is private)")
	}
}

func TestLifetimeHandling(t *testing.T) {
	src := `/// A reference with a lifetime.
pub struct Ref<'a, T: 'a> {
    pub data: &'a T,
}

/// Borrows data with lifetime.
pub fn borrow<'a>(data: &'a [u8]) -> &'a [u8] {
    data
}
`
	analysis := parseFile([]byte(src))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(analysis.Structs))
	}
	if analysis.Structs[0].Name != "Ref" {
		t.Errorf("expected Ref, got %q", analysis.Structs[0].Name)
	}
	// Generics should contain lifetime
	if !strings.Contains(analysis.Structs[0].Generics, "'a") {
		t.Errorf("expected lifetime in generics: %q", analysis.Structs[0].Generics)
	}

	if len(analysis.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(analysis.Functions))
	}
	if analysis.Functions[0].Name != "borrow" {
		t.Errorf("expected borrow, got %q", analysis.Functions[0].Name)
	}
}

func TestDocCommentExtraction(t *testing.T) {
	src := `/// First line of doc.
///
/// More details here.
///
/// # Arguments
///
/// * ` + "`x`" + ` - The x value
pub fn documented(x: i32) -> i32 {
    x + 1
}
`
	analysis := parseFile([]byte(src))
	if analysis == nil || len(analysis.Functions) == 0 {
		t.Fatal("expected function")
	}

	fn := analysis.Functions[0]
	if fn.DocComment == "" {
		t.Fatal("expected doc comment")
	}
	if !strings.Contains(fn.DocComment, "First line") {
		t.Errorf("doc comment missing first line: %q", fn.DocComment)
	}
	if !strings.Contains(fn.DocComment, "More details") {
		t.Errorf("doc comment missing details: %q", fn.DocComment)
	}
}

func TestWrapperDetection(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		isWrapper  bool
		targetName string
		targetKind string
	}{
		{
			name: "simple_delegation",
			src: `pub fn wrapper(x: i32) -> i32 {
    inner(x)
}`,
			isWrapper:  true,
			targetName: "inner",
			targetKind: "function",
		},
		{
			name: "self_method_delegation",
			src: `pub fn get(&self) -> &T {
    self.inner_get()
}`,
			isWrapper:  true,
			targetName: "inner_get",
			targetKind: "self_method",
		},
		{
			name: "not_a_wrapper",
			src: `pub fn complex(x: i32, y: i32) -> i32 {
    let z = x + y;
    z * 2
}`,
			isWrapper: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isWrapper, target, kind := detectWrapper(tt.src)
			if isWrapper != tt.isWrapper {
				t.Errorf("isWrapper: got %v, want %v", isWrapper, tt.isWrapper)
			}
			if tt.isWrapper {
				if target != tt.targetName {
					t.Errorf("targetName: got %q, want %q", target, tt.targetName)
				}
				if kind != tt.targetKind {
					t.Errorf("targetKind: got %q, want %q", kind, tt.targetKind)
				}
			}
		})
	}
}

func TestReExportFollowing(t *testing.T) {
	dir := testFixtureDir(t, map[string]string{
		"lib.rs": sampleReExport + "\n" + `pub struct LocalType { pub x: i32 }`,
		"runtime.rs": `/// The main runtime.
pub struct Runtime {
    pub handle: Handle,
}
`,
		"io.rs": `/// Async read trait.
pub trait AsyncRead {
    fn poll_read(&self) -> usize;
}
`,
	})

	src := New(dir, Config{
		Owner:     "test",
		Repo:      "test",
		CrateName: "test",
	})

	ids, err := src.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	// The re-export of Runtime and Read (alias for AsyncRead) should be present
	foundRuntime := false
	foundRead := false
	for _, id := range ids {
		_, frag := splitFragment(id)
		if frag == "Runtime" {
			foundRuntime = true
		}
		if frag == "Read" {
			foundRead = true
		}
	}

	if !foundRuntime {
		t.Error("expected Runtime to be discoverable (via re-export)")
	}
	if !foundRead {
		t.Error("expected Read (alias for AsyncRead) to be discoverable")
	}
}

func TestDocHiddenFiltering(t *testing.T) {
	analysis := parseFile([]byte(sampleDocHidden))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	// hidden_function should be filtered out, visible_function should remain
	for _, fn := range analysis.Functions {
		if fn.Name == "hidden_function" {
			t.Error("hidden_function should be filtered by #[doc(hidden)]")
		}
	}

	found := false
	for _, fn := range analysis.Functions {
		if fn.Name == "visible_function" {
			found = true
		}
	}
	if !found {
		t.Error("visible_function should be present")
	}
}

func TestTestModuleSkipping(t *testing.T) {
	analysis := parseFile([]byte(sampleTestModule))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	// public_fn should be found
	found := false
	for _, fn := range analysis.Functions {
		if fn.Name == "public_fn" {
			found = true
		}
		if fn.Name == "test_helper" {
			t.Error("test_helper from #[cfg(test)] module should not be discovered")
		}
	}
	if !found {
		t.Error("expected public_fn to be discovered")
	}
}

func TestConstantParsing(t *testing.T) {
	analysis := parseFile([]byte(sampleConstant))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Constants) < 2 {
		t.Fatalf("expected at least 2 constants, got %d", len(analysis.Constants))
	}

	constMap := make(map[string]*constantDef)
	for i := range analysis.Constants {
		constMap[analysis.Constants[i].Name] = &analysis.Constants[i]
	}

	if c, ok := constMap["MAX_BUFFER_SIZE"]; ok {
		if c.Type != "usize" {
			t.Errorf("expected type usize, got %q", c.Type)
		}
		if c.IsStatic {
			t.Error("MAX_BUFFER_SIZE should not be static")
		}
	} else {
		t.Error("missing MAX_BUFFER_SIZE")
	}

	if c, ok := constMap["ALLOCATOR"]; ok {
		if !c.IsStatic {
			t.Error("ALLOCATOR should be static")
		}
	} else {
		t.Error("missing ALLOCATOR")
	}
}

func TestTypeAliasParsing(t *testing.T) {
	analysis := parseFile([]byte(sampleTypeAlias))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.TypeAliases) != 1 {
		t.Fatalf("expected 1 type alias, got %d", len(analysis.TypeAliases))
	}

	ta := analysis.TypeAliases[0]
	if ta.Name != "BoxFuture" {
		t.Errorf("expected BoxFuture, got %q", ta.Name)
	}
	if ta.Generics == "" {
		t.Error("expected generics with lifetime")
	}
}

func TestParseEntity(t *testing.T) {
	dir := testFixtureDir(t, map[string]string{
		"lib.rs": sampleStruct,
	})

	src := New(dir, Config{
		Owner:     "test",
		Repo:      "test",
		CrateName: "test",
	})

	_, err := src.DiscoverEntities(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	libPath := filepath.Join(dir, "src", "lib.rs")
	content, err := os.ReadFile(libPath)
	if err != nil {
		t.Fatal(err)
	}

	entityID := libPath + "#Pool"
	entity, _, err := src.ParseEntity(context.Background(), entityID, content)
	if err != nil {
		t.Fatal(err)
	}

	if entity.Name != "Pool" {
		t.Errorf("expected Pool, got %q", entity.Name)
	}
	if entity.Kind != "struct" {
		t.Errorf("expected kind struct, got %q", entity.Kind)
	}
	if entity.Description == "" {
		t.Error("expected non-empty description")
	}
	if len(entity.Properties) == 0 {
		t.Error("expected properties (struct fields)")
	}
}

func TestMacroParsing(t *testing.T) {
	analysis := parseFile([]byte(sampleMacro))
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Macros) != 1 {
		t.Fatalf("expected 1 macro, got %d", len(analysis.Macros))
	}

	m := analysis.Macros[0]
	if m.Name != "vec" {
		t.Errorf("expected vec, got %q", m.Name)
	}
	if m.DocComment == "" {
		t.Error("expected doc comment for macro")
	}
}

func TestSourceInterface(t *testing.T) {
	src := New("/tmp/test", Config{
		Owner:       "tokio-rs",
		Repo:        "tokio",
		CrateName:   "tokio",
		Version:     "v1.0.0",
		Description: "Async runtime for Rust",
	})

	if src.ID() != "rust/tokio" {
		t.Errorf("unexpected ID: %q", src.ID())
	}

	meta := src.Meta()
	if meta.Language != "rust" {
		t.Errorf("expected language rust, got %q", meta.Language)
	}
	if meta.Name != "tokio" {
		t.Errorf("expected name tokio, got %q", meta.Name)
	}
	if meta.Version != "v1.0.0" {
		t.Errorf("expected version v1.0.0, got %q", meta.Version)
	}
}
