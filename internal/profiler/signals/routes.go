package signals

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// RoutesExtractor extracts information about API routes and services
type RoutesExtractor struct {
	root string
}

// NewRoutesExtractor creates a new routes extractor
func NewRoutesExtractor(root string) *RoutesExtractor {
	return &RoutesExtractor{root: root}
}

// Extract processes files and detects routes and services
func (r *RoutesExtractor) Extract(files []*shared.FileInfo, existing *shared.SignalData) {
	for _, file := range files {
		if r.isRouteFile(file.Path, file.Extension) {
			r.extractRoutesFromFile(file, existing)
		}
	}
}

// isRouteFile checks if a file likely contains route definitions
func (r *RoutesExtractor) isRouteFile(path, extension string) bool {
	lowerPath := strings.ToLower(path)

	// Check for common route file patterns
	routePatterns := []string{
		"/routes/",
		"/router/",
		"/controllers/",
		"/handlers/",
		"/endpoints/",
		"/api/",
		"routes.js",
		"routes.ts",
		"router.js",
		"router.ts",
		"app.js",
		"app.ts",
		"server.js",
		"server.ts",
		"index.js",
		"index.ts",
		"main.go",
	}

	for _, pattern := range routePatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	// Check for Laravel route files
	if strings.Contains(lowerPath, "web.php") || strings.Contains(lowerPath, "api.php") {
		return true
	}

	// Check for Go files that might contain routes
	if extension == ".go" && (strings.Contains(lowerPath, "handler") ||
		strings.Contains(lowerPath, "controller") ||
		strings.Contains(lowerPath, "route")) {
		return true
	}

	// Check for Python Flask/Django files
	if extension == ".py" && (strings.Contains(lowerPath, "views") ||
		strings.Contains(lowerPath, "urls") ||
		strings.Contains(lowerPath, "routes")) {
		return true
	}

	return false
}

// extractRoutesFromFile extracts routes from a specific file
func (r *RoutesExtractor) extractRoutesFromFile(file *shared.FileInfo, signals *shared.SignalData) {
	switch file.Extension {
	case ".js", ".ts", ".jsx", ".tsx":
		r.extractJSRoutes(file, signals)
	case ".go":
		r.extractGoRoutes(file, signals)
	case ".php":
		r.extractPHPRoutes(file, signals)
	case ".py":
		r.extractPythonRoutes(file, signals)
	}
}

// extractJSRoutes extracts routes from JavaScript/TypeScript files
func (r *RoutesExtractor) extractJSRoutes(file *shared.FileInfo, signals *shared.SignalData) {
	content, err := os.ReadFile(filepath.Join(r.root, file.Path))
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")

	// Patterns for common JS/TS route definitions
	patterns := []*regexp.Regexp{
		// Express.js patterns
		regexp.MustCompile(`app\.(get|post|put|delete|patch|head|options)\s*\(\s*['"]([^'"]+)['"]`),
		regexp.MustCompile(`router\.(get|post|put|delete|patch|head|options)\s*\(\s*['"]([^'"]+)['"]`),
		// Next.js API routes (file-based)
		regexp.MustCompile(`export\s+(?:default\s+)?(?:async\s+)?function\s+(\w+)`),
		// Fastify patterns
		regexp.MustCompile(`fastify\.(get|post|put|delete|patch|head|options)\s*\(\s*['"]([^'"]+)['"]`),
		// Koa patterns
		regexp.MustCompile(`\.use\s*\(\s*route\s*\(\s*['"]([^'"]+)['"]`),
	}

	for i, line := range lines {
		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 3 {
					method := strings.ToUpper(match[1])
					path := match[2]
					name := r.generateRouteName(method, path, file.Path, i)

					route := shared.RouteOrService{
						Kind: "route",
						Path: file.Path,
						Name: name,
					}
					signals.RoutesServices = append(signals.RoutesServices, route)
				}
			}
		}

		// Check for Next.js API routes (file-based routing)
		if strings.Contains(strings.ToLower(file.Path), "/api/") &&
			(strings.Contains(line, "export default") || strings.Contains(line, "export async function")) {
			routeName := r.extractNextJSRouteName(file.Path)
			if routeName != "" {
				route := shared.RouteOrService{
					Kind: "route",
					Path: file.Path,
					Name: routeName,
				}
				signals.RoutesServices = append(signals.RoutesServices, route)
			}
		}
	}
}

