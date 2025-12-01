# ASCII Art Welcome Header Design

**Date:** 2025-12-01
**Status:** Approved

## Overview

Add an ASCII art welcome header that displays when users run `chronicle` with no arguments. This provides a more welcoming and branded CLI experience.

## Design Decisions

### Style
- **Full block style** using Unicode box-drawing characters (███ ╗ ╔ ║ ╚ ╝ ═)
- **7 rows tall** for the "CHRONICLE" text
- **Tagline:** "📝 Timestamped logging for your development journey"

### Implementation Approach
Based on the simple approach used in https://github.com/harperreed/toki:

**Simply update the `Long` field in root.go** - No custom Run function needed. Cobra automatically displays the Long description when users run `chronicle` with no arguments.

### Behavior
- `chronicle` → Shows ASCII art + description
- `chronicle --help` → Shows ASCII art + description (same as above)
- `chronicle <command>` → Runs that command (no ASCII art)
- `chronicle foo bar` → Auto-adds command per existing logic (no ASCII art)

## Implementation

Update `/Users/harper/Public/src/personal/chronicle/internal/cli/root.go`:

```go
var rootCmd = &cobra.Command{
    Use:   "chronicle",
    Short: "Timestamped logging tool",
    Long: `
████████╗██╗  ██╗██████╗  ██████╗ ███╗   ██╗██╗ ██████╗██╗     ███████╗
██╔════╝██║  ██║██╔══██╗██╔═══██╗████╗  ██║██║██╔════╝██║     ██╔════╝
██║     ███████║██████╔╝██║   ██║██╔██╗ ██║██║██║     ██║     █████╗
██║     ██╔══██║██╔══██╗██║   ██║██║╚██╗██║██║██║     ██║     ██╔══╝
╚██████╗██║  ██║██║  ██║╚██████╔╝██║ ╚████║██║╚██████╗███████╗███████╗
 ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝ ╚═════╝╚══════╝╚══════╝

     📝 Timestamped logging for your development journey

Chronicle logs timestamped messages with metadata to SQLite and optional project log files.`,
}
```

## Testing

Manual testing required:
1. Run `chronicle` - should show ASCII art
2. Run `chronicle --help` - should show ASCII art
3. Run `chronicle list` - should list entries (no ASCII art)
4. Run `chronicle add test` - should add entry (no ASCII art)

## Success Criteria

- ✅ ASCII art displays when running `chronicle` with no arguments
- ✅ ASCII art uses Unicode box-drawing characters
- ✅ Tagline is centered and includes emoji
- ✅ All existing CLI behavior preserved
- ✅ Terminal width compatibility (fits in 80+ char terminals)
