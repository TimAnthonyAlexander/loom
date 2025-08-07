# Debugging Loom

This guide explains how to get better error logging and debugging information when TypeScript/Vite errors occur in the Loom application.

## Problem

When TypeScript/Vite errors occur in the frontend, the entire page becomes a blank white page without any error information visible. This makes debugging difficult.

## Solutions

### 1. Development Mode with Debug Options

For development, use the debug mode which enables Chrome DevTools:

```bash
# Run in debug mode with Chrome DevTools
make debug

# Or run the frontend in debug mode only
cd cmd/loomgui/frontend
npm run dev:debug
```

### 2. Built App with Debug Options

For the built application, use the debug script:

```bash
# First build the app
make build-macos

# Then run with debug options
cd cmd/loomgui
./run_debug.sh
```

### 3. Manual Chrome DevTools Access

If DevTools don't open automatically:

1. Open Chrome and navigate to `chrome://inspect/#devices`
2. Click "Open dedicated DevTools for Node"
3. Look for your Loom app in the list and click "inspect"

### 4. Console Logging

The application now includes comprehensive error logging:

- **Error Boundary**: Catches React component errors and displays them in the UI
- **Global Error Handlers**: Catch JavaScript errors and unhandled promise rejections
- **Wails Runtime Logging**: Errors are logged to the Wails runtime (visible in terminal)

### 5. Environment Variables

Set these environment variables for additional debugging:

```bash
export WAILS_DEBUG=1
export WAILS_LOGGING=debug
```

### 6. Vite Configuration

The Vite configuration has been updated with:

- Source maps enabled for better debugging
- Detailed error overlay in development
- Enhanced logging for build warnings and errors

## Error Types and Solutions

### TypeScript Compilation Errors

These are caught by the global error handlers and logged to:
- Browser console
- Wails runtime (terminal output)
- Error boundary UI (if they cause React errors)

### Vite Build Errors

These are caught by the Vite configuration and logged to:
- Terminal during build process
- Browser console with detailed stack traces

### Runtime JavaScript Errors

These are caught by:
- Global `window.addEventListener('error')` handler
- Global `window.addEventListener('unhandledrejection')` handler
- React Error Boundary component

## Troubleshooting

### No Error Information Visible

1. Check the terminal where you ran the app for Wails runtime logs
2. Open Chrome DevTools manually using `chrome://inspect/#devices`
3. Look for error messages in the browser console

### Blank White Page

1. The Error Boundary should now show error details instead of a blank page
2. Check the terminal for Go/backend error logs
3. Use the debug mode to get Chrome DevTools access

### Build Errors

1. Run `make clean` then `make build` to ensure a clean build
2. Check the frontend build logs: `cd cmd/loomgui/frontend && npm run build`
3. Look for TypeScript compilation errors in the build output

## Development Workflow

For the best debugging experience during development:

1. Use `make debug` to run with Chrome DevTools enabled
2. Keep the terminal open to see Wails runtime logs
3. Use the browser's DevTools console for frontend debugging
4. Check the Error Boundary UI for React-specific errors

## Production Debugging

For debugging the built application:

1. Use `./run_debug.sh` script
2. Set environment variables for additional logging
3. Access Chrome DevTools manually if needed
4. Check system logs for any native errors 