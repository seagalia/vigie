package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/grpchealth/v2"
	"connectrpc.com/grpcreflect/v2"
	"github.com/rs/zerolog"
	"github.com/seagalia/vigie/config"
	"github.com/seagalia/vigie/logging"
	"github.com/seagalia/vigie/sdk/vigie/eds/v1/edsconnect"
	"github.com/seagalia/vigie/services"
	"github.com/seagalia/vigie/ui"
)

type App struct {
	entry   zerolog.Logger
	cnfPath string
	cnf     *config.Config
	version string
}

func NewApp(cnf *config.Config, cnfPath, version string) (*App, error) {
	err := logging.Setup(logging.Options{
		Level:   cnf.Log.Level,
		NoColor: cnf.Log.NoColor,
		InJson:  cnf.Log.InJson,
	})
	if err != nil {
		return nil, fmt.Errorf("setup logging: %w", err)
	}
	return &App{
		entry:   logging.Component("app"),
		cnf:     cnf,
		cnfPath: cnfPath,
		version: version,
	}, nil
}

func (a *App) Config() *config.Config {
	return a.cnf
}

// Handler routes the endpoints answer, over GET /services and the EDS service, the page and the reload; reload
// re-reads the config file into them.
func (a *App) Handler() (http.Handler, func() error, error) {
	handlerSvcs, err := services.NewHandler(a.cnf.Services)
	if err != nil {
		return nil, nil, fmt.Errorf("load services: %w", err)
	}
	page := ui.NewPage(a.cnf.Services, a.version)

	reload := func() error {
		a.entry.Info().Msg("Reload requested")
		cnf, err := config.ReadConfig(a.cnfPath)
		if err != nil {
			a.entry.Error().Err(err).Msg("read config")
			return err
		}
		err = handlerSvcs.LoadServices(cnf.Services)
		if err != nil {
			a.entry.Error().Err(err).Msg("load services")
			return err
		}
		// after the services only, the page must not list services that aren't served
		page.LoadServices(cnf.Services)
		a.entry.Info().Msg("Services loaded")
		return nil
	}

	mux := http.NewServeMux()
	mux.Handle("GET /services", handlerSvcs)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	page.Mount(mux)
	rpcServer := connect.NewServer(logCalls(logging.Component("eds")))
	edsconnect.RegisterEndpointDiscoveryServiceHandler(rpcServer, handlerSvcs)
	grpchealth.Register(rpcServer, grpchealth.NewStaticChecker())
	grpcreflect.Register(rpcServer)
	connecthttp.Mount(mux, rpcServer)
	if !a.cnf.DisableReloadByRequest {
		mux.HandleFunc("POST /reload", func(w http.ResponseWriter, r *http.Request) {
			err := reload()
			if err != nil {
				// the previous services keep being served, the caller must know its new file was refused
				http.Error(w, "services not reloaded: "+err.Error(), http.StatusUnprocessableEntity)
				return
			}
			_, _ = w.Write([]byte("services reloaded"))
		})
	}
	if len(a.cnf.ServerConfig.BasicAuth) > 0 {
		return requireBasicAuth(a.cnf.ServerConfig.BasicAuth, mux), reload, nil
	}
	return mux, reload, nil
}

func (a *App) Run() error {
	handler, reload, err := a.Handler()
	if err != nil {
		return err
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	go func() {
		for sig := range c {
			a.entry.Info().Stringer("signal", sig).Msg("Received signal")
			_ = reload()
		}
	}()

	withTLS := a.cnf.ServerConfig.Tls != nil
	server := &http.Server{
		Addr:      a.cnf.ServerConfig.ListenAddr,
		Handler:   handler,
		Protocols: serverProtocols(withTLS),
	}
	if withTLS {
		a.entry.Info().Str("listen_addr", server.Addr).Msg("Listening with TLS")
		return server.ListenAndServeTLS(a.cnf.ServerConfig.Tls.CertFile, a.cnf.ServerConfig.Tls.KeyFile)
	}

	a.entry.Info().Str("listen_addr", server.Addr).Msg("Listening")
	return server.ListenAndServe()
}

func logCalls(entry zerolog.Logger) connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			start := time.Now()
			err := next(ctx, spec, stream)
			duration := time.Since(start)
			if err != nil {
				entry.Warn().Err(err).
					Str("procedure", spec.Procedure).
					Stringer("code", connect.CodeOf(err)).
					Dur("duration", duration).
					Msg("Call failed")
				return err
			}
			entry.Debug().Str("procedure", spec.Procedure).Dur("duration", duration).Msg("Call served")
			return nil
		}
	}
}

// serverProtocols adds unencrypted HTTP/2 (h2c) to HTTP/1 and HTTP/2 without TLS only, so a gRPC client reaches
// Vigie without server.tls while a server with TLS keeps every request encrypted
func serverProtocols(withTLS bool) *http.Protocols {
	protocols := &http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(!withTLS)
	return protocols
}
