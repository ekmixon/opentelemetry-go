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

package otlptracegrpc // import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/internal/connection"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/internal/otlpconfig"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type client struct {
	connection *connection.Connection

	lock         sync.Mutex
	tracesClient coltracepb.TraceServiceClient
}

var _ otlptrace.Client = (*client)(nil)

var (
	errNoClient = errors.New("no client")
)

// NewClient creates a new gRPC trace client.
func NewClient(opts ...Option) otlptrace.Client {
	cfg := otlpconfig.NewDefaultConfig()
	otlpconfig.ApplyGRPCEnvConfigs(&cfg)
	for _, opt := range opts {
		opt.applyGRPCOption(&cfg)
	}

	c := &client{}
	c.connection = connection.NewConnection(cfg, cfg.Traces, c.handleNewConnection)

	return c
}

func (c *client) handleNewConnection(cc *grpc.ClientConn) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if cc != nil {
		c.tracesClient = coltracepb.NewTraceServiceClient(cc)
	} else {
		c.tracesClient = nil
	}
}

// Start establishes a connection to the collector.
func (c *client) Start(ctx context.Context) error {
	return c.connection.StartConnection(ctx)
}

// Stop shuts down the connection to the collector.
func (c *client) Stop(ctx context.Context) error {
	return c.connection.Shutdown(ctx)
}

// UploadTraces sends a batch of spans to the collector.
func (c *client) UploadTraces(ctx context.Context, protoSpans []*tracepb.ResourceSpans) error {
	if !c.connection.Connected() {
		return fmt.Errorf("traces exporter is disconnected from the server %s: %w", c.connection.SCfg.Endpoint, c.connection.LastConnectError())
	}

	ctx, cancel := c.connection.ContextWithStop(ctx)
	defer cancel()
	ctx, tCancel := context.WithTimeout(ctx, c.connection.SCfg.Timeout)
	defer tCancel()

	ctx = c.connection.ContextWithMetadata(ctx)
	err := func() error {
		c.lock.Lock()
		defer c.lock.Unlock()
		if c.tracesClient == nil {
			return errNoClient
		}
		return c.connection.DoRequest(ctx, func(ctx context.Context) error {
			_, err := c.tracesClient.Export(ctx, &coltracepb.ExportTraceServiceRequest{
				ResourceSpans: protoSpans,
			})
			return err
		})
	}()
	if err != nil {
		c.connection.SetStateDisconnected(err)
	}
	return err
}
