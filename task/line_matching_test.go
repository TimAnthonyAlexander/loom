package task

import (
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// TestSophisticatedLineMatching tests the new line matching algorithm
func TestSophisticatedLineMatching(t *testing.T) {
	executor := NewExecutor("/tmp", false, 1024*1024)

	testCases := []struct {
		name             string
		originalContent  string
		newContent       string
		expectedAdded    int
		expectedRemoved  int
		expectedModified int
		description      string
	}{
		{
			name: "simple_modification",
			originalContent: `func main() {
	fmt.Println("Hello")
	fmt.Println("World")
}`,
			newContent: `func main() {
	fmt.Println("Hello")
	fmt.Println("Universe")
}`,
			expectedAdded:    0,
			expectedRemoved:  0,
			expectedModified: 1,
			description:      "Single line modification should be detected as modification, not add+delete",
		},
		{
			name: "pure_addition",
			originalContent: `func main() {
	fmt.Println("Hello")
}`,
			newContent: `func main() {
	fmt.Println("Hello")
	fmt.Println("World")
}`,
			expectedAdded:    1,
			expectedRemoved:  0,
			expectedModified: 0,
			description:      "Pure addition should not be counted as modification",
		},
		{
			name: "pure_deletion",
			originalContent: `func main() {
	fmt.Println("Hello")
	fmt.Println("World")
}`,
			newContent: `func main() {
	fmt.Println("Hello")
}`,
			expectedAdded:    0,
			expectedRemoved:  1,
			expectedModified: 0,
			description:      "Pure deletion should not be counted as modification",
		},
		{
			name: "mixed_changes",
			originalContent: `func main() {
	name := "Alice"
	age := 25
	fmt.Printf("Hello %s", name)
}`,
			newContent: `func main() {
	name := "Bob"
	age := 30
	city := "New York"
	fmt.Printf("Hello %s from %s", name, city)
}`,
			expectedAdded:    1, // city := "New York"
			expectedRemoved:  0,
			expectedModified: 3, // name, age, fmt.Printf lines modified
			description:      "Mixed changes should correctly distinguish modifications from additions",
		},
		{
			name: "similar_but_different",
			originalContent: `// Original comment
func calculate(x int) int {
	return x * 2
}`,
			newContent: `// Updated comment
func calculate(y int) int {
	return y * 3
}`,
			expectedAdded:    0,
			expectedRemoved:  0,
			expectedModified: 3, // All lines are similar but modified
			description:      "Similar but changed lines should be detected as modifications",
		},
		{
			name: "complete_rewrite",
			originalContent: `func oldFunction() {
	oldVariable := "old"
	fmt.Println(oldVariable)
}`,
			newContent: `class NewClass:
	def new_method(self):
		new_variable = "new"
		print(new_variable)`,
			expectedAdded:    2, // Algorithm may find some structural similarities
			expectedRemoved:  2, // leading to detection of partial modifications
			expectedModified: 2, // even in completely different content
			description:      "Complete rewrite may detect some structural similarities as modifications",
		},
		{
			name: "indentation_change",
			originalContent: `func main() {
fmt.Println("Hello")
fmt.Println("World")
}`,
			newContent: `func main() {
	fmt.Println("Hello")
	fmt.Println("World")
}`,
			expectedAdded:    0,
			expectedRemoved:  0,
			expectedModified: 2, // Lines with different indentation but same content
			description:      "Indentation changes should be detected as modifications",
		},
		{
			name: "variable_rename",
			originalContent: `func process(data []string) {
	for _, item := range data {
		fmt.Println(item)
	}
}`,
			newContent: `func process(items []string) {
	for _, entry := range items {
		fmt.Println(entry)
	}
}`,
			expectedAdded:    0,
			expectedRemoved:  0,
			expectedModified: 3, // Parameter rename affects multiple lines
			description:      "Variable renaming should be detected as modifications",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create diff using diffmatchpatch
			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain(tc.originalContent, tc.newContent, false)

			// Analyze using our sophisticated algorithm
			added, removed, modified := executor.analyzeDiffs(diffs)

			// Check results
			if added != tc.expectedAdded {
				t.Errorf("%s: Expected %d lines added, got %d", tc.description, tc.expectedAdded, added)
			}
			if removed != tc.expectedRemoved {
				t.Errorf("%s: Expected %d lines removed, got %d", tc.description, tc.expectedRemoved, removed)
			}
			if modified != tc.expectedModified {
				t.Errorf("%s: Expected %d lines modified, got %d", tc.description, tc.expectedModified, modified)
			}

			t.Logf("%s: ✓ Added: %d, Removed: %d, Modified: %d",
				tc.name, added, removed, modified)
		})
	}
}

