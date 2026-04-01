# Upkeep Go CLI — Coverage Analysis Report

## Executive Summary

This report analyzes four low-coverage packages and two medium-coverage packages in the Upkeep codebase to identify untested functions and prioritize test coverage improvements.

### Coverage Status
- **internal/ui** — 22.7% coverage (CRITICAL)
- **internal/notify** — 30.8% coverage (CRITICAL)
- **internal/provider** — 44.1% coverage (LOW)
- **internal/logging** — 73.2% coverage (MEDIUM - quick wins available)
- **internal/state** — 73.9% coverage (MEDIUM - quick wins available)

---

## 1. internal/ui — 22.7% Coverage ❌ CRITICAL

### Package Overview
- **Files:** 5 source files (~2,400 LOC)
- **Main Responsibility:** TUI rendering, interactive config editing, live progress tables
- **Key Libraries:** pterm (terminal UI), bufio (stdin)
- **Testability:** Mixed — some functions are pure and easily testable; others require terminal interaction or TUI library mocking

### 1.1 Source Files Analysis

#### **ui.go** (392 LOC)
**Exported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `WrapPackages` | `func(pkgs []string, maxWidth int) []string` | ✅ YES | ⭐⭐⭐ EASY | Pure function, multiple test cases exist |
| `IsTTY` | `func() bool` | ❌ NO | ⭐ HARD | Uses `term.IsTerminal(os.Stdout.Fd())` — not mockable without reflection |
| `RenderScanSummaryTable` | `func(rows []ScanSummaryRow) int` | ✅ PARTIAL | ⭐⭐ MEDIUM | Tested for no-panic but not for output correctness; TTY vs non-TTY paths untested |
| `StatusLine` | `func(w io.Writer, ...) void` | ✅ YES | ⭐⭐⭐ EASY | Tested for output format |
| `ProgressBar` | `func(total int) func()` | ❌ NO | ⭐ HARD | Returns pterm progress bar; requires mocking pterm |
| `PrintInfo` | `func(format string, args ...any) void` | ❌ NO | ⭐ HARD | Calls pterm.Info.Println or fmt.Println; would need mock |
| `PrintWarning` | `func(format string, args ...any) void` | ❌ NO | ⭐ HARD | Same as PrintInfo |
| `PrintError` | `func(format string, args ...any) void` | ❌ NO | ⭐ HARD | Same as PrintInfo |
| `ScanSummaryRowsFromResults` | `func(map[string]ScanResult, map[string]string) []ScanSummaryRow` | ✅ YES | ⭐⭐⭐ EASY | Pure function, tested |
| `GroupSubRows` | `func(map[string][]string) []GroupSubRow` | ✅ YES | ⭐⭐⭐ EASY | Pure function, well-tested |
| `FormatGroupedPackageList` | `func(map[string][]string) string` | ✅ YES | ⭐⭐⭐ EASY | Pure function, tested |

**Unexported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `termWidth` | `func() int` | ❌ NO | ⭐ HARD | Uses term.GetSize() — requires TTY or mocking |
| `statusEmoji` | `func(status string) string` | ❌ NO | ⭐⭐⭐ EASY | Pure function, easily unit-testable |

#### **configeditor.go** (701 LOC)
**Exported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `RunConfigEditor` | `func(*config.Config) (*config.Config, bool, error)` | ❌ NO | ⭐ HARD | Interactive pterm menu; requires TUI simulation or mocking |

