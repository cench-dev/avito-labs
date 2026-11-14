package observability

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

type OTelConfig struct {
	Endpoint string
	Protocol string
	Sampler  string
}

type OTel struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
}

func InitOTel(ctx context.Context, cfg OTelConfig) (*OTel, error) {
	protocol := strings.ToLower(strings.TrimSpace(cfg.Protocol))
	if protocol == "" {
		protocol = "grpc"
	}

	sampler, err := newSampler(cfg.Sampler)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(semconv.ServiceName("trip-service")),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTel resource: %w", err)
	}

	traceOptions := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	}
	metricOptions := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}

	if strings.TrimSpace(cfg.Endpoint) != "" {
		switch protocol {
		case "grpc":
			traceExporter, err := otlptracegrpc.New(ctx)
			if err != nil {
				return nil, fmt.Errorf("create trace exporter: %w", err)
			}

			metricExporter, err := otlpmetricgrpc.New(ctx)
			if err != nil {
				return nil, fmt.Errorf("create metric exporter: %w", err)
			}

			traceOptions = append(traceOptions, sdktrace.WithBatcher(traceExporter))
			metricOptions = append(
				metricOptions,
				sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
			)

		case "http/protobuf":
			traceExporter, err := otlptracehttp.New(ctx)
			if err != nil {
				return nil, fmt.Errorf("create trace exporter: %w", err)
			}

			metricExporter, err := otlpmetrichttp.New(ctx)
			if err != nil {
				return nil, fmt.Errorf("create metric exporter: %w", err)
			}

			traceOptions = append(traceOptions, sdktrace.WithBatcher(traceExporter))
			metricOptions = append(
				metricOptions,
				sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
			)

		default:
			return nil, fmt.Errorf(
				"unsupported OTEL_EXPORTER_OTLP_PROTOCOL %q",
				protocol,
			)
		}
	}

	tp := sdktrace.NewTracerProvider(traceOptions...)
	mp := sdkmetric.NewMeterProvider(metricOptions...)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	return &OTel{
		TracerProvider: tp,
		MeterProvider:  mp,
	}, nil
}

func (o *OTel) Shutdown(ctx context.Context) error {
	var firstErr error

	if err := o.MeterProvider.Shutdown(ctx); err != nil {
		firstErr = fmt.Errorf("shutdown meter provider: %w", err)
	}

	if err := o.TracerProvider.Shutdown(ctx); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("shutdown tracer provider: %w", err)
	}

	return firstErr
}

func newSampler(name string) (sdktrace.Sampler, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), nil
	case "always_on":
		return sdktrace.AlwaysSample(), nil
	case "always_off":
		return sdktrace.NeverSample(), nil
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(0.1), nil
	default:
		return nil, fmt.Errorf("unsupported OTEL_TRACES_SAMPLER %q", name)
	}
}
