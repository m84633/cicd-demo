package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"partivo_tickets/internal/worker"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// AllowedHeaders defines the list of headers that are allowed to be passed through the gRPC-Gateway.
var allowedHeadersSet map[string]struct{}

// HttpHandlerRegister defines a function that registers custom HTTP handlers.
type HttpHandlerRegister func(mux *http.ServeMux)

// App manages the gRPC server, HTTP gateway, and background workers.
type App struct {
	httpServer *http.Server
	gRPCServer *grpc.Server
	workers    []worker.Worker
	port       int
	logger     *zap.Logger
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewApp creates and configures a new application server.
func NewApp(port int, logger *zap.Logger, grpcServices []interface{}, register HttpHandlerRegister, unaryInterceptors []grpc.UnaryServerInterceptor, allowHeaders map[string]struct{}, workers []worker.Worker) (*App, func(), error) {
	allowedHeadersSet = allowHeaders
	// Create gRPC server with chained interceptors
	s := grpc.NewServer(grpc.ChainUnaryInterceptor(unaryInterceptors...))

	// Register gRPC services
	registerGRPCServices(s, grpcServices...)

	// Register Health Check server
	healthcheck := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthcheck)

	// Register reflection service on gRPC server.
	reflection.Register(s)

	// Create the gRPC-Gateway mux
	gwmux := newGatewayMux()

	// Register gRPC-Gateway handlers
	endpoint := fmt.Sprintf("localhost:%d", port)
	if err := registerGatewayHandlers(context.Background(), gwmux, endpoint); err != nil {
		return nil, nil, err
	}

	// Create a new HTTP serve mux and register the gateway handler
	mux := http.NewServeMux()
	mux.Handle("/", gwmux)
	if register != nil {
		register(mux)
	}
	//mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
	//	//cli, cleanup, err := events.NewClient("192.168.1.129:8081")
	//	//if err != nil {
	//	//	fmt.Println(err.Error())
	//	//	return
	//	//}
	//	//defer cleanup()
	//	//event, err := cli.GetEventByID(context.Background(), "68afd0fe369e6dfba49d243")
	//	//fmt.Println("event:")
	//	//fmt.Println(event)
	//	cli, cleanup, err := events.NewClient("192.168.1.129:8081")
	//	if err != nil {
	//		fmt.Println(err.Error())
	//		return
	//	}
	//	defer cleanup()
	//	session, err := cli.GetSessionByID(context.Background(), "6889967f0e353d0bf81de5c9")
	//	fmt.Println("session: ", session)
	//
	//})

	// Create the main HTTP server
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: grpcHandlerFunc(s, mux),
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		httpServer: httpServer,
		gRPCServer: s,
		workers:    workers,
		port:       port,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}

	// The cleanup function will be called by main to gracefully shut down.
	cleanup := func() {
		app.logger.Info("Cleanup: stopping server and worker...")
		app.cancel() // Signal all background goroutines to stop

		// Create a context with a timeout for the final shutdown.
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		// Gracefully stop gRPC server
		app.gRPCServer.GracefulStop()

		// Shut down HTTP server
		if err := app.httpServer.Shutdown(shutdownCtx); err != nil {
			app.logger.Error("HTTP server shutdown failed", zap.Error(err))
		}
		app.logger.Info("Cleanup finished.")
	}

	return app, cleanup, nil
}

// Run starts the application server and all background workers.
func (a *App) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", a.port, err)
	}

	// Start Outbox Worker in a goroutine
	for _, w := range a.workers {
		go w.Start(a.ctx)
	}

	// Start HTTP/gRPC server in a goroutine
	go func() {
		a.logger.Info("server started", zap.Int("port", a.port))
		if err := a.httpServer.Serve(lis); err != nil && err != http.ErrServerClosed {
			a.logger.Error("HTTP server Serve error", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	a.logger.Info("Shutting down server...")
	a.cancel() // This will trigger the cleanup function via the context in Start methods

	return nil
}

// newGatewayMux creates and configures a new gRPC-Gateway mux
func newGatewayMux() *runtime.ServeMux {
	return runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(func(k string) (string, bool) {
			if _, ok := allowedHeadersSet[k]; ok {
				return k, true
			}
			return runtime.DefaultHeaderMatcher(k)
		}),
		runtime.WithErrorHandler(func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
			st, ok := status.FromError(err)
			if !ok {
				st = status.New(codes.Unknown, "Unknown error")
			}

			httpCode := runtime.HTTPStatusFromCode(st.Code())

			resp := map[string]interface{}{
				"status":  "error",
				"code":    httpCode,
				"message": st.Message(),
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(httpCode)
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		}),
	)
}

func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	return h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	}), &http2.Server{})
}
