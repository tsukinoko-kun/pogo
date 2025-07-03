package main

import (
	"fmt"
	"github.com/tsukinoko-kun/pogo/db"
	"github.com/tsukinoko-kun/pogo/serve"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

var (
	host string
)

func init() {
	if hostEnv, ok := os.LookupEnv("HOST"); ok {
		host = hostEnv
	} else {
		if portEnv, ok := os.LookupEnv("PORT"); ok {
			host = ":" + portEnv
		}
	}
}

func main() {
	db.Connect()
	app := serve.NewApp()
	app.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("request to unregistered path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
	if err := app.Start(host); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
		return
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGABRT)
	<-sig
	log.Println("shutting down server")
	if err := app.Stop(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
		return
	}
	log.Println("server stopped")

	log.Println("disconnecting from database")
	db.Disconnect()
	log.Println("database disconnected")
}
