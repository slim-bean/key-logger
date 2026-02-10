
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

```
./key-logger \
  --output=loki \
  --client.url=http://localhost:3100/loki/api/v1/push \
  --label job=keylogger \
  --label host=myhost
```

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
| `--enable-keylogger` | `true` | Enable keystroke logging |
| `--enable-window-tracker` | `true` | Enable active window tracking |
| `--enable-screencap` | `true` | Enable screenshot capture |
| `--screencap-interval` | `5s` | Screenshot capture interval |
| `--idle-timeout` | `5m` | Idle time before pausing capture |
| `--s3-endpoint` | | S3 URL for screenshot uploads |
| `--bucket` | | S3 bucket name |
| `--accessKey` | | S3 access key |
| `--secretKey` | | S3 secret key |

## macOS permissions

On macOS the following permissions are needed (grant in System Settings > Privacy & Security):

- **Accessibility**: Required for keystroke logging and window title detection
- **Screen Recording**: Required for screenshot capture and window name fallback via CGWindowList
