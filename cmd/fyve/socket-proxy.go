package fyve

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"tailscale.com/tsnet"
	"time"
)

func init() {
	rootCmd.AddCommand(SocketProxyCmd())
}

var (
	socketProxy *httputil.ReverseProxy
)

func SocketProxyCmd() *cobra.Command {
	var (
		hostname    string
		stateDir    string
		loginServer string
		socketPath  string
	)

	cmd := &cobra.Command{
		Use:   "socket-proxy",
		Short: "Docker socket proxy server",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if hostname == "" {
				if val := os.Getenv("TS_HOSTNAME"); val != "" {
					hostname = val
				} else {
					hostname = "socket-proxy"
				}
			}

			if stateDir == "" {
				if val := os.Getenv("TS_STATE_DIR"); val != "" {
					stateDir = val
				} else {
					stateDir = "socket-proxy.state"
				}
			}

			if socketPath == "" {
				if val := os.Getenv("DOCKER_SOCKET"); val != "" {
					socketPath = val
				} else {
					socketPath = "/var/run/docker.sock"
				}
			}

			s := &tsnet.Server{
				Dir:        filepath.Join(stateDir, "tsnet"),
				Hostname:   hostname,
				ControlURL: loginServer,
			}

			// Wait until tailscale is fully up, so that CertDomains has data.
			if _, err := s.Up(context.Background()); err != nil {
				return fmt.Errorf("tailscale did not come up: %w", err)
			}

			socketURLDummy, _ := url.Parse("http://localhost") // dummy URL - we use the unix socket
			socketProxy = httputil.NewSingleHostReverseProxy(socketURLDummy)
			socketProxy.Transport = &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			}

			l, err := s.Listen("tcp", ":2375")
			if err != nil {
				slog.Error("error listening on address", "error", err)
				os.Exit(2)
			}
			srv := &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					slog.Debug(r.URL.Path)
					socketProxy.ServeHTTP(w, r)
				}),
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			go func() {
				<-ctx.Done()
				slog.Info("Signal received, stopping...")
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(ctx)
				slog.Info("Stopped")
			}()

			slog.Info(fmt.Sprintf("Starting socket-proxy: %s:2375...", s.Hostname))
			if err := srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("proxy server problem", "error", err)
				os.Exit(2)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&hostname, "hostname", "", "", "Tailscale hostname to use")
	cmd.Flags().StringVarP(&stateDir, "state-dir", "s", "", "Tailscale coordination server URL")
	cmd.Flags().StringVarP(&loginServer, "login-server", "", "", "Server state directory")
	cmd.Flags().StringVarP(&socketPath, "socket-path", "", "", "Docker socket path")

	return cmd
}
