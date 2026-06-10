package typescript

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Parser Tests ---

func TestParseClass(t *testing.T) {
	src := []byte(`
/** A sample class. */
export class MyService<T extends Base> extends BaseService implements Serializable {
    /** The name property. */
    public name: string;
    private count: number;
    protected readonly items: T[];

    /** Constructor. */
    constructor(private readonly config: Config) {
        super();
    }

    /** Get the data. */
    async getData(id: string, options?: RequestOptions): Promise<T> {
        return this.fetch(id);
    }

    static create(): MyService<any> {
        return new MyService({});
    }
}
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}

	if len(analysis.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(analysis.Classes))
	}

	cls := analysis.Classes[0]
	if cls.Name != "MyService" {
		t.Errorf("expected class name MyService, got %q", cls.Name)
	}
	if !cls.Exported {
		t.Error("expected class to be exported")
	}
	if cls.Extends != "BaseService" {
		t.Errorf("expected extends BaseService, got %q", cls.Extends)
	}
	if len(cls.Implements) == 0 || cls.Implements[0] != "Serializable" {
		t.Errorf("expected implements Serializable, got %v", cls.Implements)
	}
	if !strings.Contains(cls.TypeParams, "T") {
		t.Errorf("expected type params to contain T, got %q", cls.TypeParams)
	}
	if !strings.Contains(cls.DocComment, "A sample class") {
		t.Errorf("expected doc comment to contain 'A sample class', got %q", cls.DocComment)
	}

	// Check properties
	if len(cls.Properties) < 2 {
		t.Fatalf("expected at least 2 properties, got %d", len(cls.Properties))
	}

	// Check methods
	if len(cls.Methods) < 2 {
		t.Fatalf("expected at least 2 methods, got %d", len(cls.Methods))
	}

	// Find getData method
	var getDataMethod *methodDef
	for i := range cls.Methods {
		if cls.Methods[i].Name == "getData" {
			getDataMethod = &cls.Methods[i]
			break
		}
	}
	if getDataMethod == nil {
		t.Fatal("method getData not found")
	}
	if !getDataMethod.Async {
		t.Error("expected getData to be async")
	}
	if len(getDataMethod.Params) < 1 {
		t.Error("expected getData to have params")
	}
	if !strings.Contains(getDataMethod.DocComment, "Get the data") {
		t.Errorf("expected doc comment, got %q", getDataMethod.DocComment)
	}
}

func TestParseAbstractClass(t *testing.T) {
	src := []byte(`
/** Abstract base class. */
export abstract class AbstractHandler {
    abstract handle(request: Request): Response;

    protected log(msg: string): void {
        console.log(msg);
    }
}
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(analysis.Classes))
	}

	cls := analysis.Classes[0]
	if cls.Name != "AbstractHandler" {
		t.Errorf("expected AbstractHandler, got %q", cls.Name)
	}
	if !cls.Abstract {
		t.Error("expected class to be abstract")
	}
}

