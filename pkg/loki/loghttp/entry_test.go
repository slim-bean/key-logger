package loghttp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
)

// Exercise the jsonparser path used for individual Loki entries, not only
// jsoniter's separate streaming decoder. All payloads are synthetic.
func TestEntryUnmarshalJSONUnicodeAndTimestamp(t *testing.T) {
	for _, ts := range []int64{0, -1, -9223372036854775808, 9223372036854775807} {
		for _, rawLine := range []string{`"synthetic\nline"`, `"\uD83D\uDE00"`, `"\uDB29A7FA"`, `"\uDC00"`, `"long-padding-to-cross-eight-byte-boundaries-\"quoted\""`} {
			t.Run(fmt.Sprintf("%d/%s", ts, rawLine), func(t *testing.T) {
				var want string
				if err := json.Unmarshal([]byte(rawLine), &want); err != nil {
					t.Fatal(err)
				}
				payload := []byte(`["` + strconv.FormatInt(ts, 10) + `",` + rawLine + `]`)
				var entry Entry
				if err := entry.UnmarshalJSON(payload); err != nil {
					t.Fatal(err)
				}
				if entry.Timestamp.UnixNano() != ts || entry.Line != want {
					t.Fatalf("got (%d, %q), want (%d, %q)", entry.Timestamp.UnixNano(), entry.Line, ts, want)
				}
			})
		}
	}
}

func TestEntryUnmarshalJSONRejectsMalformedInput(t *testing.T) {
	for _, payload := range []string{
		`["-","synthetic"]`,
		`["9223372036854775808","synthetic"]`,
		`["-9223372036854775809","synthetic"]`,
		`["0","bad\qescape"]`,
		`["0",42]`,
		`{"0","synthetic"]`,
	} {
		t.Run(payload, func(t *testing.T) {
			var entry Entry
			if err := entry.UnmarshalJSON([]byte(payload)); err == nil {
				t.Fatal("accepted malformed entry")
			}
		})
	}
}
