package serve

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
)

type App struct {
	server *http.Server
	mux    *http.ServeMux
}

func NewApp() *App {
	a := &App{
		mux: http.NewServeMux(),
	}
	a.mux.HandleFunc("/rpc/init", a.handleInit)
	a.mux.HandleFunc("/rpc/{repo}/{func}", a.handleRpc)
	return a
}

func (a *App) HandleFunc(pattern string, handler http.HandlerFunc) {
	a.mux.HandleFunc(pattern, handler)
}

func (a *App) Handle(pattern string, handler http.Handler) {
	a.mux.Handle(pattern, handler)
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *App) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Println("Listening on", ln.(*net.TCPListener).Addr())
	a.server = &http.Server{
		Addr:    addr,
		Handler: a.mux,
	}
	go func() {
		if err := a.server.Serve(ln); err != nil {
			if err == http.ErrServerClosed {
				fmt.Println("Server closed")
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}()
	return nil
}

func (a *App) Stop() error {
	if a.server != nil {
		if err := a.server.Shutdown(context.Background()); err != nil {
			return err
		}
		a.server = nil
	}
	return nil
}
