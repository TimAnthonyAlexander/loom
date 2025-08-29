# Project Rules (detected)

**Languages:** Typescript, Go

## Entrypoints

**Frontend:**
- `ui/frontend/src/main.ts` (vite)
- `ui/frontend/src/main.ts` (vite)
- `web/src/main.ts` (vite)

**Backend:**
- `ui/frontend/wailsjs/runtime/runtime.js`
- `ui/wails.json` (wails)

## Commands

**Development:** `vite --port 5173 --strictPort`

**Build:** `tsc && vite build`

**Lint/Format:** `eslint .`

**Other Commands:**
- `vite preview` (package.json)
- `vite preview` (package.json)
- `make WAILS_BIN` (make)
- `make APP_DIR` (make)
- `make FRONTEND_DIR` (make)

## Tools & Configuration

**Linting/Formatting:** eslint

**Build Tools:** vite, vite

## Generated/Ignored Files

**Do not edit:** node_modules/, vendor/, dist/, build/, target/, *.generated.*, *.pb.go, *.g.dart, *.d.ts, *.min.*

## Key Files

- `go.mod` (graph_central, config)
- `ui/frontend/package.json` (graph_central, config)
- `ui/frontend/wailsjs/runtime/package.json` (graph_central, config)
- `web/package.json` (graph_central, config)
- `ui/frontend/src/types/ui.ts` (graph_central)
- `ui/frontend/wailsjs/go/bridge/App.js` (graph_central, entrypoint)
- `ui/frontend/wailsjs/runtime/runtime.js` (graph_central)
- `web/src/App.css` (graph_central)
- `ui/frontend/src/components/diff/DiffViewer.tsx` ()
- `ui/frontend/src/components/JohnBerry.tsx` ()


---
*This file was generated automatically by Loom Project Profiler*
