package buffer

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/go-logfmt/logfmt"
)

// Use the actual logger and WAL, not a mock encoder or capture subsystem.
// DEL must be escaped on the wire, but survive JSON persistence and logfmt
// decoding just like tabs, newlines, quotes, backslashes and Unicode.
func TestLogfmtControlCharactersSurviveWAL(t *testing.T) {
	var output bytes.Buffer
	logger := log.NewLogfmtLogger(&output)
	want := "synthetic\x7f text\t\n\"quoted\" \\ café 😀"
	if err := logger.Log("msg", want, "win\x7fdow", "fixture", "empty", ""); err != nil {
		t.Fatal(err)
	}
	wire := output.String()
	if strings.ContainsRune(wire, '\x7f') || !strings.Contains(wire, `\u007f`) {
		t.Fatalf("DEL must be escaped in logfmt: %q", wire)
	}
	if strings.Count(wire, "\n") != 1 {
		t.Fatalf("log injection: expected one physical record: %q", wire)
	}
	wal, err := NewWAL(t.TempDir(), 1<<20, log.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = wal.Close() })
	entry := Entry{Ts: 1750000000123456789, Labels: map[string]string{"job": "fixture"}, Line: strings.TrimSuffix(wire, "\n")}
	if err := wal.Append(entry); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := wal.ListSegments()
	if err != nil || len(files) != 1 {
		t.Fatalf("segments = %v, %v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var got Entry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Ts != entry.Ts || got.Line != entry.Line || got.Labels["job"] != "fixture" {
		t.Fatalf("WAL changed record: %+v", got)
	}
	decoder := logfmt.NewDecoder(strings.NewReader(got.Line))
	if !decoder.ScanRecord() {
		t.Fatal("missing logfmt record", decoder.Err())
	}
	values := map[string]string{}
	for decoder.ScanKeyval() {
		values[string(decoder.Key())] = string(decoder.Value())
	}
	if decoder.Err() != nil || decoder.ScanRecord() {
		t.Fatal("unexpected decoder state", decoder.Err())
	}
	if len(values) != 3 || values["msg"] != want || values["window"] != "fixture" || values["empty"] != "" {
		t.Fatalf("logfmt roundtrip = %#v", values)
	}
}
