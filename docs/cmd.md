# Module Documentation: `cmd`

The `cmd` module implements the Auvis CLI using [Cobra](https://github.com/spf13/cobra). It provides the developer-facing command surface: starting the gateway, validating policies, scaffolding new projects, tailing audit logs, and querying gateway status.

---

## 📂 File Index

- [`root.go`](#1-rootgo) — Root command, global flags, subcommand registration, shared helpers
- [`dev.go`](#2-devgo) — `auvis dev` — gateway startup, wiring, graceful shutdown
- [`init.go`](#3-initgo) — `auvis init` — project scaffolding
- [`validate.go`](#4-validatego) — `auvis validate` — policy lint and correctness checking
- [`status.go`](#5-statusgo) — `auvis status` — gateway health check
- [`logs.go`](#6-logsgo) — `auvis logs` — audit log viewing and filtering
- [`policies.go`](#7-policiesgo) — `auvis policies list` — policy introspection
- [`version.go`](#8-versiongo) — `auvis version` — version output

---

## 1. `root.go`

### `const version = "0.2.5"`

Current binary release version. Displayed by `auvis --version` and `auvis version`.

---

### Package-level variables

#### Global flags (persistent — available on all subcommands)

| Flag | Short | Default | Description |
|---|---|---|---|
| `--config` | `-c` | `"auvis.yaml"` | Path to the YAML configuration file |
| `--policies` | | `"./policies"` | Path to the policies directory |
| `--verbose` | `-v` | `false` | Enable verbose output |

These are stored in package-level variables: `cfgFile`, `policiesDir`, `verbose`.

#### Color helpers

`bold`, `green`, `red`, `yellow`, `cyan`, `dimmed` — `github.com/fatih/color` instances. Automatically disabled when stdout is not a TTY.

---

### `rootCmd`

Base `cobra.Command`. Running `auvis` with no subcommand invokes `cmd.Help()`.

---

### `Execute()`

Entry point called from `main.go`. Calls `rootCmd.Execute()`. Prints red error to stderr and exits with code `1` on failure.

---

### `init()` *(auto-invoked by Go runtime)*

Registers all global flags on `rootCmd`. Sets version template. Registers all subcommands:
- `initCmd`, `validateCmd`, `devCmd`, `statusCmd`, `logsCmd`, `versionCmd`
- `policiesCmd` with `policiesListCmd` as a sub-subcommand

---

### Helper functions

| Function | Output | Description |
|---|---|---|
| `printSuccess(format, a...)` | Green `✓ message` | Positive outcome line |
| `printError(format, a...)` | Red `✗ message` | Error line to stderr |
| `printWarning(format, a...)` | Yellow `⚠ message` | Warning line |
| `printInfo(format, a...)` | Cyan `→ message` | Info/progress line |
| `printDimmed(format, a...)` | Faint `  message` | Secondary/detail line |
| `separator()` | Faint `────────────────────────────────────────────────────` | Section divider |

---

## 2. `dev.go`

### `auvis dev`

Starts the gateway in development mode with live policy reloading. This is the primary command for running Auvis.

---

### `runDev() error`

**Full startup sequence:**

1. **Logging**: Initialises `slog` with `slog.NewTextHandler(os.Stderr, {Level: slog.LevelInfo})`.

2. **Config**: Calls `config.LoadConfig(cfgFile)`. Fails if `cfg.UpstreamURL == ""`.

3. **Automatic Policy Validation**: Calls `performPolicyValidation(cfg.PoliciesDir)`. Validates YAML syntax, rule structures, condition expressions, and checks for duplicate policy names. Aborts startup (`FATAL`) if policy validation fails.

4. **Audit Logger**: Calls `audit.NewRotatingLogger(cfg.AuditFile, 512, cfg.AuditMaxSizeMB, cfg.AuditMaxAgeDays)`.

5. **Policy Store**: Calls `policy.NewStore()`. Calls `store.LoadDir(cfg.PoliciesDir)`. Calls `store.WatchDir(cfg.PoliciesDir)` to enable hot-reload.

6. **Destination Setup**: Calls `buildDestinations(cfg.Destinations, skipDestinationCheck)`:
   - Runs `d.Connect()` reachability and auth check for every destination.
   - If `--skip-destination-check` is set, logs `slog.Warn` on connection failure and proceeds anyway.
   - If a connection fails without `--skip-destination-check`, cleans up opened pools, logs `FATAL`, and aborts gateway boot.

7. **Transform function**: `transform.NewStreamApplyFn(store)`.

8. **HTTP ServeMux**:
   - `audit.RegisterHandlers(mux, ...)` → mounts `/auvis/audit`, `/auvis/status`, `/metrics`, `/auvis/metrics`.
   - `GET /healthz` & `GET /health` → **Liveness Probe**: returns `200 OK` when process is running and serving traffic. Excluded from proxy policy evaluation and audit logging.
   - `GET /readyz` & `GET /ready` → **Readiness Probe**: returns `200 OK` (`"ready"` or `"degraded"`) when policies are loaded and circuit breakers are operational; returns `503 Service Unavailable` if 0 policies are loaded or all destination breakers are `Open`. Excluded from proxy policy evaluation and audit logging.
   - `/` → `proxy.Handler(cfg, store, transformFn, auditLog, &requestCount, dest)`.

9. **Startup Summary & HTTP Server**:
   - Prints clean terminal startup summary via `printStartupSummary()` (listening port, upstream URL, policies loaded, connected destinations, audit file path).
   - Starts HTTP server on `Addr: ":"+cfg.Port`.

10. **Graceful Shutdown**:
   - Listens for `os.Interrupt` or `syscall.SIGTERM`.
   - On signal: calls `server.Shutdown(ctx)` with 10-second timeout.
   - Calls `auditLog.Close()` to flush pending audit entries.
   - Logs `slog.Info("Gateway gracefully terminated", "total_requests", N)`.

---

### `buildDestinations(dcs []config.DestinationConfig) ([]pipeline.Destination, []string, error)`

Iterates all destination configs. For each:
1. Calls `buildDestination(dc)` to construct the driver.
2. Calls `d.Connect()` to establish connection.
3. Appends to `dests` and `names` slices.

Returns on the first error with descriptive context (`"destination[N] <type>: ..."` or `"destination[N] <type> connect: ..."`).

---

### `buildDestination(dc config.DestinationConfig) (pipeline.Destination, error)`

Switch on `dc.Type`:

| Type | Constructor |
|---|---|
| `"postgres"` | `destination.NewPostgresDestination(dc.Settings, dc.Auth, dc.TLS)` |
| `"pinecone"` | `destination.NewPineconeDestination(dc.Settings, dc.Auth, dc.TLS)` |
| `"weaviate"` | `destination.NewWeaviateDestination(dc.Settings, dc.Auth, dc.TLS)` |
| `"pgvector"` | `destination.NewPgVectorDestination(dc.Settings, dc.Auth, dc.TLS)` |
| anything else | `error: "unknown destination type %q (supported: postgres, pinecone, weaviate, pgvector)"` |

---

## 3. `init.go`

### `auvis init`

Scaffolds a new Auvis project in the current directory:
- Creates `auvis.yaml` with example config.
- Creates `policies/` directory.
- Creates `policies/example.yaml` with a commented policy.
- Prints onboarding instructions.

---

## 4. `validate.go`

### `auvis validate`

Deep policy validation command. Checks every `.yaml`/`.yml` file in the policies directory.

**Validation checks performed:**
- YAML syntax.
- Required fields: `name`, `match.resource`, `match.roles`, `rules`.
- Valid `action` values.
- Valid `condition` expression syntax.
- Duplicate policy names within the same file.
- Overlap detection between policies with the same `role + resource` key.
- `filter_condition` expression syntax check.
- `data_path` existence check against a sample response structure (if provided).

Exits `0` on success, `1` on validation failures.

---

## 5. `status.go`

### `auvis status`

Queries `GET /auvis/status` on the running gateway. Prints:
- Gateway uptime.
- Total requests processed.
- List of active compiled policies with their rules.

---

## 6. `logs.go`

### `auvis logs`

Reads and renders the audit log file with filtering support.

**Flags:**
- `--limit N` — Show last N entries (default: 50).
- `--role analyst` — Filter by user role.
- `--user user-123` — Filter by user ID.
- `--resource /users` — Filter by resource path.
- `--follow` — Tail the log file continuously.

---

## 7. `policies.go`

### `auvis policies list`

Lists all compiled effective policies from the running gateway by calling `GET /auvis/status`. Renders as a formatted table showing role, resource, source policies, and applied rules.

---

## 8. `version.go`

### `auvis version`

Prints `auvis version X.Y.Z` and exits.