// extractGoRoutes extracts routes from Go files
func (r *RoutesExtractor) extractGoRoutes(file *shared.FileInfo, signals *shared.SignalData) {
	content, err := os.ReadFile(filepath.Join(r.root, file.Path))
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")

	// Patterns for common Go route definitions
	patterns := []*regexp.Regexp{
		// Gorilla Mux patterns
		regexp.MustCompile(`\.HandleFunc\s*\(\s*"([^"]+)"\s*,\s*(\w+)`),
		regexp.MustCompile(`\.Handle\s*\(\s*"([^"]+)"\s*,\s*(\w+)`),
		// Gin patterns
		regexp.MustCompile(`\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*\(\s*"([^"]+)"\s*,\s*(\w+)`),
		// Echo patterns
		regexp.MustCompile(`e\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*\(\s*"([^"]+)"\s*,\s*(\w+)`),
		// Chi patterns
		regexp.MustCompile(`r\.(Get|Post|Put|Delete|Patch|Head|Options)\s*\(\s*"([^"]+)"\s*,\s*(\w+)`),
		// Standard library patterns
		regexp.MustCompile(`http\.HandleFunc\s*\(\s*"([^"]+)"\s*,\s*(\w+)`),
	}

	for i, line := range lines {
		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 3 {
					path := match[1]
					handler := match[2]

					// For patterns with explicit method
					method := "GET" // default
					if len(match) >= 4 {
						method = strings.ToUpper(match[1])
						path = match[2]
						handler = match[3]
					}

					name := r.generateRouteName(method, path, handler, i)

					route := shared.RouteOrService{
						Kind: "route",
						Path: file.Path,
						Name: name,
					}
					signals.RoutesServices = append(signals.RoutesServices, route)
				}
			}
		}

		// Look for service-like structs
		if strings.Contains(line, "type") && strings.Contains(line, "Service") && strings.Contains(line, "struct") {
			serviceName := r.extractGoServiceName(line)
			if serviceName != "" {
				service := shared.RouteOrService{
					Kind: "service",
					Path: file.Path,
					Name: serviceName,
				}
				signals.RoutesServices = append(signals.RoutesServices, service)
			}
		}
	}
}

// extractPHPRoutes extracts routes from PHP files (Laravel focus)
func (r *RoutesExtractor) extractPHPRoutes(file *shared.FileInfo, signals *shared.SignalData) {
	content, err := os.ReadFile(filepath.Join(r.root, file.Path))
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")

	// Patterns for Laravel route definitions
	patterns := []*regexp.Regexp{
		// Laravel Route facade
		regexp.MustCompile(`Route::(get|post|put|delete|patch|head|options)\s*\(\s*['"]([^'"]+)['"]`),
		regexp.MustCompile(`Route::match\s*\(\s*\[([^\]]+)\]\s*,\s*['"]([^'"]+)['"]`),
		regexp.MustCompile(`Route::any\s*\(\s*['"]([^'"]+)['"]`),
		// Resource routes
		regexp.MustCompile(`Route::resource\s*\(\s*['"]([^'"]+)['"]`),
		regexp.MustCompile(`Route::apiResource\s*\(\s*['"]([^'"]+)['"]`),
	}

	for i, line := range lines {
		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 2 {
					var method, path string

					if strings.Contains(match[0], "::resource") || strings.Contains(match[0], "::apiResource") {
						method = "RESOURCE"
						path = match[1]
					} else if strings.Contains(match[0], "::any") {
						method = "ANY"
						path = match[1]
					} else if strings.Contains(match[0], "::match") {
						method = match[1]
						path = match[2]
					} else {
						method = strings.ToUpper(match[1])
						path = match[2]
					}

					name := r.generateRouteName(method, path, file.Path, i)

					route := shared.RouteOrService{
						Kind: "route",
						Path: file.Path,
						Name: name,
					}
					signals.RoutesServices = append(signals.RoutesServices, route)
				}
			}
		}

		// Look for controller classes
		if strings.Contains(line, "class") && strings.Contains(line, "Controller") {
			controllerName := r.extractPHPControllerName(line)
			if controllerName != "" {
				service := shared.RouteOrService{
					Kind: "service",
					Path: file.Path,
					Name: controllerName,
				}
				signals.RoutesServices = append(signals.RoutesServices, service)
			}
		}
	}
}

