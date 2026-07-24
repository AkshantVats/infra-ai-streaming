// SPDX-License-Identifier: MIT
// Example: two-tool agent that traces a weather lookup and a calculator call
// across a W3C traceparent boundary.
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/akshantvats/traceforge-go/traceforge"
)

func main() {
	ctx := context.Background()

	// Root span — represents the full agent turn.
	rootCtx, rootSpan := traceforge.StartSpan(ctx, "agent_turn")
	fmt.Printf("trace_id: %s\n", rootSpan.TraceID)

	// Simulate calling weather tool over HTTP.
	weatherCtx, weatherSpan := traceforge.StartSpan(rootCtx, "weather_api")
	callWeather(weatherCtx)
	traceforge.EndSpan(weatherCtx, weatherSpan, traceforge.StatusOK, nil)
	fmt.Printf("weather span_id: %s latency: %.2fms\n", weatherSpan.SpanID, weatherSpan.LatencyMs)

	// Simulate calling calculator tool over HTTP.
	calcCtx, calcSpan := traceforge.StartSpan(rootCtx, "calculator")
	callCalculator(calcCtx)
	traceforge.EndSpan(calcCtx, calcSpan, traceforge.StatusOK, nil)
	fmt.Printf("calc span_id: %s latency: %.2fms\n", calcSpan.SpanID, calcSpan.LatencyMs)

	// Close root span.
	traceforge.EndSpan(rootCtx, rootSpan, traceforge.StatusOK, nil)
	fmt.Printf("root span_id: %s total: %.2fms\n", rootSpan.SpanID, rootSpan.LatencyMs)
}

// callWeather simulates an outbound HTTP call with traceparent injection.
func callWeather(ctx context.Context) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8081/weather", nil)
	traceforge.InjectTraceContext(ctx, req.Header)
	// In a real agent the response would be read here.
}

// callCalculator simulates an outbound HTTP call with traceparent injection.
func callCalculator(ctx context.Context) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8082/calc", nil)
	traceforge.InjectTraceContext(ctx, req.Header)
}
