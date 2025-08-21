package signals

import (
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// CodegenExtractor extracts information from code generation configurations
type CodegenExtractor struct {
	root string
}

// NewCodegenExtractor creates a new codegen extractor
func NewCodegenExtractor(root string) *CodegenExtractor {
	return &CodegenExtractor{root: root}
}

// Extract processes files and detects code generation configurations
func (c *CodegenExtractor) Extract(files []*shared.FileInfo, existing *shared.SignalData) {
	for _, file := range files {
		if c.isCodegenFile(file.Path, file.Basename) {
			c.processCodegenFile(file, existing)
		}
	}

	// Also scan for common codegen patterns in directories
	c.scanForCodegenPatterns(files, existing)
}

// isCodegenFile checks if a file is related to code generation
func (c *CodegenExtractor) isCodegenFile(path, basename string) bool {
	lowerPath := strings.ToLower(path)
	lowerBasename := strings.ToLower(basename)

	codegenFiles := map[string]bool{
		// OpenAPI/Swagger
		"openapi.json": true,
		"openapi.yaml": true,
		"openapi.yml":  true,
		"swagger.json": true,
		"swagger.yaml": true,
		"swagger.yml":  true,
		"api.json":     true,
		"api.yaml":     true,
		"api.yml":      true,

		// Prisma
		"schema.prisma": true,

		// Protocol Buffers
		"buf.yaml":      true,
		"buf.yml":       true,
		"buf.gen.yaml":  true,
		"buf.gen.yml":   true,
		"buf.work.yaml": true,
		"buf.work.yml":  true,

		// SQL code generation
		"sqlc.yaml": true,
		"sqlc.yml":  true,

		// GraphQL
		"schema.graphql":    true,
		"schema.gql":        true,
		"codegen.yml":       true,
		"codegen.yaml":      true,
		"graphql.config.js": true,

		// gRPC
		"proto.lock": true,

		// Other
		"generate.go": true,
	}

	if codegenFiles[lowerBasename] {
		return true
	}

	// Check for patterns in path
	codegenPatterns := []string{
		"/prisma/",
		"/proto/",
		"/graphql/",
		"/migrations/",
		"/schema/",
		"/api/",
		"/openapi/",
		"/swagger/",
	}

	for _, pattern := range codegenPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	// Check file extensions
	ext := strings.ToLower(filepath.Ext(basename))
	codegenExtensions := map[string]bool{
		".proto":   true,
		".graphql": true,
		".gql":     true,
	}

	return codegenExtensions[ext]
}

// processCodegenFile processes a single codegen file
func (c *CodegenExtractor) processCodegenFile(file *shared.FileInfo, signals *shared.SignalData) {
	tool := c.detectCodegenTool(file.Path, file.Basename)

	spec := shared.CodegenSpec{
		Tool:  tool,
		Paths: []string{file.Path},
	}

	// Check if we already have this tool, and if so, add to its paths
	found := false
	for i, existing := range signals.Codegen {
		if existing.Tool == tool {
			// Add path if not already present
			pathExists := false
			for _, existingPath := range existing.Paths {
				if existingPath == file.Path {
					pathExists = true
					break
				}
			}
			if !pathExists {
				signals.Codegen[i].Paths = append(signals.Codegen[i].Paths, file.Path)
			}
			found = true
			break
		}
	}

	if !found {
		signals.Codegen = append(signals.Codegen, spec)
	}
}

// detectCodegenTool determines the code generation tool from file
func (c *CodegenExtractor) detectCodegenTool(path, basename string) string {
	lowerPath := strings.ToLower(path)
	lowerBasename := strings.ToLower(basename)

	// Direct mappings
	toolMap := map[string]string{
		"schema.prisma":     "prisma",
		"buf.yaml":          "buf",
		"buf.yml":           "buf",
		"buf.gen.yaml":      "buf",
		"buf.gen.yml":       "buf",
		"buf.work.yaml":     "buf",
		"buf.work.yml":      "buf",
		"sqlc.yaml":         "sqlc",
		"sqlc.yml":          "sqlc",
		"codegen.yml":       "graphql-codegen",
		"codegen.yaml":      "graphql-codegen",
		"graphql.config.js": "graphql",
		"generate.go":       "go-generate",
		"proto.lock":        "protobuf",
	}

	if tool, exists := toolMap[lowerBasename]; exists {
		return tool
	}

	// Pattern matching
	if strings.Contains(lowerBasename, "openapi") || strings.Contains(lowerBasename, "swagger") {
		return "openapi"
	}

	if strings.Contains(lowerBasename, "graphql") || strings.Contains(lowerBasename, "gql") {
		return "graphql"
	}

	if strings.HasSuffix(lowerBasename, ".proto") {
		return "protobuf"
	}

	if strings.Contains(lowerPath, "/prisma/") {
		return "prisma"
	}

	if strings.Contains(lowerPath, "/migrations/") {
		// Determine migration tool based on context
		if strings.Contains(lowerPath, "laravel") || strings.Contains(lowerPath, "/database/migrations/") {
			return "laravel-migrations"
		}
		if strings.Contains(lowerPath, "django") {
			return "django-migrations"
		}
		if strings.Contains(lowerPath, "rails") {
			return "rails-migrations"
		}
		return "migrations"
	}

	if strings.Contains(lowerPath, "/proto/") {
		return "protobuf"
	}

	return "unknown"
}

// scanForCodegenPatterns scans for common codegen directory patterns
func (c *CodegenExtractor) scanForCodegenPatterns(files []*shared.FileInfo, signals *shared.SignalData) {
	// Look for migration directories
	c.scanForMigrations(files, signals)

	// Look for proto directories
	c.scanForProtoFiles(files, signals)

	// Look for generated code patterns
	c.scanForGeneratedCode(files, signals)
}

// scanForMigrations scans for database migration files
func (c *CodegenExtractor) scanForMigrations(files []*shared.FileInfo, signals *shared.SignalData) {
	migrationPaths := make(map[string][]string)

	for _, file := range files {
		lowerPath := strings.ToLower(file.Path)

		// Look for migration directories
		if strings.Contains(lowerPath, "/migrations/") || strings.Contains(lowerPath, "/migrate/") {
			tool := "migrations"

			// Determine specific migration tool
			if strings.Contains(lowerPath, "/database/migrations/") || strings.Contains(lowerPath, "laravel") {
				tool = "laravel-migrations"
			} else if strings.Contains(lowerPath, "django") {
				tool = "django-migrations"
			} else if strings.Contains(lowerPath, "rails") {
				tool = "rails-migrations"
			} else if strings.Contains(lowerPath, "prisma") {
				tool = "prisma"
			}

			migrationPaths[tool] = append(migrationPaths[tool], file.Path)
		}
	}

	// Add migration specs
	for tool, paths := range migrationPaths {
		if len(paths) > 0 {
			// Check if we already have this tool
			found := false
			for i, existing := range signals.Codegen {
				if existing.Tool == tool {
					signals.Codegen[i].Paths = append(signals.Codegen[i].Paths, paths...)
					found = true
					break
				}
			}

			if !found {
				signals.Codegen = append(signals.Codegen, shared.CodegenSpec{
					Tool:  tool,
					Paths: paths,
				})
			}
		}
	}
}

// scanForProtoFiles scans for Protocol Buffer files
func (c *CodegenExtractor) scanForProtoFiles(files []*shared.FileInfo, signals *shared.SignalData) {
	var protoPaths []string

	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file.Path), ".proto") {
			protoPaths = append(protoPaths, file.Path)
		}
	}

	if len(protoPaths) > 0 {
		// Check if we already have protobuf tool
		found := false
		for i, existing := range signals.Codegen {
			if existing.Tool == "protobuf" {
				signals.Codegen[i].Paths = append(signals.Codegen[i].Paths, protoPaths...)
				found = true
				break
			}
		}

		if !found {
			signals.Codegen = append(signals.Codegen, shared.CodegenSpec{
				Tool:  "protobuf",
				Paths: protoPaths,
			})
		}
	}
}

