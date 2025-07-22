# Loom Edit Module

A robust, deterministic file editing system for AI assistants that prevents corrupted patches and ensures safe file modifications.

## Overview

The Loom Edit module implements a precise, machine-readable syntax for file edits that eliminates the ambiguity and errors of natural language editing instructions.

## Command Syntax

```
>>LOOM_EDIT file=<RELATIVE_PATH> v=<FILE_SHA> <ACTION> <START>-<END>
<NEW TEXT LINES…>
<<LOOM_EDIT
```

### Fields

- `file=` - Target file path (relative)
- `v=` - SHA-1 hash of the entire file when read (prevents applying to changed files)
- `<ACTION>` - One of: `REPLACE`, `INSERT_AFTER`, `INSERT_BEFORE`, `DELETE`
- `<START>-<END>` - 1-based inclusive line numbers
- Body - The replacement/insertion text (empty for DELETE)

## Supported Actions

### REPLACE
Replaces lines START through END with new content.
```
>>LOOM_EDIT file=docs/README.md v=abc123... REPLACE 4-5
New line 4
New line 5
<<LOOM_EDIT
```

### INSERT_AFTER
Inserts new content after the specified line.
```
>>LOOM_EDIT file=docs/README.md v=abc123... INSERT_AFTER 9
New content to insert
<<LOOM_EDIT
```

### INSERT_BEFORE
Inserts new content before the specified line.
```
>>LOOM_EDIT file=docs/README.md v=abc123... INSERT_BEFORE 5
New content to insert
<<LOOM_EDIT
```

### DELETE
Removes lines START through END.
```
>>LOOM_EDIT file=docs/README.md v=abc123... DELETE 15-17
<<LOOM_EDIT
```

## API

### ParseEditCommand(input string) (*EditCommand, error)
Parses a LOOM_EDIT command block into an EditCommand struct.

### ApplyEdit(filePath string, cmd *EditCommand) error
Applies an EditCommand to a file with full validation:
- Verifies file SHA matches
- Validates line ranges
- Confirms old slice hash
- Applies the requested operation

### HashContent(content string) string
Computes SHA-1 hash of content for validation.

## Safety Features

1. **File SHA validation** - Prevents applying edits to changed files
2. **Slice hash validation** - Ensures the target lines haven't changed
3. **Range validation** - Checks line numbers are within bounds
4. **Deterministic operations** - Each action has explicit, unambiguous semantics
5. **Newline normalization** - Handles CRLF, LF, and mixed line endings consistently
6. **Trailing newline preservation** - Maintains original file's trailing newline behavior

## Robustness Features

### Cross-Platform Line Ending Support
The module automatically normalizes different line ending formats:
- Windows (`\r\n`) → Unix (`\n`)
- Classic Mac (`\r`) → Unix (`\n`) 
- Mixed line endings are handled consistently

This prevents hash mismatches and parsing errors when working with files from different platforms.

### Trailing Newline Preservation
Files ending with a newline character maintain that behavior after editing. Files without trailing newlines keep that format. This ensures compatibility with POSIX tools and maintains the original file's formatting conventions.

## Testing

Run the test suite:
```bash
go test -v
```

The tests validate all four operations against example cases and include comprehensive error handling tests. 
