# http-synthetics

An HTTP server with test methods for synthetic tests:

* `GET /` is a hello-world.
* `GET /call-after-server-shutdown?url=https://someurl` returns a 204 and will cause the provided URL to be called after stopping the server. Optionally, a `delay` can be provided to override a wait time in seconds (default is 5s).
* `GET /before-server-shutdown?url=https://someurl` returns a 204 and will cause the provided URL to be called when a SIGTERM signal is captured and before stopping the server. Optionally, a `delay` can be provided to override a wait time in seconds (default is 5s).
* `GET /close` will cause the server to be shut down. Query parameters:
  * `delay` (default `0`) can be set to a positive number. The code will then wait the amount in seconds before the server is shut down.
  * `force` (default `false`) can be set to `true`. The code then calls the forceful `Close()` instead of the graceful `Shutdown()` function on the Go HTTP server.
  * `silent` (default `false`) can be set to `true`. The shutdown will then not be logged.
  * `terminate` (default `true`) can be set to `false`. The code will then after shutting down the server not end the process but wait for another SIGINT or SIGTERM signal.
* `GET /flaky?code=503` will return a 503 on every other call, with 200 on the respective next call. The `code` can be omitted, it will default to `502`.
* `GET /livecheck` returns 204 initially. `PUT /livecheck?code=newCode` changes this.
`delay` argument can be provided which overrides the default five seconds that it waits before it performs the call.
* `GET /env?env=someEnvKey` returns the value of an environment variable in the response body.
* `GET /request-header?header=someHeaderKey` returns the value of a request header in the response body. If the header is present multiple times, then the response contains all values concatenated by comma.
* `GET /sleep?delay=5` will return a 204 after five seconds.
* `GET /write-regularly?interval=1&count=10` will respond with 200 and write a 4 kB message into the response body every n seconds defined by the interval, n times defined by the count.
* `/ws` endpoint serves a websocket with an echo service that return everything that is send to it.
