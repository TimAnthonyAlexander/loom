#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}✨ Starting Loom GUI...${NC}"
echo -e "${BLUE}🌐 GUI: http://localhost:34115${NC}"
echo -e "${BLUE}🔧 Frontend: http://localhost:1420${NC}"
echo ""

# Ensure wails is in PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Show only important messages - filter out repetitive binding generation
wails dev 2>&1 | grep -v "KnownStructs:" | grep -v "Not found: time.Time" | grep -E "(Done\.|ERROR|FATAL|Failed|Error|VITE.*ready|Using DevServer|Serving assets|Ctrl\+C|INF \|)"