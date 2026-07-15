package stack_backend_otel

import (
	"fmt"

	common_model_v1 "go.opentelemetry.io/proto/otlp/common/v1"

	"github.com/themakers/stack/stack_backend"
)

// attrsToKeyValue converts one attribute slice into OTLP KeyValue. The result
// is preallocated to the input length (slices.Concat used to create a
// redundant intermediate copy).
func attrsToKeyValue(attrs []stack_backend.Attr) []*common_model_v1.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	kv := make([]*common_model_v1.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		kv = append(kv, &common_model_v1.KeyValue{
			Key: attr.Name,
			Value: &common_model_v1.AnyValue{
				Value: &common_model_v1.AnyValue_StringValue{
					StringValue: otlpString(attr.Value),
				},
			},
		})
	}
	return kv
}

// otlpString formats an attribute value into a string with a fast path for
// common types (no fmt reflection for strings).
func otlpString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case stack_backend.RawAttrValue:
		return string(val)
	case error:
		return val.Error()
	default:
		return fmt.Sprint(v)
	}
}
