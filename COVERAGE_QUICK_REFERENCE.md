# Test Coverage Quick Reference

## Priority Order: Highest ROI First

### 🔥 PHASE 1: DO THIS FIRST (2-3 hours total)

#### 1. internal/logging (30 min) — Reach 95% coverage
```bash
# Add to logger_test.go:
✅ TestParseLevel — All cases (debug, info, warn, error, invalid)
✅ TestDebugMethod — Test l.Debug() writes at correct level
✅ TestInfoMethod — Test l.Info() writes at correct level  
✅ TestWarnMethod — Test l.Warn() writes at correct level
✅ TestErrorMethod — Test l.Error() writes at correct level
✅ TestClose — Close multiple times (idempotency)
✅ TestWriter — Writer() returns valid io.Writer
```

#### 2. internal/state (20 min) — Reach 95% coverage
```bash
# Add to state_test.go:
✅ TestDefaultStatePath — Verify default path exists
✅ TestExpandHome — Test ~ expansion and non-~ paths
✅ TestSetProviderResult — Direct test of provider result setting
```

#### 3. internal/provider helpers (25 min) — Easy wins
```bash
# Add to helpers_test.go:
✅ TestExitCode — Test nil, exec.ExitError, other errors
✅ TestFormatCommand — Test command formatting with args
✅ TestCommandExists — Test with common commands (sh, echo)
✅ TestSetVerboseOutput — Setter/getter for verbose writer
✅ TestVerboseWriter — Get current verbose writer
```

#### 4. internal/notify (2 min)
```bash
# Add to notify_test.go:
✅ TestNew — Constructor creates Notifier with correct config
```

#### 5. internal/ui pure functions (90 min) — 7 tests
```bash
# Add to ui_test.go:
✅ TestStatusEmojiAllCases — All emoji for status strings
✅ TestReportColumns — Pure function for all state transitions
✅ TestRowStatusAndOutdated — Pure function with various inputs
✅ TestDisplayNameFor — Pure name lookup
✅ TestCopyConfig — Config deep copy
✅ TestCopyStringSlice — String slice copy
✅ TestCopyStringBoolMap — Map[string]bool copy
```

**SUBTOTAL: 2-3 hours → +20% overall coverage**

---

### ⭐ PHASE 2: MEDIUM DIFFICULTY (1-2 weeks)

#### Provider Constructors (30 min)
```bash
# Add to each provider_test.go:
TestNewBrewProvider, TestNewNpmProvider, TestNewPipProvider, etc.
# 11 providers × 3 min each = 33 minutes
```

#### Provider Getters (15 min)
```bash
# Test Name(), DisplayName(), DependsOn() for each provider
# Same files, trivial tests
```

#### Notify Paths (45 min)
```bash
# Add to notify_test.go (requires exec.Command mock):
✅ TestNotify_TerminalNotifierEnabled — Mock exec output
✅ TestNotify_OsascriptEnabled — Mock exec output
✅ TestNotify_AutoDetect — Mock exec.LookPath for PATH lookups
```

#### UI Mocking (requires pterm mock layer)
```bash
# Medium difficulty: need pterm interface
✅ TestIsTTY — Mock term.IsTerminal
✅ TestTermWidth — Mock term.GetSize
✅ TestProgressBar — Mock pterm.DefaultProgressbar
✅ TestPrintInfo/Warning/Error — Mock pterm output
```

---

### 🟰 PHASE 3: LONG-TERM (ongoing)

#### Provider Scan/Update Methods
- Requires mocking exec.CommandContext
- High complexity but critical functionality
- Consider integration tests instead (run real provider if tool available)

#### TUI Integration Tests
- RunConfigEditor + edit* functions
- Requires stdin simulation or pterm mock framework
- Consider acceptance tests or manual QA

#### File Locking Tests
- lockFile/unlockFile in state package
- Hard to test portably; low priority

---

## Testing Strategy Reference

### How to Mock exec.Command
```go
// Option 1: Create a test double that captures commands
type MockCommand struct {
    Name     string
    Args     []string
    Output   string
    ExitCode int
}

// Option 2: Use environment variables
os.Setenv("PATH", "/test/path:"+os.Getenv("PATH"))
// Create fake command in test directory

// Option 3: Use a library like github.com/golang/mock
```

### How to Test TUI Interactions
```go
// Option 1: Test output formatting, not interaction
// e.g., verify WrapPackages before testing full table

// Option 2: Create pterm interface wrapper
type PTerm interface {
    ShowSelect(opts []string) (string, error)
    ShowConfirm(msg string) (bool, error)
    ShowTextInput(prompt string) (string, error)
}

// Option 3: Integration tests with prepared stdin
cmd := exec.Command("./upkeep", "edit-config")
cmd.Stdin = strings.NewReader("1\nNO\n")
```

---

## File Locations

```
internal/ui/
  ├─ ui.go + ui_test.go
  ├─ configeditor.go + configeditor_test.go
  ├─ fieldeditors.go [add fieldeditors_test.go]
  ├─ confirm.go [add confirm_test.go]
  └─ live_table.go + live_table_test.go

internal/notify/
  └─ notify.go + notify_test.go

internal/provider/
  ├─ provider.go [no tests needed]
  ├─ registry.go + registry_test.go ✅
  ├─ helpers.go [add helpers_test.go]
  ├─ export_test.go [modify as needed]
  └─ [all]_test.go

internal/logging/
  └─ logger.go + logger_test.go

internal/state/
  └─ state.go + state_test.go
```

---

## Coverage Goals

| Package | Current | Phase 1 | Phase 2 | Phase 3 | Target |
|---------|---------|---------|---------|---------|--------|
| logging | 73.2% | →95% | →98% | →99% | 99% |
| state | 73.9% | →95% | →98% | →99% | 99% |
| provider | 44.1% | →55% | →75% | →90% | 90% |
| notify | 30.8% | →35% | →70% | →85% | 85% |
| ui | 22.7% | →30% | →55% | →75% | 75% |

---

## Dependency Graph (for ordering)

```
logging (no dependencies) → FIRST
state (no dependencies) → FIRST
provider/helpers → provider implementations
provider/implementations → engine (higher-level)
notify (independent) → ANYTIME
ui (most dependencies) → LAST
```