**Unexported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `editGeneralSection` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editProvidersSection` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editNotificationsSection` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editLoggingSection` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editBrewProvider` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editBrewCaskProvider` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editNpmProvider` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editComposerProvider` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editPipProvider` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editRustProvider` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editEditorProvider` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editOmzProvider` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editVimProvider` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editVagrantProvider` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `editVirtualBoxProvider` | `func(*config.Config) error` | ❌ NO | ⭐ HARD | Interactive menu |
| `copyConfig` | `func(*config.Config) *config.Config` | ❌ NO | ⭐⭐⭐ EASY | Pure copy function, easily unit-testable |
| `copyStringSlice` | `func([]string) []string` | ❌ NO | ⭐⭐⭐ EASY | Pure function |
| `copyStringBoolMap` | `func(map[string]bool) map[string]bool` | ❌ NO | ⭐⭐⭐ EASY | Pure function |
| `startsWith` | `func(string, string) bool` | ✅ YES | ⭐⭐⭐ EASY | Pure string comparison, tested |

#### **fieldeditors.go** (273 LOC)
**Unexported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `editBool` | `func(string, bool) (bool, error)` | ❌ NO | ⭐ HARD | Calls pterm.DefaultInteractiveConfirm |
| `editInt` | `func(string, int, int, int) (int, error)` | ❌ NO | ⭐ HARD | Calls pterm.DefaultInteractiveTextInput; loops on validation |
| `editString` | `func(string, string) (string, error)` | ❌ NO | ⭐ HARD | Calls pterm.DefaultInteractiveTextInput |
| `editEnum` | `func(string, string, []string) (string, error)` | ❌ NO | ⭐ HARD | Calls pterm.DefaultInteractiveSelect |
| `editStringSlice` | `func(string, []string) ([]string, error)` | ❌ NO | ⭐ HARD | Complex pterm interaction; loops |
| `editMapStringBool` | `func(string, map[string]bool) (map[string]bool, error)` | ❌ NO | ⭐ HARD | Complex pterm interaction; loops |
| `formatMenuLabel` | `func(string, any) string` | ✅ YES | ⭐⭐⭐ EASY | Pure function using type assertions, tested |
| `formatSlice` | `func([]string) string` | ✅ YES | ⭐⭐⭐ EASY | Pure function, tested |
| `formatMap` | `func(map[string]bool) string` | ✅ YES | ⭐⭐⭐ EASY | Pure function, tested |
| `removeFromSlice` | `func([]string, string) []string` | ✅ YES | ⭐⭐⭐ EASY | Pure function, tested |
| `mapKeys` | `func(map[string]bool) []string` | ✅ YES | ⭐⭐⭐ EASY | Pure function, tested |

#### **confirm.go** (49 LOC)
**Exported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `Confirm` | `func(string, bool) bool` | ✅ PARTIAL | ⭐ HARD | Tested only with `yesFlag=true`; TTY and fallback paths untested |

**Unexported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `simpleConfirm` | `func(string) bool` | ❌ NO | ⭐ HARD | Reads stdin directly; difficult to mock without os mocking |

#### **live_table.go** (348 LOC)
**Exported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `NewLiveUpdateTable` | `func([]ScanSummaryRow, int, io.Writer) *LiveUpdateTable` | ✅ YES | ⭐⭐⭐ EASY | Tested for construction; well-isolated |
| `OnProviderStart` | `func(name string) void` | ✅ YES | ⭐⭐⭐ EASY | Tested indirectly via output verification |
| `OnProviderComplete` | `func(name string, result UpdateResult) void` | ✅ YES | ⭐⭐⭐ EASY | Tested through non-TTY output verification |
| `SetTotalDuration` | `func(duration) void` | ✅ PARTIAL | ⭐⭐⭐ EASY | Tested indirectly; direct test would be trivial |
| `Stop` | `func() void` | ✅ YES | ⭐⭐⭐ EASY | Tested for idempotency and panic-safety |

**Unexported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `render` | `func() void` | ❌ NO | ⭐⭐ MEDIUM | Uses pterm.AreaPrinter; depends on TTY or AreaPrinter mock |
| `reportColumns` | `func(*providerUpdateState) (5 strings)` | ❌ NO | ⭐⭐⭐ EASY | Pure function, easily unit-testable |
| `rowStatusAndOutdated` | `func(ScanSummaryRow, *providerUpdateState) (string, string)` | ❌ NO | ⭐⭐⭐ EASY | Pure function, easily unit-testable |
| `displayNameFor` | `func(string) string` | ❌ NO | ⭐⭐⭐ EASY | Pure lookup function, easily unit-testable |

### 1.2 Test Files Summary

**Files with Tests:**
- `ui_test.go` — Tests `RenderScanSummaryTable`, `StatusLine`, `Confirm`, `ScanSummaryRowsFromResults`, `FormatGroupedPackageList`, `GroupSubRows`, `WrapPackages`
- `configeditor_test.go` — Tests `formatMenuLabel`, `formatSlice`, `formatMap`, `removeFromSlice`, `mapKeys`, `startsWith`
- `live_table_test.go` — Tests `NewLiveUpdateTable`, `OnProviderStart`, `OnProviderComplete`, state transitions, concurrency, idempotency

### 1.3 Quick Wins for UI Coverage

**Easy to test (pure functions, no mocking needed):**
1. ✅ `statusEmoji(string) string` — Add unit test for all emoji cases
2. ✅ `reportColumns(*providerUpdateState) (5 strings)` — Add unit test for all state transitions
3. ✅ `rowStatusAndOutdated(ScanSummaryRow, *providerUpdateState) (string, string)` — Add unit test
4. ✅ `displayNameFor(string) string` — Add unit test
5. ✅ `copyConfig(*config.Config) *config.Config` — Add unit test
6. ✅ `copyStringSlice([]string) []string` — Add unit test (or rely on Go's standard deep copy test)
7. ✅ `copyStringBoolMap(map[string]bool) map[string]bool` — Add unit test

**Medium difficulty (require some mocking):**
1. ⭐ `IsTTY() bool` — Mock `term.IsTerminal()` or use interface injection
2. ⭐ `termWidth() int` — Mock `term.GetSize()` or interface injection
3. ⭐ `ProgressBar(int) func()` — Mock pterm progress bar

**Hard (require terminal simulation or major refactoring):**
- `RunConfigEditor` and all edit* functions — Consider integration tests or TUI mocking framework

---

## 2. internal/notify — 30.8% Coverage ❌ CRITICAL

### Package Overview
- **Files:** 1 source file (87 LOC)
- **Main Responsibility:** macOS Notification Center integration via terminal-notifier or osascript
- **External Dependencies:** exec.Command (OS-level), terminal-notifier, osascript
- **Testability:** Excellent for parsing/building; difficult for actual command execution

### 2.1 Source Files Analysis

#### **notify.go** (87 LOC)
**Exported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `New` | `func(config.NotificationsConfig) *Notifier` | ❌ NO | ⭐⭐⭐ EASY | Simple constructor, trivial |
| `Notify` | `func(title, message, url string) error` | ❌ PARTIAL | ⭐ HARD | Tested only for disabled case; actual exec paths untested |
| `BuildTerminalNotifierArgs` | `func(title, message, url string) []string` | ✅ YES | ⭐⭐⭐ EASY | Pure function, tested for both with/without URL |
| `BuildOsascriptScript` | `func(title, message string) string` | ✅ YES | ⭐⭐⭐ EASY | Pure function, tested for output format |

**Unexported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `notifyTerminalNotifier` | `func(title, message, url string) error` | ❌ NO | ⭐⭐ MEDIUM | Calls exec.Command; can mock with test doubles |
| `notifyOsascript` | `func(title, message string) error` | ❌ NO | ⭐⭐ MEDIUM | Calls exec.Command; can mock with test doubles |

### 2.2 Test Files Summary

**notify_test.go:**
- ✅ Tests `Notify` with disabled notifications
- ✅ Tests `BuildTerminalNotifierArgs` (with and without URL)
- ✅ Tests `BuildOsascriptScript`

**NOT TESTED:**
- `New()` constructor
- `notifyTerminalNotifier()` — actual execution path
- `notifyOsascript()` — actual execution path
- `Notify()` with enabled notifications and auto-detection

### 2.3 Untested Notification Flows

1. **Notify with terminal-notifier explicitly configured:**
   - Should construct proper args
   - Should call exec.Command with correct args
   - Should return error if command fails

2. **Notify with osascript explicitly configured:**
   - Should construct proper script
   - Should call exec.Command with correct args
   - Should return error if command fails

3. **Notify with auto-detection (nil tool):**
   - Should detect terminal-notifier on PATH and use it
   - Should fall back to osascript if terminal-notifier not found
   - Should return error if both unavailable

### 2.4 Quick Wins for Notify Coverage

**High-value tests to add:**
1. ✅ `TestNew` — Test constructor creates Notifier with correct config
2. ✅ `TestNotify_TerminalNotifierPath` — Mock exec.Command, verify args
3. ✅ `TestNotify_OsascriptPath` — Mock exec.Command, verify script
4. ✅ `TestNotify_AutoDetect_PreferTerminalNotifier` — Mock PATH lookup
5. ✅ `TestNotify_AutoDetect_FallbackOsascript` — Mock PATH lookup
6. ✅ `TestNotify_CommandNotFound` — Test error handling when command missing

**Recommended approach:**
- Create a test double for `exec.Command` OR
- Use `os/exec` mocking library to intercept commands
- Example: Use environment variables to redirect command lookup (mock terminal-notifier availability)

---

## 3. internal/provider — 44.1% Coverage ❌ LOW

### Package Overview
- **Files:** 24 source files (~70 LOC each on average)
- **Structure:** Core types + 11 provider implementations (brew, npm, pip, rust, etc.)
- **Key Files:** provider.go (interface def), registry.go (provider registry), helpers.go (command execution)
- **Testability:** Highly provider-specific; core helpers easily testable

### 3.1 Core Package Files

#### **provider.go** (59 LOC)
**Exported Types/Interfaces:**

| Name | Type | Used | Notes |
|------|------|------|-------|
| `ErrDependencyNotMet` | var error | ✅ | Sentinel error, no test needed |
| `OutdatedItem` | struct | ✅ | Data struct, no behavior |
| `ScanResult` | struct | ✅ | Data struct, no behavior |
| `UpdateResult` | struct | ✅ | Data struct, no behavior |
| `Provider` | interface | ✅ | Interface, tested indirectly via implementations |

**No functions to test — pure data definitions.**

#### **registry.go** (111 LOC)
**Exported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `Register` | `func(Provider) void` | ✅ YES | ⭐⭐⭐ EASY | Global registry function, tested via registry_test.go |
| `Get` | `func(string) (Provider, error)` | ✅ YES | ⭐⭐⭐ EASY | Tested |
| `List` | `func() []string` | ✅ YES | ⭐⭐⭐ EASY | Tested |
| `GetAll` | `func() []Provider` | ✅ YES | ⭐⭐⭐ EASY | Tested |
| `GetByNames` | `func([]string) ([]Provider, error)` | ✅ YES | ⭐⭐⭐ EASY | Tested |

**Registry Methods (tested via package-level functions):**
- `(*Registry).Register`, `Get`, `List`, `GetAll`, `GetByNames` — ✅ All tested

#### **helpers.go** (158 LOC)
**Exported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `SetVerboseOutput` | `func(io.Writer) void` | ❌ NO | ⭐⭐⭐ EASY | Global state setter; trivial |
| `getVerboseWriter` | `func() io.Writer` | ❌ NO | ⭐⭐⭐ EASY | Global state getter; trivial |
| `CommandExists` | `func(string) bool` | ❌ NO | ⭐⭐ MEDIUM | Calls exec.LookPath; can mock PATH or mock lookup |
| `RunCommand` | `func(context, string, ...string) (string, string, error)` | ❌ NO | ⭐⭐ MEDIUM | Spawns subprocess; hard to test without mocking exec |
| `RunCommandWithLog` | `func(context, *Logger, string, ...string) (string, error)` | ❌ NO | ⭐⭐ MEDIUM | Spawns subprocess with logging |
| `RunCommandEnv` | `func(context, []string, string, ...string) (string, string, error)` | ❌ NO | ⭐⭐ MEDIUM | Spawns subprocess with env override |
| `RunCommandEnvWithLog` | `func(context, *Logger, []string, string, ...string) (string, error)` | ❌ NO | ⭐⭐ MEDIUM | Spawns subprocess with env + logging |
| `RunCommandVerbose` | `func(context, *Logger, io.Writer, string, ...string) (string, error)` | ❌ NO | ⭐⭐ MEDIUM | Spawns subprocess with verbose output |
| `ExitCode` | `func(error) int` | ❌ NO | ⭐⭐⭐ EASY | Pure function extracting exit code from exec.ExitError |
| `FormatCommand` | `func(string, ...string) string` | ❌ NO | ⭐⭐⭐ EASY | Pure formatting function |

**Unexported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `getVerboseWriter` | `func() io.Writer` | ❌ NO | ⭐⭐⭐ EASY | Pure getter (private) |

### 3.2 Provider Implementations Coverage

**Status Overview:**
- ✅ **Tested Providers:** brew, brewcask, npm, composer, pip, rust, vagrant, vim, virtualbox, omz, editor
- Most providers have **basic JSON parsing tests** but lack **integration/E2E tests**

**Example: Brew Provider**

| Method | Tested? | Coverage |
|--------|---------|----------|
| `NewBrewProvider` | ❌ NO | Constructor, untested |
| `Name()` | ❌ NO | Trivial getter, untested |
| `DisplayName()` | ❌ NO | Trivial getter, untested |
| `DependsOn()` | ❌ NO | Returns nil, untested |
| `Scan(ctx)` | ❌ NO | **Critical**: Runs `brew update` and `brew outdated --json=v2` |
| `Update(ctx, items)` | ❌ NO | **Critical**: Runs `brew upgrade` and post-hooks |
| `parseBrewOutdated(string)` | ✅ YES | JSON parsing tested |
| `logf(string, ...any)` | ❌ NO | No-op logging, untested |

**Pattern:** All providers have **parsing/helper functions tested** but **Scan/Update methods untested** (they spawn actual OS commands).

### 3.3 Untested High-Value Functions in Helpers

| Function | Reason for Low Coverage | Solution |
|----------|------------------------|----------|
| `ExitCode(error) int` | Never called in tests | Add simple unit test: `TestExitCode` |
| `FormatCommand(name, args) string` | Never called in tests | Add simple unit test: `TestFormatCommand` |
| `CommandExists(name) bool` | Never called in tests | Mock exec.LookPath or use real commands (echo, ls) |
| Run* functions | Spawning real processes is avoided | Mock exec.CommandContext or use test doubles |

### 3.4 Quick Wins for Provider Coverage

**Easy tests (no subprocess needed):**
1. ✅ `TestExitCode` — Test with nil, exec.ExitError, other errors
2. ✅ `TestFormatCommand` — Test formatting with various args
3. ✅ `TestCommandExists` — Test with common commands (sh, echo)
4. ✅ Provider constructors — `TestNewBrewProvider`, `TestNewNpmProvider`, etc.
5. ✅ Trivial getters — `Name()`, `DisplayName()`, `DependsOn()` for all providers

**Medium difficulty (require mocking):**
1. ⭐ `CommandExists` — Mock exec.LookPath
2. ⭐ Run* helper functions — Mock exec.CommandContext
3. ⭐ Provider `Scan` methods — Mock command output or mock exec

**Hard (E2E tests, require actual tools):**
- Full `Scan` + `Update` flow for each provider (integration tests)

---

## 4. internal/logging — 73.2% Coverage ✅ MEDIUM

### Package Overview
- **Files:** 1 source file (157 LOC)
- **Main Responsibility:** Daily rotating log files, level-based filtering
- **Testability:** Excellent — uses file I/O which is fully testable with temp directories

### 4.1 Source Files Analysis

#### **logger.go** (157 LOC)
**Exported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `ParseLevel` | `func(string) Level` | ❌ NO | ⭐⭐⭐ EASY | Pure function, easily unit-testable |
| `New` | `func(string, Level) *Logger` | ✅ YES | ⭐⭐⭐ EASY | Constructor, tested |
| `Debug` | `func(string, ...any) void` | ❌ NO | ⭐⭐⭐ EASY | Pure logging method, easily testable |
| `Info` | `func(string, ...any) void` | ❌ NO | ⭐⭐⭐ EASY | Pure logging method, easily testable |
| `Warn` | `func(string, ...any) void` | ❌ NO | ⭐⭐⭐ EASY | Pure logging method, easily testable |
| `Error` | `func(string, ...any) void` | ❌ NO | ⭐⭐⭐ EASY | Pure logging method, easily testable |
| `Writer` | `func() io.Writer` | ❌ NO | ⭐⭐⭐ EASY | Returns log file handle, testable with temp dir |
| `CurrentLogPath` | `func() string` | ✅ YES | ⭐⭐⭐ EASY | Tested |
| `Close` | `func() error` | ❌ NO | ⭐⭐⭐ EASY | Closes file, easily testable |
| `CaptureOutput` | `func(*exec.Cmd) (string, error)` | ✅ YES | ⭐⭐⭐ EASY | Tested with subprocess capture |

**Unexported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `log` | `func(Level, string, string, ...any) void` | ✅ PARTIAL | ⭐⭐⭐ EASY | Core logging logic, implicitly tested via Debug/Info/Warn/Error |
| `openFile` | `func() (*os.File, error)` | ✅ PARTIAL | ⭐⭐⭐ EASY | Tested indirectly; rotation logic needs explicit test |

### 4.2 Test Files Summary

**logger_test.go:**
- ✅ `TestLogger_CreatesFileWithCorrectDate` — Tests file creation
- ✅ `TestLogger_AppendMode` — Tests append behavior across logger instances
- ✅ `TestLogger_SubprocessCaptureWritesToLog` — Tests CaptureOutput
- ✅ `TestLogger_CurrentLogPath` — Tests path generation
- ✅ `TestLogger_LevelFiltering` — Tests log level filtering (WARN level + above)

### 4.3 Untested Functions

| Function | Why Not Tested | Quick Fix |
|----------|----------------|-----------|
| `ParseLevel(string) Level` | Never directly tested | Add `TestParseLevel` with all cases: "debug", "info", "warn", "error", invalid |
| `Debug/Info/Warn/Error` | Only Info/Warn tested indirectly | Add explicit tests for each method |
| `Writer()` | Never called in tests | Add `TestWriter` — verify it returns valid io.Writer |
| `Close()` | Never called in tests | Add `TestClose` — verify file handle closes |
| File rotation (day boundary) | Not tested | Hard to test without time mocking; lower priority |

### 4.4 Quick Wins for Logging Coverage

**High-value, simple tests:**
1. ✅ `TestParseLevel` — Test all level strings + default
2. ✅ `TestDebugMethod` — Test debug logging at LevelDebug
3. ✅ `TestInfoMethod` — Test info logging at LevelInfo
4. ✅ `TestWarnMethod` — Test warn logging at LevelWarn
5. ✅ `TestErrorMethod` — Test error logging at LevelError
6. ✅ `TestClose` — Test closing logger multiple times (idempotency)
7. ✅ `TestWriter` — Test Writer() returns valid handle

**Result:** These 7 tests should easily bring coverage to 90%+

---

## 5. internal/state — 73.9% Coverage ✅ MEDIUM

### Package Overview
- **Files:** 1 source file (199 LOC)
- **Main Responsibility:** Persistent state for resumability (saves/loads JSON state file)
- **Key Feature:** File locking via unix.Flock
- **Testability:** Excellent — uses file I/O and JSON, fully testable

### 5.1 Source Files Analysis

#### **state.go** (199 LOC)
**Exported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `DefaultStatePath` | `func() string` | ❌ NO | ⭐⭐⭐ EASY | Pure string expansion, easily testable |
| `New` | `func(string) *State` | ✅ YES | ⭐⭐⭐ EASY | Constructor, tested indirectly |
| `Load` | `func(string) (*State, error)` | ✅ YES | ⭐⭐⭐ EASY | Tested for missing files and round-trip |
| `(*State).Save` | `func() error` | ✅ YES | ⭐⭐⭐ EASY | Tested for round-trip persistence |
| `(*State).GetFailed` | `func() []string` | ✅ YES | ⭐⭐⭐ EASY | Tested |
| `(*State).GetDeferred` | `func() DeferredState` | ✅ YES | ⭐⭐⭐ EASY | Tested |
| `(*State).Clear` | `func() void` | ✅ YES | ⭐⭐⭐ EASY | Tested |
| `(*State).SetProviderResult` | `func(string, ProviderStatus) void` | ✅ PARTIAL | ⭐⭐⭐ EASY | Tested indirectly; direct test would be trivial |

**Unexported Functions:**

| Function | Signature | Tested? | Testability | Notes |
|----------|-----------|---------|-------------|-------|
| `lockFile` | `func(*os.File, bool) error` | ❌ NO | ⭐ HARD | Unix-specific flock; hard to test portably |
| `unlockFile` | `func(*os.File) error` | ❌ NO | ⭐ HARD | Unix-specific flock; hard to test portably |
| `expandHome` | `func(string) string` | ❌ NO | ⭐⭐⭐ EASY | Pure string expansion, easily testable |

### 5.2 Test Files Summary

**state_test.go:**
- ✅ `TestSaveLoad_RoundTrip` — Tests full save/load cycle
- ✅ `TestLoad_MissingFileReturnsEmpty` — Tests missing file handling
- ✅ `TestGetFailed` — Tests filtering failed providers
- ✅ `TestGetDeferred` — Tests deferred state retrieval
- ✅ `TestClear` — Tests state reset
- ✅ `TestSave_CreatesDirectory` — Tests directory creation

### 5.3 Untested Functions

| Function | Why Not Tested | Quick Fix |
|----------|----------------|-----------|
| `DefaultStatePath()` | Pure string function | Add `TestDefaultStatePath` |
| `expandHome(string)` | Pure string function | Add `TestExpandHome` with ~ and non-~ paths |
| `SetProviderResult` | Tested indirectly | Add direct test |

### 5.4 Quick Wins for State Coverage

**High-value, simple tests:**
1. ✅ `TestDefaultStatePath` — Verify it returns ~/.local/state/upkeep/last-run.json
2. ✅ `TestExpandHome` — Test ~ expansion, non-~ paths, edge cases
3. ✅ `TestSetProviderResult` — Test adding/updating provider results
4. ✅ `TestSave_Atomic` — Test atomic write (temp file then rename)
5. ✅ `TestLoad_ConcurrentReads` — Test concurrent Load calls (shared lock)

**Result:** These 4-5 tests should bring coverage to 90%+

---

## Summary & Recommendations

### High-Priority Coverage Gaps

| Package | Coverage | Critical Issues | Recommended Actions |
|---------|----------|-----------------|-------------------|
| **ui** | 22.7% | 50+ untested functions; complex pterm interactions | 1) Test all pure helper functions (10+ easy tests) 2) Add pterm mocking layer 3) Integration tests for TUI flows |
| **notify** | 30.8% | Actual notification execution untested | 1) Mock exec.Command 2) Add tests for all notification paths 3) Test auto-detection |
| **provider** | 44.1% | Helpers untested; provider Scan/Update not exercised | 1) Add 5 trivial tests for helpers 2) Mock subprocess execution 3) Integration tests |
| **logging** | 73.2% ✅ | 3-4 methods never tested | 1) Add `TestParseLevel` 2) Add individual log level tests 3) Test Close() |
| **state** | 73.9% ✅ | 2-3 pure functions never tested | 1) Add `TestDefaultStatePath` 2) Add `TestExpandHome` 3) Direct `SetProviderResult` test |

### Testing Strategy by Complexity

**Phase 1: Quick Wins (2-4 hours)**
- State package: 4 tests → reach 95% coverage
- Logging package: 7 tests → reach 95% coverage
- Provider helpers: 5 tests → reach 75% coverage
- Notify pure functions: Already tested, just need Notify() paths

**Phase 2: Moderate (1-2 weeks)**
- Create exec.Command mock/spy for provider helpers
- Create pterm mock layer for UI
- Add tests for all helper Run* functions
- Add tests for notify paths

**Phase 3: Long-term (ongoing)**
- Integration tests for each provider Scan/Update
- Full TUI interaction tests (require pterm mocking framework)
- Config editor tests (require stdin/TUI simulation)

### Recommended Quick Wins (Effort vs. Reward)

| Test | Effort | Value | Priority |
|------|--------|-------|----------|
| `TestParseLevel` in logging | 5 min | +5% coverage | 🔥 DO FIRST |
| `TestDefaultStatePath` in state | 5 min | +2% coverage | 🔥 DO FIRST |
| `TestExpandHome` in state | 10 min | +2% coverage | 🔥 DO FIRST |
| `TestExitCode` in provider/helpers | 5 min | +1% coverage | ⭐ EASY |
| `TestFormatCommand` in provider/helpers | 10 min | +1% coverage | ⭐ EASY |
| Provider constructor tests (11 providers) | 30 min | +3% coverage | ⭐ EASY |
| `statusEmoji` in ui | 10 min | +0.5% coverage | ⭐ EASY |
| Copy functions tests (3) in ui | 15 min | +0.5% coverage | ⭐ EASY |

**Total Low-Hanging Fruit:** ~90 minutes of work → +15-20% coverage on these packages

