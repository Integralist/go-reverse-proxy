package proxy

import (
	"net"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
	"time"

	"github.com/integralist/go-reverse-proxy/routing"
)

var responseHeaders = []string{
	"X-Forwarded-Host",
	"X-Origin-Host",
	"X-Router-Upstream",
	"X-Router-Upstream-OriginalHost",
	"X-Router-Upstream-OriginalPath",
	"X-Router-Upstream-OriginalPathModified",
	"X-Router-Upstream-Override",
	"X-Router-Upstream-OverrideHost",
	"X-Router-Upstream-OverridePath",
}

// GenerateProxy returns a unique reverse proxy instance which includes logic
// for handling override behaviour for configuration routes.
func GenerateProxy(conf routing.Config) http.Handler {
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.Header.Add("X-Router-Upstream", conf.Upstream.Name)
			req.Header.Add("X-Router-Upstream-OriginalHost", conf.Upstream.Host)
			req.Header.Add("X-Router-Upstream-OriginalPath", req.URL.Path)
			req.Header.Add("X-Forwarded-Host", req.Host)
			req.Header.Add("X-Origin-Host", conf.Upstream.Host)

			// This was done to unblock the ability to test the code, but it would be
			// better if there were no references to testing within the code itself,
			// so I think the only way to solve this would be to inject the
			// modifications as a func dependency so we could modify the request
			// object differently depending on the runtime environment context.
			if req.Header.Get("X-Testing") != "true" {
				req.URL.Scheme = "https"
			}

			// some upstreams will reject a request if the given Host HTTP header
			// isn't recognised, and so we always make sure to tweak that header
			// so it isn't set to the proxy's host value, but set to the upsteam's.
			req.Host = conf.Upstream.Host
			req.URL.Host = conf.Upstream.Host

			if conf.ModifyPath != "" {
				if strings.Contains(conf.ModifyPath, "$") {
					req.URL.Path = conf.ModifyPath

					for k, v := range req.URL.Query() {
						// req.URL.Query() = map[foo:[ncg_foovalue] bar:[ncg_barvalue]]
						isCaptureGroup := strings.HasPrefix(k, "ncg_")

						if isCaptureGroup {
							// interpolate query param value into modified request path
							cleanKeyPrefix := strings.Replace(k, "ncg_", "", 1)
							r := strings.NewReplacer("$", "", cleanKeyPrefix, v[0])
							req.URL.Path = r.Replace(req.URL.Path)
						}
					}
				} else {
					req.URL.Path = conf.ModifyPath
				}
				req.Header.Add("X-Router-Upstream-OriginalPathModified", req.URL.Path)
			}

			if conf.Override.Header != "" && conf.Override.Match != "" {
				if req.Header.Get(conf.Override.Header) == conf.Override.Match {
					if conf.Override.ModifyPath != "" {
						// TODO: duplicated logic, needs moving into a separate function
						if strings.Contains(conf.Override.ModifyPath, "$") {
							req.URL.Path = conf.Override.ModifyPath

							for k, v := range req.URL.Query() {
								// req.URL.Query() = map[foo:[ncg_foovalue] bar:[ncg_barvalue]]
								isCaptureGroup := strings.HasPrefix(k, "ncg_")

								if isCaptureGroup {
									// interpolate query param value into modified request path
									cleanKeyPrefix := strings.Replace(k, "ncg_", "", 1)
									r := strings.NewReplacer("$", "", cleanKeyPrefix, v[0])
									req.URL.Path = r.Replace(req.URL.Path)
								}
							}
						} else {
							req.URL.Path = conf.Override.ModifyPath
						}

						req.Header.Add("X-Router-Upstream-OverridePath", req.URL.Path)
					}

					if conf.Override.Upstream != nil && conf.Override.Upstream.Host != "" && conf.Override.Upstream.Name != "" {
						req.Host = conf.Override.Upstream.Host
						req.URL.Host = conf.Override.Upstream.Host
						req.Header.Add("X-Router-Upstream-Override", conf.Override.Upstream.Name)
						req.Header.Add("X-Router-Upstream-OverrideHost", conf.Override.Upstream.Host)
					}
				}
			}

			if conf.Override.Query != "" && conf.Override.Match != "" {
				pattern := regexp.MustCompile(conf.Override.Match)
				param := req.URL.Query().Get(conf.Override.Query)

				if conf.Override.MatchType == "regex" {
					match := pattern.MatchString(param)

					if match {
						if conf.Override.Upstream != nil {
							req.Host = conf.Override.Upstream.Host
							req.URL.Host = conf.Override.Upstream.Host
							req.Header.Add("X-Router-Upstream-OverrideHost", req.URL.Host)
						}
						if conf.Override.ModifyPath != "" {
							newpath := []byte{}
							queryparam := []byte(param)
							template := []byte(conf.Override.ModifyPath)

							for _, submatches := range pattern.FindAllSubmatchIndex(queryparam, -1) {
								newpath = pattern.Expand(newpath, template, queryparam, submatches)
							}

							req.URL.Path = string(newpath)
							req.Header.Add("X-Router-Upstream-OverridePath", req.URL.Path)
						}
					}
				} else {
					if param == conf.Override.Match {
						if conf.Override.Upstream != nil {
							req.Host = conf.Override.Upstream.Host
							req.URL.Host = conf.Override.Upstream.Host
							req.Header.Add("X-Router-Upstream-OverrideHost", req.URL.Host)
						}
						if conf.Override.ModifyPath != "" {
							req.URL.Path = conf.Override.ModifyPath
							req.Header.Add("X-Router-Upstream-OverridePath", req.URL.Path)
						}
					}
				}
			}

			// remove named capture groups from query
			for k := range req.URL.Query() {
				isCaptureGroup := strings.HasPrefix(k, "ncg_")

				if isCaptureGroup {
					originalQuery := req.URL.Query()
					originalQuery.Del(k)
					req.URL.RawQuery = originalQuery.Encode()
				}
			}
		},
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
		},
		ModifyResponse: func(r *http.Response) error {
			for _, header := range responseHeaders {
				value := r.Request.Header.Get(header)

				if value != "" {
					r.Header.Set(header, value)
				}
			}
			return nil
		},
	}

	return proxy
}