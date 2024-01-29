# http-synthetics

An HTTP server with test methods for synthetic tests:

* `GET /` is a hello-world.
* `GET /call-after-server-shutdown?url=https://someurl` returns a 204 and will cause the provided URL to be called after stopping the server. Optionally, a `delay` can be provided to override a wait time in seconds (default is 5s).
* `GET /before-server-shutdown?url=https://someurl` returns a 204 and will cause the provided URL to be called when a SIGTERM signal is captured and before stopping the server. Optionally, a `delay` can be provided to override a wait time in seconds (default is 5s).
* `GET /livecheck` returns 204 initially. `PUT /livecheck?code=newCode` changes this.
`delay` argument can be provided which overrides the default five seconds that it waits before it performs the call.
* `GET /sleep?delay=5` will return a 204 after five seconds.
* `GET /flaky?code=503` will return a 503 on every other call, with 200 on the respective next call. The `code` can be omited, it will default to `502`.
* `/ws` endpoint serves a websocket with an echo service that return everything that is send to it.