func TestParseInterface(t *testing.T) {
	src := []byte(`
/** Configuration interface. */
export interface Config<T = any> extends BaseConfig, Serializable {
    /** The host name. */
    readonly host: string;
    port?: number;
    data: T;

    /** Initialize the config. */
    init(options: InitOptions): void;
    validate(): boolean;
}
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(analysis.Interfaces))
	}

	iface := analysis.Interfaces[0]
	if iface.Name != "Config" {
		t.Errorf("expected Config, got %q", iface.Name)
	}
	if !iface.Exported {
		t.Error("expected interface to be exported")
	}
	if !strings.Contains(iface.TypeParams, "T") {
		t.Errorf("expected type params to contain T, got %q", iface.TypeParams)
	}
	if len(iface.Extends) < 1 {
		t.Error("expected extends clause")
	}
	if !strings.Contains(iface.DocComment, "Configuration interface") {
		t.Errorf("expected doc comment, got %q", iface.DocComment)
	}

	// Check properties
	if len(iface.Properties) < 2 {
		t.Fatalf("expected at least 2 properties, got %d", len(iface.Properties))
	}

	// Check methods
	if len(iface.Methods) < 1 {
		t.Fatalf("expected at least 1 method, got %d", len(iface.Methods))
	}
}

func TestParseTypeAlias(t *testing.T) {
	src := []byte(`
/** A union type for results. */
export type Result<T, E = Error> = Success<T> | Failure<E>;

/** Mapped type for partial properties. */
export type DeepPartial<T> = {
    [P in keyof T]?: T[P] extends object ? DeepPartial<T[P]> : T[P];
};
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.TypeAliases) < 1 {
		t.Fatalf("expected at least 1 type alias, got %d", len(analysis.TypeAliases))
	}

	ta := analysis.TypeAliases[0]
	if ta.Name != "Result" {
		t.Errorf("expected Result, got %q", ta.Name)
	}
	if !ta.Exported {
		t.Error("expected type alias to be exported")
	}
	if !strings.Contains(ta.TypeParams, "T") {
		t.Errorf("expected type params to contain T, got %q", ta.TypeParams)
	}
	if !strings.Contains(ta.DocComment, "union type for results") {
		t.Errorf("expected doc comment, got %q", ta.DocComment)
	}
}

func TestParseEnum(t *testing.T) {
	src := []byte(`
/** Status codes. */
export enum Status {
    Active = "ACTIVE",
    Inactive = "INACTIVE",
    Pending = "PENDING"
}

export const enum Direction {
    Up,
    Down,
    Left,
    Right
}
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Enums) < 1 {
		t.Fatalf("expected at least 1 enum, got %d", len(analysis.Enums))
	}

	en := analysis.Enums[0]
	if en.Name != "Status" {
		t.Errorf("expected Status, got %q", en.Name)
	}
	if !en.Exported {
		t.Error("expected enum to be exported")
	}
	if len(en.Members) < 3 {
		t.Errorf("expected at least 3 members, got %d", len(en.Members))
	}
	if !strings.Contains(en.DocComment, "Status codes") {
		t.Errorf("expected doc comment, got %q", en.DocComment)
	}
}

func TestParseFunction(t *testing.T) {
	src := []byte(`
/**
 * Creates a new instance.
 * @param {string} name - The name
 * @param {number} [count] - Optional count
 * @returns {Instance} The new instance
 */
export function createInstance<T extends Base>(name: string, count?: number): Instance<T> {
    return new Instance(name, count);
}

export async function fetchData(url: string, ...headers: string[]): Promise<Response> {
    return await fetch(url, { headers });
}
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Functions) < 1 {
		t.Fatalf("expected at least 1 function, got %d", len(analysis.Functions))
	}

	fn := analysis.Functions[0]
	if fn.Name != "createInstance" {
		t.Errorf("expected createInstance, got %q", fn.Name)
	}
	if !fn.Exported {
		t.Error("expected function to be exported")
	}
	if !strings.Contains(fn.TypeParams, "T") {
		t.Errorf("expected type params to contain T, got %q", fn.TypeParams)
	}
	if len(fn.Params) < 1 {
		t.Fatalf("expected params, got %d", len(fn.Params))
	}
	if !strings.Contains(fn.DocComment, "Creates a new instance") {
		t.Errorf("expected doc comment, got %q", fn.DocComment)
	}
}

func TestParseExportedArrowFunction(t *testing.T) {
	src := []byte(`
/** Adds two numbers. */
export const add = (a: number, b: number): number => a + b;
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Functions) < 1 {
		t.Fatalf("expected at least 1 function, got %d", len(analysis.Functions))
	}

	fn := analysis.Functions[0]
	if fn.Name != "add" {
		t.Errorf("expected add, got %q", fn.Name)
	}
	if !fn.Exported {
		t.Error("expected function to be exported")
	}
}

func TestParseNamespace(t *testing.T) {
	src := []byte(`
/** Utility namespace. */
export namespace Utils {
    export function helper(): void {}
}
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Namespaces) < 1 {
		t.Fatalf("expected at least 1 namespace, got %d", len(analysis.Namespaces))
	}

	ns := analysis.Namespaces[0]
	if ns.Name != "Utils" {
		t.Errorf("expected Utils, got %q", ns.Name)
	}
	if !ns.Exported {
		t.Error("expected namespace to be exported")
	}
}

