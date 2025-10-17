// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spanner

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/spanner/internal"
	"go.opentelemetry.io/otel"
	otelcodes "go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/status"
)

const (
	defaultTracerName = "cloud.google.com/go/spanner"
	gcpClientRepo     = "googleapis/google-cloud-go"
	gcpClientArtifact = "cloud.google.com/go/spanner"
)

func tracerProvider(config *ClientConfig) {
	if !config.EnableOpenTelemetryTracing {
		config.TracerProvider = noop.NewTracerProvider()
	}

	if config.TracerProvider == nil {
		config.TracerProvider = otel.GetTracerProvider()
	}
}

func tracer(tp trace.TracerProvider) trace.Tracer {
	return tp.Tracer(defaultTracerName, trace.WithInstrumentationVersion(internal.Version))
}

// startSpan creates a span and a context.Context containing the newly-created span.
// If the context.Context provided in `ctx` contains a span then the newly-created
// span will be a child of that span, otherwise it will be a root span.
func startSpan(ctx context.Context, name string, tp trace.TracerProvider, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if tp == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	name = prependPackageName(name)
	ctx, span := tracer(tp).Start(ctx, name, opts...)
	return ctx, span
}

// endSpan retrieves the current span from ctx and completes the span.
// If an error occurs, the error is recorded as an exception span event for this span,
// and the span status is set in the form of a code and a description.
func endSpan(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if err != nil {
		span.SetStatus(otelcodes.Error, toOpenTelemetryStatusDescription(err))
		span.RecordError(err)
	}
	span.End()
}

// toOpenTelemetryStatus converts an error to an equivalent OpenTelemetry status description.
func toOpenTelemetryStatusDescription(err error) string {
	var err2 *googleapi.Error
	if ok := errors.As(err, &err2); ok {
		return err2.Message
	} else if s, ok := status.FromError(err); ok {
		return s.Message()
	} else {
		return err.Error()
	}
}

func prependPackageName(spanName string) string {
	return fmt.Sprintf("%s.%s", gcpClientArtifact, spanName)
}

func setOpenTelemetryTracerProvider(config *openTelemetryConfig, tp trace.TracerProvider) {
	config.tracerProvider = tp
}

func shutdownTracerProvider(ctx context.Context, tp trace.TracerProvider) error {
	if tp == nil {
		return nil
	}

	tc, ok := tp.(*sdktrace.TracerProvider)
	if !ok {
		return nil
	}

	return tc.Shutdown(ctx)
}
