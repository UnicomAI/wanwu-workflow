package trace_util

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	envJaegerEnable       = "JAEGER_ENABLE"
	envJaegerOTLPEndpoint = "JAEGER_OTLP_ENDPOINT"
)

var _tracer = &Tracer{}

type Tracer struct {
	tp *sdktrace.TracerProvider
}

// InitTracer initializes the TracerProvider.
// When JAEGER_ENABLE is unset or false: no exporter, spans are discarded (backward compatible).
// When JAEGER_ENABLE=true + JAEGER_OTLP_ENDPOINT: exports spans via OTLP HTTP to Jaeger.
func InitTracer(serviceName string) error {
	enabled, _ := strconv.ParseBool(os.Getenv(envJaegerEnable))

	var tp *sdktrace.TracerProvider
	if enabled {
		endpoint := os.Getenv(envJaegerOTLPEndpoint)
		if endpoint == "" {
			return errors.New("JAEGER_ENABLE=1 but JAEGER_OTLP_ENDPOINT is empty")
		}
		var err error
		tp, err = initOTLPTracerProvider(serviceName, endpoint)
		if err != nil {
			return err
		}
		logs.Infof("[trace] tracer initialized with OTLP exporter, endpoint=%s, service=%s", endpoint, serviceName)
	} else {
		tp = initDefaultTracerProvider()
		logs.Infof("[trace] tracer initialized (no exporter, spans discarded)")
	}

	_tracer.tp = tp
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return nil
}

// ShutdownTracer flushes remaining spans before process exit.
func ShutdownTracer(ctx context.Context) {
	if _tracer.tp != nil {
		if err := _tracer.tp.Shutdown(ctx); err != nil {
			logs.Errorf("[trace] tracer shutdown error: %v", err)
		}
	}
}

func GetTraceID(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	return spanCtx.TraceID().String()
}

// initDefaultTracerProvider creates a TracerProvider without exporter (fallback mode).
func initDefaultTracerProvider() *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
}

// initOTLPTracerProvider creates a TracerProvider with OTLP HTTP exporter for Jaeger.
func initOTLPTracerProvider(serviceName, endpoint string) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTLP HTTP exporter failed: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		logs.Warnf("[trace] resource.Merge failed: %v, using fallback resource", err)
		res = resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	), nil
}