func TestParseReExports(t *testing.T) {
	src := []byte(`
export { Observable } from './observable';
export { Subject, BehaviorSubject } from './subjects';
export type { Config } from './config';
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.ReExports) < 1 {
		t.Fatalf("expected at least 1 re-export, got %d", len(analysis.ReExports))
	}
}

func TestParseDeclarationFile(t *testing.T) {
	src := []byte(`
declare module 'mylib' {
    export interface Options {
        verbose?: boolean;
        timeout: number;
    }

    export function configure(opts: Options): void;
    export class Client {
        constructor(url: string);
        get(path: string): Promise<any>;
    }
}
`)

	analysis := parseFile(src, "test.d.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	// Declaration files should be parseable
	// The entities will be inside the module declaration
	if len(analysis.Namespaces) < 1 {
		t.Logf("NOTE: Ambient module declarations parsed as namespaces; got %d", len(analysis.Namespaces))
	}
}

func TestParseGenericConstraints(t *testing.T) {
	src := []byte(`
export function merge<T extends object, U extends object>(target: T, source: U): T & U {
    return Object.assign(target, source);
}

export interface Repository<T extends Entity, K extends string | number = string> {
    find(id: K): Promise<T | null>;
    save(entity: T): Promise<T>;
}
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Functions) < 1 {
		t.Fatal("expected at least 1 function")
	}
	if len(analysis.Interfaces) < 1 {
		t.Fatal("expected at least 1 interface")
	}

	fn := analysis.Functions[0]
	if !strings.Contains(fn.TypeParams, "extends") {
		t.Errorf("expected type params with constraints, got %q", fn.TypeParams)
	}
}

