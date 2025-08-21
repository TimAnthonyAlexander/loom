package lang_graph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// GoGraphBuilder builds import graphs for Go files
type GoGraphBuilder struct {
	root       string
	graph      *shared.Graph
	moduleInfo *GoModuleInfo
	fileSet    *token.FileSet
}

// GoModuleInfo contains information about the Go module
type GoModuleInfo struct {
	ModulePath string
	RootDir    string
}

// NewGoGraphBuilder creates a new Go graph builder
func NewGoGraphBuilder(root string, graph *shared.Graph) *GoGraphBuilder {
	return &GoGraphBuilder{
		root:    root,
		graph:   graph,
		fileSet: token.NewFileSet(),
	}
}

// Build processes Go files and builds the import graph
func (g *GoGraphBuilder) Build(files []string) error {
	// Parse go.mod to understand module structure
	g.parseGoMod()

	// First, add all files as vertices to ensure they appear in the graph
	for _, file := range files {
		normalizedFile := shared.NormalizePath(file)
		g.graph.Vertices[normalizedFile] = true
	}

	for _, file := range files {
		if err := g.processFile(file); err != nil {
			// Log error but continue processing other files
			continue
		}
	}

	// Add package-level edges (files in same package are related)
	g.addPackageEdges(files)

	return nil
}

// parseGoMod parses go.mod file to extract module information
func (g *GoGraphBuilder) parseGoMod() {
	goModPath := filepath.Join(g.root, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module"))
			g.moduleInfo = &GoModuleInfo{
				ModulePath: modulePath,
				RootDir:    g.root,
			}
			break
		}
	}
}

// processFile processes a single Go file
func (g *GoGraphBuilder) processFile(filePath string) error {
	fullPath := filepath.Join(g.root, filePath)

	// Parse the Go file with imports only
	src, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}

	file, err := parser.ParseFile(g.fileSet, fullPath, src, parser.ImportsOnly)
	if err != nil {
		return err
	}

	// Extract imports
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// Check if this is a local import (within our module)
		if g.isLocalImport(importPath) {
			targetFiles := g.resolveImportToFiles(importPath)
			for _, targetFile := range targetFiles {
				// Add edge from current file to imported file
				g.graph.AddEdge(filePath, targetFile, 1.0)
			}
		}
	}

	return nil
}

// isLocalImport checks if an import path refers to a local package
func (g *GoGraphBuilder) isLocalImport(importPath string) bool {
	if g.moduleInfo == nil {
		return false
	}

	// Local imports start with the module path
	return strings.HasPrefix(importPath, g.moduleInfo.ModulePath)
}

// resolveImportToFiles resolves an import path to actual Go files
func (g *GoGraphBuilder) resolveImportToFiles(importPath string) []string {
	if g.moduleInfo == nil {
		return nil
	}

	// Convert import path to directory path
	relativePath := strings.TrimPrefix(importPath, g.moduleInfo.ModulePath)
	if relativePath == "" {
		// Importing the root module
		relativePath = "."
	} else {
		// Remove leading slash if present
		relativePath = strings.TrimPrefix(relativePath, "/")
	}

	packageDir := filepath.Join(g.root, relativePath)

	// Find all Go files in the package directory
	var files []string
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			relativeFile := filepath.Join(relativePath, entry.Name())
			// Normalize the path
			if relativeFile == entry.Name() && relativePath == "." {
				files = append(files, entry.Name())
			} else {
				files = append(files, relativeFile)
			}
		}
	}

	return files
}

// addPackageEdges adds weak edges between files in the same package
func (g *GoGraphBuilder) addPackageEdges(files []string) {
	// Group files by package (directory)
	packages := make(map[string][]string)

	for _, file := range files {
		packageDir := filepath.Dir(file)
		if packageDir == "." {
			packageDir = ""
		}
		packages[packageDir] = append(packages[packageDir], file)
	}

	// Add weak edges between files in the same package
	for _, packageFiles := range packages {
		if len(packageFiles) <= 1 {
			continue
		}

		// Add bidirectional weak edges between all files in the package
		for i, file1 := range packageFiles {
			for j, file2 := range packageFiles {
				if i != j {
					// Use a lower weight for package-level relationships
					g.graph.AddEdge(file1, file2, 0.3)
				}
			}
		}
	}
}

// GetPackageFromFile determines the package name from a Go file
func (g *GoGraphBuilder) GetPackageFromFile(filePath string) string {
	fullPath := filepath.Join(g.root, filePath)

	file, err := parser.ParseFile(g.fileSet, fullPath, nil, parser.PackageClauseOnly)
	if err != nil {
		return ""
	}

	if file.Name != nil {
		return file.Name.Name
	}

	return ""
}

// AnalyzeMainFunctions finds main functions in Go files
func (g *GoGraphBuilder) AnalyzeMainFunctions(files []string) []string {
	var mainFiles []string

	for _, file := range files {
		if g.hasMainFunction(file) {
			mainFiles = append(mainFiles, file)
		}
	}

	return mainFiles
}

// hasMainFunction checks if a Go file contains a main function
func (g *GoGraphBuilder) hasMainFunction(filePath string) bool {
	fullPath := filepath.Join(g.root, filePath)

	file, err := parser.ParseFile(g.fileSet, fullPath, nil, 0)
	if err != nil {
		return false
	}

	// Check if this is package main
	if file.Name == nil || file.Name.Name != "main" {
		return false
	}

	// Look for main function
	for _, decl := range file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if funcDecl.Name.Name == "main" && funcDecl.Recv == nil {
				return true
			}
		}
	}

	return false
}

// ExtractGoStructsAndInterfaces extracts struct and interface definitions
func (g *GoGraphBuilder) ExtractGoStructsAndInterfaces(filePath string) ([]string, []string) {
	fullPath := filepath.Join(g.root, filePath)

	file, err := parser.ParseFile(g.fileSet, fullPath, nil, 0)
	if err != nil {
		return nil, nil
	}

	var structs, interfaces []string

	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					switch typeSpec.Type.(type) {
					case *ast.StructType:
						structs = append(structs, typeSpec.Name.Name)
					case *ast.InterfaceType:
						interfaces = append(interfaces, typeSpec.Name.Name)
					}
				}
			}
		}
	}

	return structs, interfaces
}
