// Inspired by https://github.com/shipwright-io/sample-go/blob/main/source-build/main.go

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/net/websocket"
)

func main() {
	ctx := context.Background()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	port := 8080
	if strValue, ok := os.LookupEnv("PORT"); ok {
		if intValue, err := strconv.Atoi(strValue); err == nil {
			port = intValue
		}
	}

	srv := &http.Server{Addr: fmt.Sprintf(":%d", port)}

	livecheckCode := 204

	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, "Hello, World! I am using %s by the way.", runtime.Version())
		})

		http.HandleFunc("/livecheck", func(w http.ResponseWriter, request *http.Request) {
			if request.Method != "PUT" {
				w.WriteHeader(livecheckCode)
				return
			}

			queryParameters := request.URL.Query()
			if !queryParameters.Has("code") {
				w.WriteHeader(400)
				return
			}
			code, err := strconv.Atoi(queryParameters.Get("code"))
			if err != nil {
				w.WriteHeader(400)
				return
			}

			livecheckCode = code
			w.WriteHeader(204)
		})

		http.HandleFunc("/sleep", func(w http.ResponseWriter, request *http.Request) {
			queryParameters := request.URL.Query()
			if queryParameters.Has("delay") {
				seconds, err := strconv.Atoi(queryParameters.Get("delay"))
				if err != nil {
					w.WriteHeader(400)
					return
				}
				time.Sleep(time.Second * time.Duration(seconds))
				w.WriteHeader(204)
			} else {
				w.WriteHeader(400)
				return
			}
		})

		http.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
			log.Printf("starting websocket handler for an echo service")

			count, err := io.Copy(ws, ws)
			if err != nil {
				log.Printf("copy failed in websocket handler: %v", err)
			}

			log.Printf("stopping websocket handler after copying %d bytes", count)
		}))

		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	<-signals
	log.Printf("shutting down server")
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown server: %v", err)
	}
}