func TestParseDecorators(t *testing.T) {
	src := []byte(`
import { Injectable, Inject } from '@angular/core';

@Injectable()
export class UserService {
    constructor(@Inject('CONFIG') private config: any) {}

    @Log()
    getUser(id: string): User {
        return this.db.find(id);
    }
}
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	if len(analysis.Classes) < 1 {
		t.Fatal("expected at least 1 class")
	}
	// Decorators should be extracted if grammar supports them
}

// --- Wrapper Detection Tests ---

func TestDetectWrapperSimpleReturn(t *testing.T) {
	src := `function wrapper(x: number): number { return target(x); }`
	isWrapper, target, kind := detectWrapper(src, emptyIndex())
	if !isWrapper {
		t.Error("expected wrapper detection")
	}
	if target != "target" {
		t.Errorf("expected target 'target', got %q", target)
	}
	if kind != "function" {
		t.Errorf("expected kind 'function', got %q", kind)
	}
}

func TestDetectWrapperMethodCall(t *testing.T) {
	src := `function proxy(id: string): Promise<User> { return this.service.getUser(id); }`
	isWrapper, target, kind := detectWrapper(src, emptyIndex())
	if !isWrapper {
		t.Error("expected wrapper detection")
	}
	if kind != "method" {
		t.Errorf("expected kind 'method', got %q", kind)
	}
	_ = target
}

func TestDetectWrapperArrowFunction(t *testing.T) {
	src := `const foo = (x: number) => bar(x);`
	isWrapper, target, kind := detectWrapper(src, emptyIndex())
	if !isWrapper {
		t.Error("expected wrapper detection")
	}
	if target != "bar" {
		t.Errorf("expected target 'bar', got %q", target)
	}
	if kind != "function" {
		t.Errorf("expected kind 'function', got %q", kind)
	}
}

func TestDetectNonWrapper(t *testing.T) {
	src := `function process(x: number): number {
    const y = transform(x);
    return finalize(y);
}`
	isWrapper, _, _ := detectWrapper(src, emptyIndex())
	if isWrapper {
		t.Error("expected non-wrapper")
	}
}

// --- Discovery Tests ---

func TestDiscovery(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a TypeScript source file
	content := []byte(`
/** A main service class. */
export class MainService {
    getName(): string {
        return "main";
    }
}

/** Helper function. */
export function helper(x: number): number {
    return x * 2;
}

/** Internal type. */
export type Config = {
    host: string;
    port: number;
};

export enum Status {
    Active,
    Inactive
}
`)
	if err := os.WriteFile(filepath.Join(srcDir, "main.ts"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a test file (should be skipped)
	testContent := []byte(`
export class TestHelper {}
`)
	if err := os.WriteFile(filepath.Join(srcDir, "main.test.ts"), testContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create node_modules (should be skipped)
	nodeModDir := filepath.Join(srcDir, "node_modules", "somelib")
	if err := os.MkdirAll(nodeModDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeModDir, "index.ts"), []byte(`export class Lib {}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		LibraryID:  "typescript/test",
		Name:       "Test Library",
		SourceDirs: []string{"src/"},
	}

	idx, err := buildCodebaseIndex(tmpDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	ids := idx.buildEntityList()
	if len(ids) == 0 {
		t.Fatal("expected discovered entities")
	}

	// Verify MainService was found
	foundClass := false
	foundFunc := false
	foundType := false
	foundEnum := false
	for _, id := range ids {
		if strings.HasSuffix(id, "#MainService") {
			foundClass = true
		}
		if strings.HasSuffix(id, "#helper") {
			foundFunc = true
		}
		if strings.HasSuffix(id, "#Config") {
			foundType = true
		}
		if strings.HasSuffix(id, "#Status") {
			foundEnum = true
		}
	}
	if !foundClass {
		t.Error("expected MainService to be discovered")
	}
	if !foundFunc {
		t.Error("expected helper to be discovered")
	}
	if !foundType {
		t.Error("expected Config to be discovered")
	}
	if !foundEnum {
		t.Error("expected Status to be discovered")
	}

	// Verify test file was skipped
	for _, id := range ids {
		if strings.Contains(id, "TestHelper") {
			t.Error("test file should be skipped")
		}
	}

	// Verify node_modules was skipped
	for _, id := range ids {
		if strings.Contains(id, "node_modules") {
			t.Error("node_modules should be skipped")
		}
	}
}

// --- Source Interface Tests ---

func TestSourceInterface(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := []byte(`
/**
 * A user service for managing users.
 */
export class UserService {
    /** List of users. */
    private users: User[] = [];

    /**
     * Get a user by ID.
     * @param {string} id - The user ID
     * @returns {User} The user
     */
    getUser(id: string): User {
        return this.users.find(u => u.id === id);
    }

    /**
     * Create a new user.
     * @param {string} name - The user name
     * @param {string} email - The user email
     * @returns {User} The created user
     */
    createUser(name: string, email: string): User {
        const user = { id: genId(), name, email };
        this.users.push(user);
        return user;
    }
}

/**
 * Format a user's display name.
 * @param {User} user - The user object
 * @returns {string} The formatted name
 */
export function formatUser(user: User): string {
    return user.name + " <" + user.email + ">";
}

export interface User {
    id: string;
    name: string;
    email: string;
}
`)
	filePath := filepath.Join(srcDir, "user.ts")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		LibraryID:   "typescript/test",
		Name:        "Test",
		Description: "Test library",
		SourceURL:   "https://github.com/test/test",
		Owner:       "test",
		Repo:        "test",
		SourceDirs:  []string{"src/"},
	}

	src := New(cfg, tmpDir, WithRef("v1.0.0"))

	// Test ID
	if id := src.ID(); id != "typescript/test" {
		t.Errorf("expected ID 'typescript/test', got %q", id)
	}

	// Test Meta
	meta := src.Meta()
	if meta.Language != "typescript" {
		t.Errorf("expected language 'typescript', got %q", meta.Language)
	}
	if meta.Version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got %q", meta.Version)
	}

	// Test DiscoverEntities
	ctx := context.Background()
	ids, err := src.DiscoverEntities(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		t.Fatal("expected discovered entities")
	}

	// Find UserService entity
	var userServiceID string
	for _, id := range ids {
		if strings.HasSuffix(id, "#UserService") {
			userServiceID = id
			break
		}
	}
	if userServiceID == "" {
		t.Fatal("UserService not discovered")
	}

	// Test ParseEntity
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	entity, methodIDs, err := src.ParseEntity(ctx, userServiceID, fileContent)
	if err != nil {
		t.Fatal(err)
	}
	if entity.Name != "UserService" {
		t.Errorf("expected UserService, got %q", entity.Name)
	}
	if entity.Kind != "class" {
		t.Errorf("expected kind 'class', got %q", entity.Kind)
	}
	if !strings.Contains(entity.Description, "user service for managing users") {
		t.Errorf("expected description, got %q", entity.Description)
	}
	if len(methodIDs) < 2 {
		t.Errorf("expected at least 2 method IDs, got %d", len(methodIDs))
	}

	// Test ParseMethod
	var getUserMethodID string
	for _, mid := range methodIDs {
		if strings.Contains(mid, ".getUser") {
			getUserMethodID = mid
			break
		}
	}
	if getUserMethodID == "" {
		t.Fatal("getUser method ID not found")
	}

	method, err := src.ParseMethod(ctx, getUserMethodID, fileContent)
	if err != nil {
		t.Fatal(err)
	}
	if method.Name != "getUser" {
		t.Errorf("expected getUser, got %q", method.Name)
	}
	if len(method.Parameters) < 1 {
		t.Error("expected parameters")
	}
	if !strings.Contains(method.Description, "Get a user by ID") {
		t.Errorf("expected description, got %q", method.Description)
	}

	// Test ParseSourceCode
	code, err := src.ParseSourceCode(userServiceID, fileContent)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "class UserService") {
		t.Error("expected source code to contain class definition")
	}
}

