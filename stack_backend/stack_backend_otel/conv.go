package stack_backend_otel

import (
	"fmt"
	"math"
	"strconv"
	"time"

	common_model_v1 "go.opentelemetry.io/proto/otlp/common/v1"

	"github.com/themakers/stack/stack_backend"
)

// attrsToKeyValue converts one attribute slice into OTLP KeyValue. The result
// is preallocated to the input length (slices.Concat used to create a
// redundant intermediate copy). Typed Value kinds map to native OTLP value
// types instead of being stringified.
func attrsToKeyValue(attrs []stack_backend.Attr) []*common_model_v1.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	kv := make([]*common_model_v1.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		kv = append(kv, &common_model_v1.KeyValue{
			Key:   attr.Name,
			Value: otlpValue(attr.Value),
		})
	}
	return kv
}

func otlpValue(v stack_backend.Value) *common_model_v1.AnyValue {
	switch v.Kind() {
	case stack_backend.ValueKindString, stack_backend.ValueKindRaw:
		return otlpStr(v.String())
	case stack_backend.ValueKindInt64:
		return &common_model_v1.AnyValue{Value: &common_model_v1.AnyValue_IntValue{IntValue: v.Int64()}}
	case stack_backend.ValueKindUint64:
		if n := v.Uint64(); n <= math.MaxInt64 {
			return &common_model_v1.AnyValue{Value: &common_model_v1.AnyValue_IntValue{IntValue: int64(n)}}
		}
		return otlpStr(strconv.FormatUint(v.Uint64(), 10))
	case stack_backend.ValueKindFloat64:
		return &common_model_v1.AnyValue{Value: &common_model_v1.AnyValue_DoubleValue{DoubleValue: v.Float64()}}
	case stack_backend.ValueKindBool:
		return &common_model_v1.AnyValue{Value: &common_model_v1.AnyValue_BoolValue{BoolValue: v.Bool()}}
	case stack_backend.ValueKindDuration:
		return otlpStr(v.Duration().String())
	case stack_backend.ValueKindTime:
		return otlpStr(v.Time().Format(time.RFC3339Nano))
	case stack_backend.ValueKindError:
		if err := v.Error(); err != nil {
			return otlpStr(err.Error())
		}
		return otlpStr("")
	default:
		return otlpStr(fmt.Sprint(v.Any()))
	}
}

func otlpStr(s string) *common_model_v1.AnyValue {
	return &common_model_v1.AnyValue{
		Value: &common_model_v1.AnyValue_StringValue{StringValue: s},
	}
}
