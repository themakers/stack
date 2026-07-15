package stack_backend

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
	"unsafe"
)

// Value is a slog-style union for attribute values. Numbers, bools and
// durations are packed into num; strings are referenced via an unsafe
// pointer. Pointer-shaped values box into the any field without allocating,
// so common attribute kinds cost zero heap allocations — unlike the previous
// plain `any` field, where every escaping non-pointer value was boxed.
type Value struct {
	num uint64
	any any
}

// ValueKind describes which type a Value holds.
type ValueKind uint8

const (
	ValueKindAny ValueKind = iota
	ValueKindString
	ValueKindInt64
	ValueKindUint64
	ValueKindFloat64
	ValueKindBool
	ValueKindDuration
	ValueKindTime
	ValueKindError
	ValueKindRaw
)

// Pointer-shaped internal markers: boxing them into `any` does not allocate.
type (
	stringptr *byte // value string; num = len
	rawptr    *byte // RawAttrValue; num = len
	timeLoc   *time.Location
)

type valueKindMarker struct{ k ValueKind }

// Preallocated markers for num-carried kinds (a pointer to a global boxes
// for free).
var (
	markerInt64    = &valueKindMarker{ValueKindInt64}
	markerUint64   = &valueKindMarker{ValueKindUint64}
	markerFloat64  = &valueKindMarker{ValueKindFloat64}
	markerBool     = &valueKindMarker{ValueKindBool}
	markerDuration = &valueKindMarker{ValueKindDuration}
)

func StringValue(v string) Value {
	return Value{num: uint64(len(v)), any: stringptr(unsafe.StringData(v))}
}

func Int64Value(v int64) Value { return Value{num: uint64(v), any: markerInt64} }

func Uint64Value(v uint64) Value { return Value{num: v, any: markerUint64} }

func Float64Value(v float64) Value {
	return Value{num: math.Float64bits(v), any: markerFloat64}
}

func BoolValue(v bool) Value {
	var n uint64
	if v {
		n = 1
	}
	return Value{num: n, any: markerBool}
}

func DurationValue(v time.Duration) Value {
	return Value{num: uint64(v), any: markerDuration}
}

// TimeValue packs the wall time into num and the location into any (the
// monotonic clock reading is dropped, as in slog). Times outside the
// UnixNano range (before 1678 / after 2261, including the zero Time) take
// the boxed fallback.
func TimeValue(v time.Time) Value {
	if y := v.Year(); y < 1678 || y > 2261 {
		return Value{any: v}
	}
	return Value{num: uint64(v.UnixNano()), any: timeLoc(v.Location())}
}

// ErrorValue stores the error interface as-is: an interface-to-interface
// conversion copies two words without allocating. Kind() recognizes any
// error-typed payload as ValueKindError.
func ErrorValue(err error) Value { return Value{any: err} }

// RawValue marks a pre-rendered value that text renderers print verbatim.
func RawValue(v RawAttrValue) Value {
	s := string(v)
	return Value{num: uint64(len(s)), any: rawptr(unsafe.StringData(s))}
}

// AnyValue converts an arbitrary value, dispatching common types to their
// packed representations. Note for hot paths: the value is already boxed at
// the caller (the parameter is `any`), and for the default branch it leaks
// into the result — prefer the generic stack.F, which avoids boxing entirely.
func AnyValue(v any) Value {
	switch v := v.(type) {
	case string:
		return StringValue(v)
	case int:
		return Int64Value(int64(v))
	case int8:
		return Int64Value(int64(v))
	case int16:
		return Int64Value(int64(v))
	case int32:
		return Int64Value(int64(v))
	case int64:
		return Int64Value(v)
	case uint:
		return Uint64Value(uint64(v))
	case uint8:
		return Uint64Value(uint64(v))
	case uint16:
		return Uint64Value(uint64(v))
	case uint32:
		return Uint64Value(uint64(v))
	case uint64:
		return Uint64Value(v)
	case bool:
		return BoolValue(v)
	case float32:
		return Float64Value(float64(v))
	case float64:
		return Float64Value(v)
	case time.Duration:
		return DurationValue(v)
	case time.Time:
		return TimeValue(v)
	case RawAttrValue:
		return RawValue(v)
	case Value:
		return v
	default:
		// error is intentionally not special-cased: storing it as-is keeps
		// the interface identity, and Kind() classifies it as ValueKindError.
		return Value{any: v}
	}
}

func (v Value) Kind() ValueKind {
	switch a := v.any.(type) {
	case stringptr:
		return ValueKindString
	case rawptr:
		return ValueKindRaw
	case timeLoc:
		return ValueKindTime
	case *valueKindMarker:
		return a.k
	case time.Time:
		return ValueKindTime
	case error:
		return ValueKindError
	default:
		return ValueKindAny
	}
}

// String returns the held string for ValueKindString/ValueKindRaw; for other
// kinds it formats the value (a convenience fallback for cold paths).
func (v Value) String() string {
	switch a := v.any.(type) {
	case stringptr:
		return unsafe.String(a, v.num)
	case rawptr:
		return unsafe.String(a, v.num)
	default:
		return fmt.Sprint(v.Any())
	}
}

func (v Value) Int64() int64 { return int64(v.num) }

func (v Value) Uint64() uint64 { return v.num }

func (v Value) Float64() float64 { return math.Float64frombits(v.num) }

func (v Value) Bool() bool { return v.num != 0 }

func (v Value) Duration() time.Duration { return time.Duration(v.num) }

func (v Value) Time() time.Time {
	switch a := v.any.(type) {
	case timeLoc:
		return time.Unix(0, int64(v.num)).In((*time.Location)(a))
	case time.Time:
		return a
	default:
		return time.Time{}
	}
}

func (v Value) Error() error {
	if err, ok := v.any.(error); ok {
		return err
	}
	return nil
}

// Any unpacks the value back into an interface (allocates for packed kinds —
// intended for cold paths and interop, not for hot rendering).
func (v Value) Any() any {
	switch a := v.any.(type) {
	case stringptr:
		return unsafe.String(a, v.num)
	case rawptr:
		return RawAttrValue(unsafe.String(a, v.num))
	case timeLoc:
		return v.Time()
	case *valueKindMarker:
		switch a.k {
		case ValueKindInt64:
			return int64(v.num)
		case ValueKindUint64:
			return v.num
		case ValueKindFloat64:
			return v.Float64()
		case ValueKindBool:
			return v.Bool()
		case ValueKindDuration:
			return v.Duration()
		}
		return nil
	default:
		return v.any
	}
}

// MarshalJSON is required for the json backend, which marshals the whole
// Event: a union with unexported fields would render as {} otherwise.
func (v Value) MarshalJSON() ([]byte, error) {
	switch v.Kind() {
	case ValueKindString, ValueKindRaw:
		return json.Marshal(v.String())
	case ValueKindInt64:
		return json.Marshal(v.Int64())
	case ValueKindUint64:
		return json.Marshal(v.Uint64())
	case ValueKindFloat64:
		return json.Marshal(v.Float64())
	case ValueKindBool:
		return json.Marshal(v.Bool())
	case ValueKindDuration:
		return json.Marshal(v.Duration().String())
	case ValueKindTime:
		return json.Marshal(v.Time())
	case ValueKindError:
		if err := v.Error(); err != nil {
			return json.Marshal(err.Error())
		}
		return []byte("null"), nil
	default:
		data, err := json.Marshal(v.any)
		if err != nil {
			return json.Marshal("<attr marshal error>")
		}
		return data, nil
	}
}