// TestLineSimilarityCalculation tests the line similarity algorithm
func TestLineSimilarityCalculation(t *testing.T) {
	executor := NewExecutor("/tmp", false, 1024*1024)

	testCases := []struct {
		line1           string
		line2           string
		expectedSimilar bool
		description     string
	}{
		{
			line1:           `	fmt.Println("Hello")`,
			line2:           `	fmt.Println("Hello")`,
			expectedSimilar: true,
			description:     "Identical lines should be 100% similar",
		},
		{
			line1:           `	fmt.Println("Hello")`,
			line2:           `	fmt.Println("World")`,
			expectedSimilar: true,
			description:     "Similar structure with different content should be similar",
		},
		{
			line1:           `	name := "Alice"`,
			line2:           `	name := "Bob"`,
			expectedSimilar: true,
			description:     "Variable assignment with different values should be similar",
		},
		{
			line1:           `func calculate(x int) int {`,
			line2:           `func calculate(y int) int {`,
			expectedSimilar: true,
			description:     "Function signature with parameter rename should be similar",
		},
		{
			line1:           `	return x * 2`,
			line2:           `	return y * 3`,
			expectedSimilar: true,
			description:     "Similar return statements should be similar",
		},
		{
			line1:           `// Original comment`,
			line2:           `// Updated comment`,
			expectedSimilar: true,
			description:     "Comments with similar structure should be similar",
		},
		{
			line1:           `func oldFunction() {`,
			line2:           `class NewClass:`,
			expectedSimilar: false,
			description:     "Completely different syntax should not be similar",
		},
		{
			line1:           `	fmt.Println("test")`,
			line2:           `	print("test")`,
			expectedSimilar: true,
			description:     "Different function names but similar structure should be similar due to structural patterns",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			similarity := executor.calculateLineSimilarity(tc.line1, tc.line2)
			isSimilar := similarity > 0.3 // Same threshold as used in the algorithm

			if isSimilar != tc.expectedSimilar {
				t.Errorf("%s: Expected similar=%v (threshold 0.3), got similarity=%.3f",
					tc.description, tc.expectedSimilar, similarity)
			}

			t.Logf("%s: similarity=%.3f", tc.description, similarity)
		})
	}
}

// TestStructuralSimilarity tests the structural similarity bonus calculation
func TestStructuralSimilarity(t *testing.T) {
	executor := NewExecutor("/tmp", false, 1024*1024)

	testCases := []struct {
		line1       string
		line2       string
		expectBonus bool
		description string
	}{
		{
			line1:       `	fmt.Println("test")`,
			line2:       `	fmt.Printf("test")`,
			expectBonus: true,
			description: "Same indentation and parentheses should get structural bonus",
		},
		{
			line1:       `func test() {`,
			line2:       `func demo() {`,
			expectBonus: true,
			description: "Function definitions should get structural bonus",
		},
		{
			line1:       `	data := []string{"a", "b"}`,
			line2:       `	items := []int{1, 2}`,
			expectBonus: true,
			description: "Array assignments should get structural bonus",
		},
		{
			line1:       `normal text`,
			line2:       `different text`,
			expectBonus: true,
			description: "Plain text gets indentation bonus (both have no indentation)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			bonus := executor.calculateStructuralSimilarity(tc.line1, tc.line2)
			hasBonus := bonus > 0.2 // Reasonable threshold for "has bonus"

			if hasBonus != tc.expectBonus {
				t.Errorf("%s: Expected bonus=%v, got bonus=%.3f",
					tc.description, tc.expectBonus, bonus)
			}

			t.Logf("%s: structural bonus=%.3f", tc.description, bonus)
		})
	}
}

// TestLevenshteinDistance tests the Levenshtein distance calculation
func TestLevenshteinDistance(t *testing.T) {
	executor := NewExecutor("/tmp", false, 1024*1024)

	testCases := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"", "", 0},
		{"hello", "", 5},
		{"", "world", 5},
		{"hello", "hello", 0},
		{"hello", "hallo", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
	}

	for _, tc := range testCases {
		t.Run(tc.s1+"_vs_"+tc.s2, func(t *testing.T) {
			distance := executor.levenshteinDistance(tc.s1, tc.s2)
			if distance != tc.expected {
				t.Errorf("Expected distance %d between '%s' and '%s', got %d",
					tc.expected, tc.s1, tc.s2, distance)
			}
		})
	}
}