// extractPythonRoutes extracts routes from Python files (Flask/Django focus)
func (r *RoutesExtractor) extractPythonRoutes(file *shared.FileInfo, signals *shared.SignalData) {
	fileObj, err := os.Open(filepath.Join(r.root, file.Path))
	if err != nil {
		return
	}
	defer func() { _ = fileObj.Close() }()

	scanner := bufio.NewScanner(fileObj)
	lineNum := 0

	// Patterns for Python route definitions
	flaskPatterns := []*regexp.Regexp{
		regexp.MustCompile(`@app\.route\s*\(\s*['"]([^'"]+)['"](?:.*methods\s*=\s*\[([^\]]+)\])?`),
		regexp.MustCompile(`@blueprint\.route\s*\(\s*['"]([^'"]+)['"](?:.*methods\s*=\s*\[([^\]]+)\])?`),
	}

	djangoPatterns := []*regexp.Regexp{
		regexp.MustCompile(`path\s*\(\s*['"]([^'"]+)['"]`),
		regexp.MustCompile(`url\s*\(\s*r?['"]([^'"]+)['"]`),
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Flask patterns
		for _, pattern := range flaskPatterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 2 {
					path := match[1]
					methods := "GET" // default
					if len(match) >= 3 && match[2] != "" {
						methods = match[2]
					}

					name := r.generateRouteName(methods, path, file.Path, lineNum)

					route := shared.RouteOrService{
						Kind: "route",
						Path: file.Path,
						Name: name,
					}
					signals.RoutesServices = append(signals.RoutesServices, route)
				}
			}
		}

		// Django patterns
		for _, pattern := range djangoPatterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 2 {
					path := match[1]
					name := r.generateRouteName("", path, file.Path, lineNum)

					route := shared.RouteOrService{
						Kind: "route",
						Path: file.Path,
						Name: name,
					}
					signals.RoutesServices = append(signals.RoutesServices, route)
				}
			}
		}

		// Look for view classes
		if strings.Contains(line, "class") && (strings.Contains(line, "View") || strings.Contains(line, "ViewSet")) {
			viewName := r.extractPythonViewName(line)
			if viewName != "" {
				service := shared.RouteOrService{
					Kind: "service",
					Path: file.Path,
					Name: viewName,
				}
				signals.RoutesServices = append(signals.RoutesServices, service)
			}
		}
	}
}

// Helper functions for name extraction

func (r *RoutesExtractor) generateRouteName(method, path, context string, line int) string {
	// Clean up the path for a readable name
	cleanPath := strings.ReplaceAll(path, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, ":", "")
	cleanPath = strings.ReplaceAll(cleanPath, "{", "")
	cleanPath = strings.ReplaceAll(cleanPath, "}", "")
	cleanPath = strings.Trim(cleanPath, "_")

	if cleanPath == "" {
		cleanPath = "root"
	}

	if method != "" && method != "GET" {
		return strings.ToLower(method) + "_" + cleanPath
	}

	return cleanPath
}

func (r *RoutesExtractor) extractNextJSRouteName(path string) string {
	// Extract route name from Next.js API file path
	// e.g., "/api/users/[id].ts" -> "users_by_id"
	parts := strings.Split(path, "/")
	var routeParts []string

	for _, part := range parts {
		if part == "api" || part == "" {
			continue
		}

		// Handle dynamic routes
		if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
			param := strings.Trim(part, "[]")
			routeParts = append(routeParts, "by_"+param)
		} else {
			// Remove file extension
			if strings.Contains(part, ".") {
				part = strings.Split(part, ".")[0]
			}
			routeParts = append(routeParts, part)
		}
	}

	return strings.Join(routeParts, "_")
}

func (r *RoutesExtractor) extractGoServiceName(line string) string {
	// Extract service name from Go type definition
	// e.g., "type UserService struct {" -> "UserService"
	re := regexp.MustCompile(`type\s+(\w*Service)\s+struct`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func (r *RoutesExtractor) extractPHPControllerName(line string) string {
	// Extract controller name from PHP class definition
	// e.g., "class UserController extends Controller" -> "UserController"
	re := regexp.MustCompile(`class\s+(\w*Controller)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func (r *RoutesExtractor) extractPythonViewName(line string) string {
	// Extract view name from Python class definition
	// e.g., "class UserView(APIView):" -> "UserView"
	re := regexp.MustCompile(`class\s+(\w*(?:View|ViewSet))`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}
