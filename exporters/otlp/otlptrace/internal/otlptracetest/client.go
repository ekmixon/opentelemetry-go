// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlptracetest // import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/internal/otlptracetest"

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
)

func RunExporterShutdownTest(t *testing.T, factory func() otlptrace.Client) {
	t.Run("testClientStopHonorsTimeout", func(t *testing.T) {
		testClientStopHonorsTimeout(t, factory())
	})

	t.Run("testClientStopHonorsCancel", func(t *testing.T) {
		testClientStopHonorsCancel(t, factory())
	})

	t.Run("testClientStopNoError", func(t *testing.T) {
		testClientStopNoError(t, factory())
	})

	t.Run("testClientStopManyTimes", func(t *testing.T) {
		testClientStopManyTimes(t, factory())
	})
}

func initializeExporter(t *testing.T, client otlptrace.Client) *otlptrace.Exporter {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	e, err := otlptrace.New(ctx, client)
	if err != nil {
		t.Fatalf("failed to create exporter")
	}

	return e
}

func testClientStopHonorsTimeout(t *testing.T, client otlptrace.Client) {
	e := initializeExporter(t, client)

	innerCtx, innerCancel := context.WithTimeout(context.Background(), time.Microsecond)
	<-innerCtx.Done()
	if err := e.Shutdown(innerCtx); err == nil {
		t.Error("expected context DeadlineExceeded error, got nil")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context DeadlineExceeded error, got %v", err)
	}
	innerCancel()
}

func testClientStopHonorsCancel(t *testing.T, client otlptrace.Client) {
	e := initializeExporter(t, client)

	ctx, innerCancel := context.WithCancel(context.Background())
	innerCancel()
	if err := e.Shutdown(ctx); err == nil {
		t.Error("expected context canceled error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context canceled error, got %v", err)
	}
}

func testClientStopNoError(t *testing.T, client otlptrace.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	e := initializeExporter(t, client)
	if err := e.Shutdown(ctx); err != nil {
		t.Errorf("shutdown errored: expected nil, got %v", err)
	}
}

func testClientStopManyTimes(t *testing.T, client otlptrace.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	e := initializeExporter(t, client)

	ch := make(chan struct{})
	wg := sync.WaitGroup{}
	const num int = 20
	wg.Add(num)
	errs := make([]error, num)
	for i := 0; i < num; i++ {
		go func(idx int) {
			defer wg.Done()
			<-ch
			errs[idx] = e.Shutdown(ctx)
		}(i)
	}
	close(ch)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("failed to shutdown exporter: %v", err)
		}
	}
}
