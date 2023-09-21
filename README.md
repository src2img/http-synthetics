# http-synthetics

An HTTP server with test methods for synthetic tests:

* `GET /` is a hello-world
* `GET /livecheck` returns 204 initially. `PUT /livecheck?code=newCode` changes this
* `GET /sleep?delay=5` will return a 204 after five seconds
* `/ws` endpoint serves a websocket with an echo service that return everything that is send to it
