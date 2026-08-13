package stack_backend_otel

import (
	"testing"

	"github.com/themakers/stack/stack_backend"
)

// Structs, maps and slices must arrive as JSON string attributes — the same
// shape Value.MarshalJSON gives the json backend — not as fmt.Sprint output.
func TestOTLPValueAnyIsJSON(t *testing.T) {
	type payload struct {
		Bitrate float64 `json:"bitrate_bps"`
		Frames  uint64  `json:"frames"`
		Media   string  `json:"media"`
	}

	v := stack_backend.AnyValue(payload{Bitrate: 1500.5, Frames: 42, Media: "video"})

	got := otlpValue(v).GetStringValue()
	want := `{"bitrate_bps":1500.5,"frames":42,"media":"video"}`
	if got != want {
		t.Errorf("otlpValue(struct) = %q, want %q", got, want)
	}
}

func TestOTLPValueAnyMap(t *testing.T) {
	v := stack_backend.AnyValue(map[string]int{"a": 1})

	got := otlpValue(v).GetStringValue()
	want := `{"a":1}`
	if got != want {
		t.Errorf("otlpValue(map) = %q, want %q", got, want)
	}
}

// Unmarshalable values keep the fmt.Sprint fallback instead of dropping the
// attribute.
func TestOTLPValueAnyUnmarshalableFallsBack(t *testing.T) {
	v := stack_backend.AnyValue(func() {})

	if got := otlpValue(v).GetStringValue(); got == "" {
		t.Error("otlpValue(func) = empty, want fmt.Sprint fallback")
	}
}
