package signals

import (
	"github.com/loom/loom/internal/profiler/shared"
)

// Collector orchestrates all signal extractors
type Collector struct {
	root              string
	manifestExtractor *ManifestExtractor
	scriptExtractor   *ScriptExtractor
	ciExtractor       *CIExtractor
	docsExtractor     *DocsExtractor
	configsExtractor  *ConfigsExtractor
	codegenExtractor  *CodegenExtractor
	routesExtractor   *RoutesExtractor
}

// NewCollector creates a new signal collector
func NewCollector(root string) *Collector {
	return &Collector{
		root:              root,
		manifestExtractor: NewManifestExtractor(root),
		scriptExtractor:   NewScriptExtractor(root),
		ciExtractor:       NewCIExtractor(root),
		docsExtractor:     NewDocsExtractor(root),
		configsExtractor:  NewConfigsExtractor(root),
		codegenExtractor:  NewCodegenExtractor(root),
		routesExtractor:   NewRoutesExtractor(root),
	}
}

// Collect runs all extractors and returns aggregated signals
func (c *Collector) Collect(files []*shared.FileInfo) *shared.SignalData {
	// Start with manifest extraction as it provides the foundation
	signals := c.manifestExtractor.Extract(files)

	// Run all other extractors, each adding to the signals
	c.scriptExtractor.Extract(files, signals)
	c.ciExtractor.Extract(files, signals)
	c.docsExtractor.Extract(files, signals)
	c.configsExtractor.Extract(files, signals)
	c.codegenExtractor.Extract(files, signals)
	c.routesExtractor.Extract(files, signals)

	return signals
}