// scanForGeneratedCode scans for generated code files
func (c *CodegenExtractor) scanForGeneratedCode(files []*shared.FileInfo, signals *shared.SignalData) {
	generatedPaths := make(map[string][]string)

	for _, file := range files {
		if file.IsGenerated {
			tool := c.detectGeneratedCodeTool(file.Path, file.Basename)
			if tool != "unknown" {
				generatedPaths[tool] = append(generatedPaths[tool], file.Path)
			}
		}
	}

	// Add generated code specs
	for tool, paths := range generatedPaths {
		if len(paths) > 0 {
			// Check if we already have this tool
			found := false
			for i, existing := range signals.Codegen {
				if existing.Tool == tool {
					signals.Codegen[i].Paths = append(signals.Codegen[i].Paths, paths...)
					found = true
					break
				}
			}

			if !found {
				signals.Codegen = append(signals.Codegen, shared.CodegenSpec{
					Tool:  tool,
					Paths: paths,
				})
			}
		}
	}
}

// detectGeneratedCodeTool determines the tool that generated a file
func (c *CodegenExtractor) detectGeneratedCodeTool(path, basename string) string {
	lowerPath := strings.ToLower(path)
	lowerBasename := strings.ToLower(basename)

	// Check file patterns
	if strings.HasSuffix(lowerBasename, ".pb.go") {
		return "protobuf"
	}
	if strings.HasSuffix(lowerBasename, ".g.dart") {
		return "dart-codegen"
	}
	if strings.HasSuffix(lowerBasename, "_pb2.py") {
		return "protobuf"
	}
	if strings.Contains(lowerBasename, ".generated.") {
		if strings.Contains(lowerPath, "prisma") {
			return "prisma"
		}
		if strings.Contains(lowerPath, "graphql") {
			return "graphql-codegen"
		}
		return "codegen"
	}

	return "unknown"
}
