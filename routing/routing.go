package routing

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
)

var captureGroupsPattern = regexp.MustCompile("P<([^>]+)>")

type route struct {
	pattern *regexp.Regexp
	handler http.Handler
	config  Config
}

// Handler defines our own custom HTTP handler that supports pattern matching a
// path using regular expressions
type Handler struct {
	routes []*route
}

// HandleFunc appends a new route and coerces the given function into a HandlerFunc
// MustCompile allows this service to fail fast when provided invalid regex config
func (h *Handler) HandleFunc(config Config, handler func(http.ResponseWriter, *http.Request)) {
	h.routes = append(h.routes, &route{
		pattern: regexp.MustCompile(config.Path),
		handler: http.HandlerFunc(handler),
		config:  config,
	})
}

// ServeHTTP attempts to match the incoming request path against the configured
// route patterns we've defined, and if a match is found we take any named
// capture groups and add them onto the request's query so we can later
// interpolate the captured values back into the request path (if configured to
// be used there -- see README for examples). Before the response is sent back
// to the client we clean-up the added query parameters.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range h.routes {
		pathMatchAndCaptureGroup := route.pattern.FindStringSubmatch(r.URL.Path)

		if len(pathMatchAndCaptureGroup) > 0 {
			captureGroups := captureGroupsPattern.FindAllStringSubmatch(route.config.Path, -1)

			if len(captureGroups) > 0 {
				// captureGroups = [[P<foo> foo] [P<bar> bar]]
				newQuery := r.URL.Query()
				originalQuery, _ := url.ParseQuery(r.URL.RawQuery)

				for k, v := range originalQuery {
					newQuery.Set(k, v[0])
				}

				for i, v := range pathMatchAndCaptureGroup[1:] {
					// prefix named capture groups when adding them as quer params
					// because this helps us identify them when cleaning query string
					// before making the actual proxy request
					queryKey := fmt.Sprintf("ncg_%s", captureGroups[i][1])
					newQuery.Set(queryKey, v)
				}

				r.URL.RawQuery = newQuery.Encode()
			}

			route.handler.ServeHTTP(w, r)
			return
		}
	}

	http.NotFound(w, r)
}
