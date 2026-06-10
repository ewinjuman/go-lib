// examples/otel/main.go
//
// Contoh integrasi OpenTelemetry (OTel) dengan go-lib.
//
// Mencakup:
//   - Setup TracerProvider dengan sampler + exporter ke stdout
//   - Custom httpclient middleware yang membuat span per request
//   - Propagasi W3C TraceContext header ke downstream service
//   - Integrasi trace_id/span_id ke dalam logger
//   - Penggunaan dengan AppContext
//
// Untuk production, ganti consoleExporter dengan:
//   - otlptracehttp: go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
//   - otlptracegrpc: go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
//   - jaeger:        go get go.opentelemetry.io/otel/exporters/jaeger
package main

//
//import (
//	"context"
//	"fmt"
//	"net/http"
//	"time"
//
//	"go.opentelemetry.io/otel"
//	"go.opentelemetry.io/otel/attribute"
//	otelcodes "go.opentelemetry.io/otel/codes"
//	"go.opentelemetry.io/otel/propagation"
//	"go.opentelemetry.io/otel/sdk/resource"
//	sdktrace "go.opentelemetry.io/otel/sdk/trace"
//	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
//	"go.opentelemetry.io/otel/trace"
//
//	"github.com/ewinjuman/go-lib/v2/appContext"
//	"github.com/ewinjuman/go-lib/v2/httpclient"
//	"github.com/ewinjuman/go-lib/v2/logger"
//)
//
//// ── OTel Setup ────────────────────────────────────────────────────────────────
//
//// consoleExporter menulis span ke stdout — hanya untuk demo/dev.
//// Di production ganti dengan otlptracehttp atau otlptracegrpc.
//type consoleExporter struct{}
//
//func (e *consoleExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
//	for _, s := range spans {
//		fmt.Printf("[TRACE] %-40s | trace=%s span=%s parent=%s dur=%v status=%s\n",
//			s.Name(),
//			s.SpanContext().TraceID(),
//			s.SpanContext().SpanID(),
//			s.Parent().SpanID(),
//			s.EndTime().Sub(s.StartTime()).Round(time.Millisecond),
//			s.Status().Code,
//		)
//	}
//	return nil
//}
//
//func (e *consoleExporter) Shutdown(_ context.Context) error { return nil }
//
//// setupOTel mengkonfigurasi global TracerProvider dan TextMapPropagator.
//// Kembalikan shutdown function dan defer di main().
//func setupOTel(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
//	res, err := resource.New(ctx,
//		resource.WithAttributes(
//			semconv.ServiceName(serviceName),
//			semconv.ServiceVersion("1.0.0"),
//		),
//	)
//	if err != nil {
//		return nil, fmt.Errorf("otel resource: %w", err)
//	}
//
//	tp := sdktrace.NewTracerProvider(
//		sdktrace.WithBatcher(&consoleExporter{}),
//		sdktrace.WithResource(res),
//		sdktrace.WithSampler(sdktrace.AlwaysSample()),
//	)
//
//	// Set global provider — pakai otel.Tracer("name") di mana saja setelah ini.
//	otel.SetTracerProvider(tp)
//
//	// W3C TraceContext + Baggage untuk propagasi lintas service.
//	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
//		propagation.TraceContext{},
//		propagation.Baggage{},
//	))
//
//	return tp.Shutdown, nil
//}
//
//// ── OTel Middleware untuk httpclient ─────────────────────────────────────────
//
//const OTelKey = "otel-tracing"
//
//// OTelMiddleware membuat child span untuk setiap HTTP request dan
//// menginjeksi W3C TraceContext header agar downstream service bisa join trace.
////
//// Penggunaan:
////
////	tracer := otel.Tracer("my-service")
////	client := httpclient.New(
////	    httpclient.WithMiddleware(OTelMiddleware(tracer)),
////	)
//func OTelMiddleware(tracer trace.Tracer) httpclient.Middleware {
//	return httpclient.NewMiddleware(OTelKey, func(next httpclient.Doer) httpclient.Doer {
//		return httpclient.DoerFunc(func(req *http.Request) (*http.Response, error) {
//			ctx := req.Context()
//
//			// Extract parent span dari context jika ada (incoming request sudah punya span).
//			ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(req.Header))
//
//			spanName := fmt.Sprintf("%s %s", req.Method, req.URL.Path)
//			ctx, span := tracer.Start(ctx, spanName,
//				trace.WithSpanKind(trace.SpanKindClient),
//				trace.WithAttributes(
//					attribute.String("http.method", req.Method),
//					attribute.String("http.url", req.URL.String()),
//					attribute.String("server.address", req.URL.Host),
//				),
//			)
//			defer span.End()
//
//			// Inject traceparent + tracestate ke outgoing request.
//			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
//
//			resp, err := next.Do(req.WithContext(ctx))
//			if err != nil {
//				span.RecordError(err)
//				span.SetStatus(otelcodes.Error, err.Error())
//				return resp, err
//			}
//
//			span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
//			if resp.StatusCode >= 400 {
//				span.SetStatus(otelcodes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
//			} else {
//				span.SetStatus(otelcodes.Ok, "")
//			}
//
//			return resp, nil
//		})
//	})
//}
//
//// ── Helper: ekstrak trace info ke logger fields ───────────────────────────────
//
//// traceFields mengambil trace_id dan span_id dari context untuk disertakan ke log.
//// Gunakan ini agar log dan trace bisa dikorelasikan di Jaeger/Grafana Tempo.
//func traceFields(ctx context.Context) []logger.Field {
//	sc := trace.SpanFromContext(ctx).SpanContext()
//	if !sc.IsValid() {
//		return nil
//	}
//	return []logger.Field{
//		logger.String("trace_id", sc.TraceID().String()),
//		logger.String("span_id", sc.SpanID().String()),
//	}
//}
//
//// ── Main ──────────────────────────────────────────────────────────────────────
//
//func main() {
//	ctx := context.Background()
//
//	// 1. Init OTel — harus sebelum pakai otel.Tracer().
//	shutdown, err := setupOTel(ctx, "my-service")
//	if err != nil {
//		panic(err)
//	}
//	defer shutdown(ctx)
//
//	// 2. Init logger.
//	log := logger.GetLogger()
//	defer log.Shutdown()
//
//	tracer := otel.Tracer("my-service")
//
//	// ── Skenario A: Root span dari incoming HTTP request ──────────────────────
//	//
//	// Di Fiber/Gin middleware, ctx sudah berisi span dari otelfiber/otelgin.
//	// Di sini kita buat secara manual untuk demo.
//	ctx, rootSpan := tracer.Start(ctx, "handle /users/profile")
//	defer rootSpan.End()
//
//	// 3. Buat AppContext dari traced context — trace ID ikut terbawa.
//	appCtx := appContext.New(ctx, log)
//	appCtx.SetRequestID("req-abc123").SetUserID("user-42")
//
//	// 4. Log dengan trace context — trace_id/span_id mudah dikorelasikan di Grafana.
//	appCtx.Log().Info("request started", traceFields(appCtx.ToContext())...)
//
//	// ── Skenario B: Outgoing HTTP call dengan OTel middleware ─────────────────
//
//	client := httpclient.New(
//		httpclient.WithBaseURL("https://httpbin.org"),
//		httpclient.WithDefaultTimeout(10*time.Second),
//		// OTel middleware paling luar agar span mencakup retry + circuit breaker.
//		httpclient.WithMiddleware(OTelMiddleware(tracer)),
//		httpclient.WithMiddleware(httpclient.RetryMiddleware(httpclient.RetryConfig{
//			MaxAttempts: 2,
//			Backoff:     httpclient.ExponentialBackoff(100*time.Millisecond, 2.0),
//			RetryOn:     httpclient.RetryOnAny,
//		})),
//	)
//
//	type Result struct {
//		Headers map[string]string `json:"headers"`
//		URL     string            `json:"url"`
//	}
//	var result Result
//
//	err = client.Get("/get").
//		WithContext(appCtx.ToContext()). // context berisi root span → OTel middleware buat child span
//		WithRequestID("req-abc123").
//		Execute().
//		Consume(&result)
//	if err != nil {
//		appCtx.Log().Error("downstream call failed",
//			append(traceFields(appCtx.ToContext()), logger.Error(err))...)
//		return
//	}
//
//	// Header traceparent dikirim ke downstream — httpbin.org memantulkannya kembali.
//	appCtx.Log().Info("downstream call ok",
//		append(traceFields(appCtx.ToContext()),
//			logger.String("url", result.URL),
//			logger.String("traceparent_sent", result.Headers["Traceparent"]),
//		)...,
//	)
//
//	// ── Skenario C: Child span manual untuk operasi internal ──────────────────
//
//	func() {
//		childCtx, span := tracer.Start(appCtx.ToContext(), "db.query users")
//		defer span.End()
//
//		span.SetAttributes(
//			attribute.String("db.system", "postgresql"),
//			attribute.String("db.statement", "SELECT * FROM users WHERE id = $1"),
//		)
//
//		time.Sleep(5 * time.Millisecond) // simulasi query
//
//		log.WithContext(childCtx).Info("db query completed",
//			append(traceFields(childCtx), logger.String("table", "users"))...,
//		)
//	}()
//
//	appCtx.Log().Info("request completed",
//		append(traceFields(appCtx.ToContext()),
//			logger.Int64("duration_ms", appCtx.Duration().Milliseconds()),
//		)...,
//	)
//}
