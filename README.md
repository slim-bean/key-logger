
# key-logger

A key logger which outputs to a Loki-friendly logfmt format. Supports Windows and macOS.

## Build

```
go build -o key-logger ./cmd/key-logger/
```

## Usage

### Stdout (default)

Output goes to stdout as logfmt. Can be piped to any log collector.

```
./key-logger
```

### Direct to Loki

Use `--output=loki` to send log events directly to Loki. At least one `--label` is required.
All events are written to a disk-backed WAL buffer first, then sent by a background
goroutine. This means data survives process restarts and network outages.

```
./key-logger \
  --output=loki \
  --client.url=http://localhost:3100/loki/api/v1/push \
  --label host=myhost
```

The `job` label is set automatically per event type: `keylogger` for keystroke
events, `window` for active window events, and `screencap` for screenshot metadata.

#### Disk buffer

When `--output=loki` is active, entries are buffered in segment files on disk
(default `~/.key-logger/buffer/`). A background sender reads segments and pushes
to Loki via the HTTP push API. If Loki is unreachable, entries accumulate on disk
and are replayed when connectivity returns.

- On **429** (rate limited): respects `Retry-After` header, otherwise exponential backoff (1s to 5m)
- On **5xx / network errors**: exponential backoff with jitter
- On process restart: resumes from saved cursor position (no duplicate sends)
- Buffer size is capped (default 100MB); oldest segments are dropped when exceeded

### Filters

Use `--filter` to remove sensitive text from the output. Each filter is a Go regex
applied to all string values. Filters work with both stdout and Loki output.

```
./key-logger \
  --output=loki \
  --client.url=http://localhost:3100/loki/api/v1/push \
  --label job=keylogger \
  --label host=myhost \
  --filter "(?i)password" \
  --filter "(?i)secret"
```

Filters use `regexp.ReplaceAllString`, so the matched substring is removed.
Use broader patterns to blank the entire text field:

```
--filter "(?i).*password.*"
```

Since filters are Go regexes passed through the shell, special characters need
escaping. Use single quotes to avoid shell interpretation, and backslashes for
regex metacharacters:

```
--filter '(?i)p@ss\.word'          # literal dot (matches "p@ss.word" not "p@ssTword")
--filter '(?i)secret\d+'           # \d+ matches one or more digits ("secret123")
--filter '(?i)hunter2|correcthorse' # alternation: matches either word
--filter 'mypass\!123'             # literal exclamation mark
```

### Feature toggles

Individual subsystems can be enabled/disabled:

```
./key-logger --enable-keylogger=false    # disable keystroke logging
./key-logger --enable-window-tracker=false  # disable active window tracking
./key-logger --enable-screencap=false    # disable screenshot capture
```

### All flags

| Flag | Default | Description |
|------|---------|-------------|
| `--output` | `stdout` | Output destination: `stdout` or `loki` |
| `--label` | | Label as `key=value` for Loki (repeatable, required with `--output=loki`) |
| `--filter` | | Regex filter to remove matching text (repeatable) |
| `--client.url` | | Loki push endpoint (required with `--output=loki`) |
| `--client.tenant-id` | | Loki tenant ID (for multi-tenant setups) |
| `--buffer-dir` | `~/.key-logger/buffer/` | Directory for WAL buffer files |
| `--buffer-max-mb` | `100` | Maximum buffer size in megabytes |
| `--enable-keylogger` | `true` | Enable keystroke logging |
| `--enable-window-tracker` | `true` | Enable active window tracking |
| `--enable-screencap` | `true` | Enable screenshot capture |
| `--screencap-interval` | `5s` | Screenshot capture interval |
| `--idle-timeout` | `5m` | Idle time before pausing capture |
| `--s3-endpoint` | | S3 URL for screenshot uploads |
| `--bucket` | | S3 bucket name |
| `--accessKey` | | S3 access key |
| `--secretKey` | | S3 secret key |

## Running as a macOS service

The project includes a launchd LaunchAgent setup so key-logger starts automatically
on login, restarts on crash, and runs in the background.

### Quick start

```
make config       # creates local plist from template (first time only)
# edit com.keylogger.agent.plist — fill in your Loki URL, hostname, etc.
make install      # builds, installs binary to /usr/local/bin, loads the service
```

### Setup details

1. **`make config`** copies `service/com.keylogger.agent.plist.template` to
   `com.keylogger.agent.plist` if the local copy doesn't exist. The local copy
   is gitignored so your credentials and machine-specific config stay out of version control.

2. **Edit `com.keylogger.agent.plist`** and replace the placeholder values
   (`LOKI_URL`, `HOSTNAME`) with your real configuration. Add any extra flags
   (filters, S3 credentials, feature toggles) as additional `<string>` entries
   inside the `ProgramArguments` array.

3. **`make install`** builds the binary, copies it to `/usr/local/bin/key-logger`,
   installs the plist to `~/Library/LaunchAgents/`, and loads the service.
   It will refuse to install if placeholder values are still present.

### Managing the service

```
make start        # start the service
make stop         # stop the service
make restart      # stop + start
make status       # print service status
make logs         # tail stdout/stderr logs
make uninstall    # stop service, remove plist and binary
```

### Logs

stdout and stderr are written to `/tmp/key-logger.stdout.log` and
`/tmp/key-logger.stderr.log`. These are cleared on reboot. Change the paths
in your local plist if you want persistent logs (e.g. `~/.key-logger/`).

When using `--output=loki`, the primary data goes to Loki; these files are
mainly useful for debugging startup issues.

## macOS permissions

On macOS the following permissions are needed (grant in System Settings > Privacy & Security):

- **Accessibility**: Required for keystroke logging and window title detection
- **Screen Recording**: Required for screenshot capture and window name fallback via CGWindowList

After installing the service, grant these permissions to `/usr/local/bin/key-logger`.
If you rebuild and reinstall the binary, macOS may revoke permissions and you'll need
to re-grant them.
