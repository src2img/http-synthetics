// Inspired by https://github.com/shipwright-io/sample-go/blob/main/source-build/main.go

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/websocket"
)

var toggle bool

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

	var afterShutDownUrl *string
	afterShutDownUrlDelay := 5 * time.Second

	livecheckCode := 204

	var sigtermUrl *string
	sigtermUrlDelay := 5 * time.Second

	var doNotTerminate = false
	var silentShutdown = false
	var forceClose = false

	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			log.Println("Answering a Hello World request")
			fmt.Fprintf(w, "Hello, World! I am using %s by the way.", runtime.Version())
		})

		http.HandleFunc("/call-after-server-shutdown", func(w http.ResponseWriter, request *http.Request) {
			queryParameters := request.URL.Query()
			if !queryParameters.Has("url") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			url := queryParameters.Get("url")
			afterShutDownUrl = &url

			if queryParameters.Has("delay") {
				delay, err := strconv.Atoi(queryParameters.Get("delay"))
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				afterShutDownUrlDelay = time.Duration(delay) * time.Second
			}

			w.WriteHeader(http.StatusNoContent)
		})

		http.HandleFunc("/call-before-server-shutdown", func(w http.ResponseWriter, request *http.Request) {
			queryParameters := request.URL.Query()
			if !queryParameters.Has("url") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			url := queryParameters.Get("url")
			sigtermUrl = &url

			if queryParameters.Has("delay") {
				delay, err := strconv.Atoi(queryParameters.Get("delay"))
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				sigtermUrlDelay = time.Duration(delay) * time.Second
			}

			w.WriteHeader(http.StatusNoContent)
		})

		http.HandleFunc("/claim-memory", func(w http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodPut {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			queryParameters := request.URL.Query()

			if !queryParameters.Has("amount") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			amount, err := strconv.Atoi(queryParameters.Get("amount"))
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			data := make([]byte, amount)
			for i := 0; i < amount; i++ {
				data[i] = byte(i % 256)
			}

			w.WriteHeader(http.StatusNoContent)
		})

		http.HandleFunc("/close", func(w http.ResponseWriter, request *http.Request) {
			queryParameters := request.URL.Query()

			if queryParameters.Get("force") == "true" {
				forceClose = true
			}

			if queryParameters.Get("silent") == "true" {
				silentShutdown = true
			}

			if queryParameters.Get("terminate") == "false" {
				doNotTerminate = true
			}

			delay := 0

			if queryParameters.Has("delay") {
				var err error
				delay, err = strconv.Atoi(queryParameters.Get("delay"))
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			}

			w.WriteHeader(http.StatusAccepted)

			go func() {
				time.Sleep(time.Duration(delay) * time.Second)
				signals <- os.Interrupt
			}()
		})

		http.HandleFunc("/compute-resource-token", func(w http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet && request.Method != http.MethodPut {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			tokenFileInfo, err := os.Stat("/var/run/secrets/codeengine.cloud.ibm.com/compute-resource-token/token")
			if err != nil {
				if os.IsNotExist(err) {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "%v", err)
				return
			}

			if tokenFileInfo.IsDir() {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			tokenFileData, err := os.ReadFile("/var/run/secrets/codeengine.cloud.ibm.com/compute-resource-token/token")
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "%v", err)
				return
			}

			switch request.Method {
			case http.MethodGet:
				tokenParts := strings.Split(string(tokenFileData), ".")
				if len(tokenParts) != 3 {
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, "expected three parts in token file separated by dot, but found %d", len(tokenParts))
					return
				}

				header, err := base64.RawURLEncoding.DecodeString(tokenParts[0])
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, "%v", err)
					return
				}

				body, err := base64.RawURLEncoding.DecodeString(tokenParts[1])
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, "%v", err)
					return
				}

				signature := tokenParts[2]

				w.Header().Add("Content-Type", "application/json")
				_, err = w.Write([]byte("{\"header\":"))
				if err != nil {
					log.Printf("Error while writing message: %v", err)
					return
				}
				_, err = w.Write(header)
				if err != nil {
					log.Printf("Error while writing message: %v", err)
					return
				}
				_, err = w.Write([]byte(",\"body\":"))
				if err != nil {
					log.Printf("Error while writing message: %v", err)
					return
				}
				_, err = w.Write(body)
				if err != nil {
					log.Printf("Error while writing message: %v", err)
					return
				}
				_, err = w.Write([]byte(",\"signature\":\""))
				if err != nil {
					log.Printf("Error while writing message: %v", err)
					return
				}
				_, err = w.Write([]byte(signature))
				if err != nil {
					log.Printf("Error while writing message: %v", err)
					return
				}
				_, err = w.Write([]byte("\"}"))
				if err != nil {
					log.Printf("Error while writing message: %v", err)
					return
				}
				return

			case http.MethodPut:
				queryParameters := request.URL.Query()

				if queryParameters.Get("action") != "login" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				iamEndpoint := queryParameters.Get("iam")
				if iamEndpoint == "" {
					iamEndpoint = "https://iam.cloud.ibm.com"
				}

				trustedProfileName := queryParameters.Get("profile-name")
				if trustedProfileName == "" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				requestBody := url.Values{}
				requestBody.Set("grant_type", "urn:ibm:params:oauth:grant-type:cr-token")
				requestBody.Set("cr_token", string(tokenFileData))
				requestBody.Set("profile_name", trustedProfileName)

				iamResponse, err := http.Post(fmt.Sprintf("%s/identity/token", iamEndpoint), "application/x-www-form-urlencoded", strings.NewReader(requestBody.Encode()))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, "%v", err)
					return
				}

				defer iamResponse.Body.Close()

				if iamResponse.StatusCode == http.StatusOK {
					log.Printf("Successfully created access token from compute resource token for trusted profile %s", trustedProfileName)
					w.WriteHeader(http.StatusNoContent)
					return
				}

				iamResponseBody, _ := io.ReadAll(iamResponse.Body)
				log.Printf("Failed to create access token from compute resource token for trusted profile %s. Status: %d. Body: %s", trustedProfileName, iamResponse.StatusCode, string(iamResponseBody))
				w.WriteHeader(http.StatusForbidden)
				return
			}
		})

		http.HandleFunc("/env", func(w http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			queryParameters := request.URL.Query()
			if !queryParameters.Has("env") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)

			_, err := w.Write([]byte(os.Getenv(queryParameters.Get("env"))))
			if err != nil {
				log.Printf("Error while writing message: %v", err)
				return
			}
		})

		http.HandleFunc("/livecheck", func(w http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodPut {
				w.WriteHeader(livecheckCode)
				return
			}

			queryParameters := request.URL.Query()
			if !queryParameters.Has("code") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			code, err := strconv.Atoi(queryParameters.Get("code"))
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			livecheckCode = code
			w.WriteHeader(http.StatusNoContent)
		})

		http.HandleFunc("/request-header", func(w http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			queryParameters := request.URL.Query()
			if !queryParameters.Has("header") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)

			headerValues, exists := request.Header[queryParameters.Get("header")]
			if !exists {
				return
			}

			_, err := w.Write([]byte(strings.Join(headerValues, ",")))
			if err != nil {
				log.Printf("Error while writing message: %v", err)
				return
			}
		})

		http.HandleFunc("/sleep", func(w http.ResponseWriter, request *http.Request) {
			queryParameters := request.URL.Query()
			if queryParameters.Has("delay") {
				seconds, err := strconv.Atoi(queryParameters.Get("delay"))
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				time.Sleep(time.Second * time.Duration(seconds))
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		})

		http.HandleFunc("/flaky", func(w http.ResponseWriter, request *http.Request) {
			toggle = !toggle

			var httpStatusCode int = http.StatusBadGateway
			if queryParameters := request.URL.Query(); queryParameters.Has("code") {
				var err error
				httpStatusCode, err = strconv.Atoi(queryParameters.Get("code"))
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			}

			if toggle {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(httpStatusCode)
			}
		})

		http.HandleFunc("/write-regularly", func(w http.ResponseWriter, request *http.Request) {
			queryParameters := request.URL.Query()

			var intervalSeconds, count int
			var err error

			if queryParameters.Has("interval") {
				intervalSeconds, err = strconv.Atoi(queryParameters.Get("interval"))
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			} else {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if queryParameters.Has("count") {
				count, err = strconv.Atoi(queryParameters.Get("count"))
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			} else {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			message := make([]byte, 4<<10)
			_, err = rand.Read(message)
			if err != nil {
				log.Printf("Error while creating random message: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)

			for i := 0; i < count; i++ {
				time.Sleep(time.Duration(intervalSeconds) * time.Second)
				_, err = w.Write(message)
				if err != nil {
					log.Printf("Error while writing message: %v", err)
					return
				}
			}
		})

		http.HandleFunc("/filesystem", func(w http.ResponseWriter, r *http.Request) {
			queryParameters := r.URL.Query()
			var path string
			if queryParameters.Has("path") {
				path = queryParameters.Get("path")
			} else {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			fileInfo, err := os.Stat(path)
			if os.IsNotExist(err) {
				log.Printf("Mount not found : %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			} else if err != nil {
				log.Printf("Error accessing mount: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if fileInfo.IsDir() {
				w.WriteHeader(http.StatusNoContent)
			} else {
				if r.Method == http.MethodGet {
					data, err := os.ReadFile(path)
					if err != nil {
						log.Printf("Error reading file: %v", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					fmt.Fprintf(w, "File content: %s", string(data))
				}
				w.WriteHeader(http.StatusOK)
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

	if !silentShutdown {
		log.Print("Received shutdown request")
	}

	if sigtermUrl != nil {
		time.Sleep(sigtermUrlDelay)

		log.Printf("Calling before server close: %s", *sigtermUrl)
		resp, err := http.Get(*sigtermUrl)

		if err != nil {
			log.Printf("Failed to call %s before server close: %v", *sigtermUrl, err)
		}

		if resp != nil {
			if resp.StatusCode > 299 {
				log.Printf("Failed to call %s before server close with status code %d", *sigtermUrl, resp.StatusCode)
			} else {
				log.Printf("Successfully called %s before server close with status %d", *sigtermUrl, resp.StatusCode)
			}
		}
	}

	if !silentShutdown {
		log.Printf("shutting down server")
	}

	if forceClose {
		if err := srv.Close(); err != nil {
			log.Fatalf("failed to close server: %v", err)
		}
	} else {
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown server: %v", err)
		}
	}

	if afterShutDownUrl != nil {
		time.Sleep(afterShutDownUrlDelay)

		log.Printf("Calling after server close: %s", *afterShutDownUrl)
		resp, err := http.Get(*afterShutDownUrl)

		if err != nil {
			log.Printf("Failed to call %s after server close: %v", *afterShutDownUrl, err)
		}

		if resp != nil {
			if resp.StatusCode > 299 {
				log.Printf("Failed to call %s after server close with status code %d", *afterShutDownUrl, resp.StatusCode)
			} else {
				log.Printf("Successfully called %s after server close with status %d", *afterShutDownUrl, resp.StatusCode)
			}
		}
	}

	if doNotTerminate {
		<-signals
	}
}
