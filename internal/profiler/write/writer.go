package write

import (
	"github.com/loom/loom/internal/profiler/shared"
)

// Writer handles all output writing operations
type Writer struct {
	root          string
	profileWriter *ProjectProfileWriter
	hotlistWriter *HotlistWriter
	rulesWriter   *RulesWriter
}

// NewWriter creates a new writer
func NewWriter(root string) *Writer {
	return &Writer{
		root:          root,
		profileWriter: NewProjectProfileWriter(root),
		hotlistWriter: NewHotlistWriter(root),
		rulesWriter:   NewRulesWriter(root),
	}
}

// WriteAll writes all output files atomically
func (w *Writer) WriteAll(profile *shared.Profile) error {
	// Write project profile JSON
	if err := w.profileWriter.Write(profile); err != nil {
		return err
	}

	// Write hotlist
	if err := w.hotlistWriter.Write(profile.ImportantFiles); err != nil {
		return err
	}

	// Write rules
	if err := w.rulesWriter.Write(profile); err != nil {
		return err
	}

	return nil
}

// CheckShouldRun determines if the profiler should run
func (w *Writer) CheckShouldRun() bool {
	// Run if no profile exists
	if !w.profileWriter.Exists() {
		return true
	}

	// Run if profile is stale
	if w.profileWriter.IsStale() {
		return true
	}

	// Run if profile version is incompatible
	if _, err := w.profileWriter.Read(); err != nil {
		if _, ok := err.(*VersionIncompatibleError); ok {
			return true // Force rebuild on version mismatch
		}
	}

	return false
}

// GetExistingProfile reads an existing profile if it exists
func (w *Writer) GetExistingProfile() (*shared.Profile, error) {
	return w.profileWriter.Read()
}

// GetExistingHotlist reads an existing hotlist if it exists
func (w *Writer) GetExistingHotlist() ([]string, error) {
	return w.hotlistWriter.Read()
}

// GetExistingRules reads existing rules if they exist
func (w *Writer) GetExistingRules() (string, error) {
	return w.rulesWriter.Read()
}
