package metrics

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

func TestNewResourceUsesConfiguredAttributes(t *testing.T) {
	t.Setenv(
		"OTEL_RESOURCE_ATTRIBUTES",
		"service.namespace=custom,service.name=custom-thumbnailer,"+
			"service.instance.id=replica-1",
	)

	res, err := newResource(context.Background())
	if err != nil {
		t.Fatalf("newResource() error = %v", err)
	}

	assertResourceAttribute(t, res, semconv.ServiceNamespaceKey, "custom")
	assertResourceAttribute(t, res, semconv.ServiceNameKey, "custom-thumbnailer")
	assertResourceAttribute(t, res, semconv.ServiceInstanceIDKey, "replica-1")
}

func TestNewResourceUsesFallbacks(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")

	res, err := newResource(context.Background())
	if err != nil {
		t.Fatalf("newResource() error = %v", err)
	}

	assertResourceAttribute(t, res, semconv.ServiceNamespaceKey, "galleries")
	assertResourceAttribute(t, res, semconv.ServiceNameKey, "thumbnailer")

	instanceID, found := res.Set().Value(semconv.ServiceInstanceIDKey)
	if !found || instanceID.AsString() == "" {
		t.Fatal("service.instance.id is missing or empty")
	}
}

func assertResourceAttribute(
	t *testing.T,
	res *sdkresource.Resource,
	key attribute.Key,
	want string,
) {
	t.Helper()

	got, found := res.Set().Value(key)
	if !found {
		t.Fatalf("resource attribute %q is missing, want %q", key, want)
	}
	if got.AsString() != want {
		t.Fatalf("resource attribute %q = %q, want %q", key, got.AsString(), want)
	}
}
