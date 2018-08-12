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
				modifyPath(req, conf.ModifyPath)
			}

			if conf.Override.Header != "" && conf.Override.Match != "" {
				overrideHeader(req, conf.Override)
			}

			if conf.Override.Query != "" && conf.Override.Match != "" {
				overrideQuery(req, conf.Override)
			}

			cleanUpQueryString(req)
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

func modifyPath(req *http.Request, modifyPath string) {
	if strings.Contains(modifyPath, "$") {
		req.URL.Path = modifyPath

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
		req.URL.Path = modifyPath
	}

	req.Header.Add("X-Router-Upstream-OriginalPathModified", req.URL.Path)
}

func overrideHeader(req *http.Request, override routing.Override) {
	if req.Header.Get(override.Header) == override.Match {
		if override.ModifyPath != "" {
			// TODO: duplicated logic with modifyPath function
			if strings.Contains(override.ModifyPath, "$") {
				req.URL.Path = override.ModifyPath

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
				req.URL.Path = override.ModifyPath
			}

			req.Header.Add("X-Router-Upstream-OverridePath", req.URL.Path)
		}

		if override.Upstream != nil && override.Upstream.Host != "" && override.Upstream.Name != "" {
			req.Host = override.Upstream.Host
			req.URL.Host = override.Upstream.Host
			req.Header.Add("X-Router-Upstream-Override", override.Upstream.Name)
			req.Header.Add("X-Router-Upstream-OverrideHost", override.Upstream.Host)
		}
	}
}

func overrideQuery(req *http.Request, override routing.Override) {
	pattern := regexp.MustCompile(override.Match)
	param := req.URL.Query().Get(override.Query)

	if override.MatchType == "regex" {
		match := pattern.MatchString(param)

		if match {
			if override.Upstream != nil {
				req.Host = override.Upstream.Host
				req.URL.Host = override.Upstream.Host
				req.Header.Add("X-Router-Upstream-OverrideHost", req.URL.Host)
			}
			if override.ModifyPath != "" {
				newpath := []byte{}
				queryparam := []byte(param)
				template := []byte(override.ModifyPath)

				for _, submatches := range pattern.FindAllSubmatchIndex(queryparam, -1) {
					newpath = pattern.Expand(newpath, template, queryparam, submatches)
				}

				req.URL.Path = string(newpath)
				req.Header.Add("X-Router-Upstream-OverridePath", req.URL.Path)
			}
		}
	} else {
		if param == override.Match {
			if override.Upstream != nil {
				req.Host = override.Upstream.Host
				req.URL.Host = override.Upstream.Host
				req.Header.Add("X-Router-Upstream-OverrideHost", req.URL.Host)
			}
			if override.ModifyPath != "" {
				req.URL.Path = override.ModifyPath
				req.Header.Add("X-Router-Upstream-OverridePath", req.URL.Path)
			}
		}
	}

}

// cleanUpQueryString removes any named capture groups from the query string
// that were added by our routing logic. The capture groups were added to the
// query string so that the final request handler could easily parse the values
// and interpolate them into a modified request path, but after that they are
// not useful to either the client nor the proxied upstream.
func cleanUpQueryString(req *http.Request) {
	for k := range req.URL.Query() {
		isCaptureGroup := strings.HasPrefix(k, "ncg_")

		if isCaptureGroup {
			originalQuery := req.URL.Query()
			originalQuery.Del(k)
			req.URL.RawQuery = originalQuery.Encode()
		}
	}
}
