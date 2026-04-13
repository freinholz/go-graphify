package extract

import "github.com/safishamsi/graphify/go/internal/core"

// RegisterBuiltins wires every built-in extractor into the registry. Call once
// at startup (or from init()). Keeps Register explicit so hosted variants can
// register additional extractors (tree-sitter, LLM) on top.
func RegisterBuiltins() {
	Register(PythonExtractor{})
	Register(GoASTExtractor{})
	Register(JSExtractor{})

	// Java-family
	Register(&GenericExtractor{
		Name: "java", Exts: []string{".java"},
		ClassRe:  compile(`(?m)^\s*(?:public|protected|private)?\s*(?:abstract\s+|final\s+|static\s+)*(?:class|interface|enum)\s+([A-Za-z_]\w*)`),
		FuncRe:   compile(`(?m)^\s*(?:public|protected|private|static|final|abstract|synchronized|\s)+[\w<>\[\],\s]+\s+([A-Za-z_]\w*)\s*\([^)]*\)\s*(?:throws\s+[\w,\s]+)?\s*\{`),
		ImportRe: compile(`(?m)^\s*import\s+(?:static\s+)?([\w.*]+)\s*;`),
		CallRe:   compile(`\b([A-Za-z_]\w*)\s*\(`),
	})
	// Kotlin
	Register(&GenericExtractor{
		Name: "kotlin", Exts: []string{".kt", ".kts"},
		ClassRe:  compile(`(?m)^\s*(?:abstract\s+|open\s+|data\s+|sealed\s+)*(?:class|interface|object)\s+([A-Za-z_]\w*)`),
		FuncRe:   compile(`(?m)^\s*(?:suspend\s+|inline\s+|private\s+|public\s+|internal\s+|protected\s+|override\s+)*fun\s+(?:<[^>]+>\s+)?([A-Za-z_]\w*)\s*\(`),
		ImportRe: compile(`(?m)^\s*import\s+([\w.*]+)`),
		CallRe:   compile(`\b([A-Za-z_]\w*)\s*\(`),
	})
	// Scala
	Register(&GenericExtractor{
		Name: "scala", Exts: []string{".scala"},
		ClassRe:  compile(`(?m)^\s*(?:case\s+)?(?:class|object|trait)\s+([A-Za-z_]\w*)`),
		FuncRe:   compile(`(?m)^\s*def\s+([A-Za-z_]\w*)\s*[(\[:]`),
		ImportRe: compile(`(?m)^\s*import\s+([\w.${}_]+)`),
		CallRe:   compile(`\b([A-Za-z_]\w*)\s*\(`),
	})
	// C#
	Register(&GenericExtractor{
		Name: "csharp", Exts: []string{".cs"},
		ClassRe:  compile(`(?m)^\s*(?:public|private|internal|protected|static|abstract|sealed|partial|\s)+(?:class|interface|struct|record)\s+([A-Za-z_]\w*)`),
		FuncRe:   compile(`(?m)^\s*(?:public|private|internal|protected|static|virtual|override|async|\s)+[\w<>\[\],\s?]+\s+([A-Za-z_]\w*)\s*\(`),
		ImportRe: compile(`(?m)^\s*using\s+(?:static\s+)?([\w.]+)\s*;`),
		CallRe:   compile(`\b([A-Za-z_]\w*)\s*\(`),
	})
	// Rust
	Register(&GenericExtractor{
		Name: "rust", Exts: []string{".rs"},
		ClassRe:  compile(`(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?(?:struct|enum|trait|impl)\s+([A-Za-z_]\w*)`),
		FuncRe:   compile(`(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?fn\s+([A-Za-z_]\w*)\s*[<(]`),
		ImportRe: compile(`(?m)^\s*use\s+([\w:]+)`),
		CallRe:   compile(`\b([A-Za-z_]\w*)\s*\(`),
	})
	// C / C++
	Register(&GenericExtractor{
		Name: "c", Exts: []string{".c", ".h"},
		FuncRe:   compile(`(?m)^\s*(?:static\s+|extern\s+|inline\s+)*[\w\s\*]+\s+([A-Za-z_]\w*)\s*\([^)]*\)\s*\{`),
		ImportRe: compile(`(?m)^\s*#include\s+[<"]([^>"]+)[>"]`),
		CallRe:   compile(`\b([A-Za-z_]\w*)\s*\(`),
	})
	Register(&GenericExtractor{
		Name: "cpp", Exts: []string{".cpp", ".cc", ".cxx", ".hpp", ".hh"},
		ClassRe:  compile(`(?m)^\s*(?:class|struct)\s+([A-Za-z_]\w*)`),
		FuncRe:   compile(`(?m)^\s*(?:static\s+|inline\s+|virtual\s+|\s)*[\w\s\*<>:&,]+\s+([A-Za-z_]\w*)\s*\([^)]*\)\s*(?:const\s*)?\{`),
		ImportRe: compile(`(?m)^\s*#include\s+[<"]([^>"]+)[>"]`),
		CallRe:   compile(`\b([A-Za-z_]\w*)\s*\(`),
	})
	// Ruby
	Register(&GenericExtractor{
		Name: "ruby", Exts: []string{".rb"},
		ClassRe:  compile(`(?m)^\s*(?:class|module)\s+([A-Za-z_]\w*)`),
		FuncRe:   compile(`(?m)^\s*def\s+(?:self\.)?([A-Za-z_]\w*[!?=]?)`),
		ImportRe: compile(`(?m)^\s*require(?:_relative)?\s+['"]([^'"]+)['"]`),
	})
	// PHP
	Register(&GenericExtractor{
		Name: "php", Exts: []string{".php"},
		ClassRe:  compile(`(?m)^\s*(?:abstract\s+|final\s+)?(?:class|interface|trait)\s+([A-Za-z_]\w*)`),
		FuncRe:   compile(`(?m)^\s*(?:public|private|protected|static|\s)*function\s+([A-Za-z_]\w*)\s*\(`),
		ImportRe: compile(`(?m)^\s*use\s+([\w\\]+)`),
		CallRe:   compile(`\b([A-Za-z_]\w*)\s*\(`),
	})
	// Swift
	Register(&GenericExtractor{
		Name: "swift", Exts: []string{".swift"},
		ClassRe:  compile(`(?m)^\s*(?:public\s+|private\s+|internal\s+|open\s+|fileprivate\s+)?(?:class|struct|enum|protocol|actor)\s+([A-Za-z_]\w*)`),
		FuncRe:   compile(`(?m)^\s*(?:public\s+|private\s+|internal\s+|static\s+|override\s+|mutating\s+|func\s+)+([A-Za-z_]\w*)\s*[<(]`),
		ImportRe: compile(`(?m)^\s*import\s+([\w.]+)`),
		CallRe:   compile(`\b([A-Za-z_]\w*)\s*\(`),
	})
	// Lua
	Register(&GenericExtractor{
		Name: "lua", Exts: []string{".lua"},
		FuncRe:   compile(`(?m)^\s*(?:local\s+)?function\s+([A-Za-z_][\w.:]*)`),
		ImportRe: compile(`require\s*\(?\s*['"]([^'"]+)['"]`),
	})
	// Elixir
	Register(&GenericExtractor{
		Name: "elixir", Exts: []string{".ex", ".exs"},
		ClassRe:  compile(`(?m)^\s*defmodule\s+([\w.]+)`),
		FuncRe:   compile(`(?m)^\s*defp?\s+([A-Za-z_]\w*[!?]?)\s*[(\s]`),
		ImportRe: compile(`(?m)^\s*(?:import|alias|use)\s+([\w.]+)`),
	})
	// Julia
	Register(&GenericExtractor{
		Name: "julia", Exts: []string{".jl"},
		ClassRe:  compile(`(?m)^\s*(?:mutable\s+)?struct\s+([A-Za-z_]\w*)`),
		FuncRe:   compile(`(?m)^\s*function\s+([A-Za-z_]\w*)`),
		ImportRe: compile(`(?m)^\s*(?:using|import|include)\s+\(?["']?([\w.]+)["']?\)?`),
	})
	// Zig
	Register(&GenericExtractor{
		Name: "zig", Exts: []string{".zig"},
		ClassRe:  compile(`(?m)^\s*(?:pub\s+)?(?:const|var)\s+([A-Za-z_]\w*)\s*=\s*(?:packed\s+|extern\s+)?struct`),
		FuncRe:   compile(`(?m)^\s*(?:pub\s+)?fn\s+([A-Za-z_]\w*)\s*\(`),
		ImportRe: compile(`@import\s*\(\s*"([^"]+)"\s*\)`),
	})
	// PowerShell
	Register(&GenericExtractor{
		Name: "powershell", Exts: []string{".ps1"},
		FuncRe:   compile(`(?m)^\s*function\s+(?:global:|local:|script:|private:)?([A-Za-z_][\w-]*)`),
		ImportRe: compile(`(?m)^\s*(?:Import-Module|using module|using namespace)\s+([\w.]+)`),
	})
	// Objective-C
	Register(&GenericExtractor{
		Name: "objc", Exts: []string{".m", ".mm"},
		ClassRe:  compile(`(?m)^\s*@(?:interface|implementation|protocol)\s+([A-Za-z_]\w*)`),
		ImportRe: compile(`(?m)^\s*#import\s+[<"]([^>"]+)[>"]`),
	})
}

var _ = core.FileCode // keep import used when stripping extractors