func TestOverloadedFunctions(t *testing.T) {
	src := []byte(`
export function parse(input: string): number;
export function parse(input: number): string;
export function parse(input: string | number): string | number {
    if (typeof input === 'string') return parseInt(input);
    return input.toString();
}
`)

	analysis := parseFile(src, "test.ts")
	if analysis == nil {
		t.Fatal("parseFile returned nil")
	}
	// The implementation should be captured (overloads are signatures only)
	if len(analysis.Functions) < 1 {
		t.Fatalf("expected at least 1 function, got %d", len(analysis.Functions))
	}

	// At least one should have the name "parse"
	found := false
	for _, fn := range analysis.Functions {
		if fn.Name == "parse" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find function named 'parse'")
	}
}

func TestDeclarationFileParsing(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := []byte(`
/** The main API interface. */
export interface API {
    /** Fetch resources. */
    fetch(url: string): Promise<Response>;
    /** Configure the API. */
    configure(opts: Options): void;
}

/** Options type. */
export type Options = {
    baseURL: string;
    timeout?: number;
};

/** Create a client. */
export declare function createClient(opts: Options): API;
`)
	filePath := filepath.Join(srcDir, "types.d.ts")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		LibraryID:               "typescript/test",
		Name:                    "Test",
		SourceDirs:              []string{"src/"},
		IncludeDeclarationFiles: true,
	}

	idx, err := buildCodebaseIndex(tmpDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	ids := idx.buildEntityList()
	if len(ids) == 0 {
		t.Fatal("expected entities from .d.ts file")
	}

	foundAPI := false
	foundOptions := false
	for _, id := range ids {
		if strings.HasSuffix(id, "#API") {
			foundAPI = true
		}
		if strings.HasSuffix(id, "#Options") {
			foundOptions = true
		}
	}
	if !foundAPI {
		t.Error("expected API interface to be discovered")
	}
	if !foundOptions {
		t.Error("expected Options type to be discovered")
	}
}
