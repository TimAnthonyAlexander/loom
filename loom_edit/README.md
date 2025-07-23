# LOOM_EDIT

A simple, deterministic file editing format with content validation.

## Format

```
>>LOOM_EDIT file=<RELATIVE_PATH> <ACTION> <START>-<END>
<NEW TEXT LINES…>
<<LOOM_EDIT
```

## Supported Actions

- **REPLACE**: Replace lines START-END with new content
- **INSERT_AFTER**: Insert new content after line START
- **INSERT_BEFORE**: Insert new content before line START
- **DELETE**: Remove lines START-END (empty body)
- **SEARCH_REPLACE**: Replace all occurrences of a string with another string
- **CREATE**: Create a new file with the given content

## Examples

**Single line replacement**:
```
>>LOOM_EDIT file=main.go REPLACE 42
    username := "john"
<<LOOM_EDIT
```

**Multi-line replacement**:
```
>>LOOM_EDIT file=handler.go REPLACE 28-31
        return &ValidationError{
            Field:   "request", 
            Message: "request cannot be nil",
        }
<<LOOM_EDIT
```

**Insert after line**:
```
>>LOOM_EDIT file=config.go INSERT_AFTER 15
    newConfigOption := "value"
<<LOOM_EDIT
```

**Delete lines**:
```
>>LOOM_EDIT file=utils.go DELETE 20-22
<<LOOM_EDIT
```

**Search and replace**:
```
>>LOOM_EDIT file=config.go SEARCH_REPLACE "localhost:8080" "localhost:9090"
<<LOOM_EDIT
```

**Create new file**:
```
>>LOOM_EDIT file=docs/FEATURES.md CREATE
# Features

This is a new file created with the CREATE action.
<<LOOM_EDIT
```

## Features

- Line-based file editing with strong validation
- Content hashing for safety
- Automatic directory creation for new files
- Robust error handling
- Cross-platform line ending normalization
- Trailing newline preservation 
