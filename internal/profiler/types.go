package profiler

import "github.com/loom/loom/internal/profiler/shared"

// Re-export shared types for convenience
type Graph = shared.Graph
type FileInfo = shared.FileInfo
type HeuristicWeights = shared.HeuristicWeights
type ImportantFile = shared.ImportantFile
type SignalData = shared.SignalData
type EntryPoint = shared.EntryPoint
type Script = shared.Script
type CIConfig = shared.CIConfig
type ConfigFile = shared.ConfigFile
type CodegenSpec = shared.CodegenSpec
type RouteOrService = shared.RouteOrService
type Profile = shared.Profile

// NewGraph creates a new empty graph
var NewGraph = shared.NewGraph
