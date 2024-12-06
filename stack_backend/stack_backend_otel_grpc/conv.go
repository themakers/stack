package stack_backend_otel_grpc

import (
	"fmt"
	"slices"

	common_model_v1 "go.opentelemetry.io/proto/otlp/common/v1"

	"github.com/themakers/stack/stack_backend"
)

func attrsToKeyValue(attrs ...[]stack_backend.Attr) (kv []*common_model_v1.KeyValue) {
	for _, attr := range slices.Concat(attrs...) {
		kv = append(kv, &common_model_v1.KeyValue{
			Key: attr.Name,
			Value: &common_model_v1.AnyValue{
				Value: &common_model_v1.AnyValue_StringValue{
					StringValue: fmt.Sprint(attr.Value),
				},
			},
		})
	}

	return kv
}
