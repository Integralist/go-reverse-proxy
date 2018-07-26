// The tests in this file are verifying the expected behaviour from the
// integration perspective. We stub the upstream responses so they return the
// specific upstream and path that was finally requested. This way we can be
// sure the routing configuration we have is mutating the request as expected.

package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/integralist/go-reverse-proxy/routing"
	"github.com/integralist/go-reverse-proxy/upstreams"
)

type Response struct {
	Upstream string `json:"upstream"`
	URL      string `json:"url"`
}

var handler = &routing.Handler{}
var server = httptest.NewServer(handler)

var mockHTTPBin = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, fmt.Sprintf(`{"upstream": "httpbin", "url": "%s"}`, r.URL))
}))

var mockIntegralist = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, fmt.Sprintf(`{"upstream": "integralist", "url": "%s"}`, r.URL))
}))

func configureRouting() {
	for _, conf := range routing.Configuration {
		if conf.Upstream.Name == "httpbin" {
			u, err := url.Parse(mockHTTPBin.URL)
			if err != nil {
				log.Fatal(err)
			}

			host := fmt.Sprintf("%s:%s", u.Hostname(), u.Port())

			conf.Upstream = &upstreams.Upstream{
				Name: "httpbin",
				Host: host,
			}
		}

		if conf.Override.Upstream != nil && conf.Override.Upstream.Name == "integralist" {
			u, err := url.Parse(mockIntegralist.URL)
			if err != nil {
				log.Fatal(err)
			}

			host := fmt.Sprintf("%s:%s", u.Hostname(), u.Port())

			conf.Override.Upstream = &upstreams.Upstream{
				Name: "integralist",
				Host: host,
			}
		}

		proxy := GenerateProxy(conf)

		handler.HandleFunc(conf, func(w http.ResponseWriter, r *http.Request) {
			r.Header.Add("X-Testing", "true")
			r.URL.Scheme = "http"
			proxy.ServeHTTP(w, r)
		})
	}
}

func verifyResponse(res *Response, upstream string, path string, t *testing.T) {
	if res.Upstream != upstream {
		t.Errorf("The response:\n '%s'\ndidn't match the expectation:\n '%s'", res.Upstream, upstream)
	}

	if res.URL != path {
		t.Errorf("The response:\n '%s'\ndidn't match the expectation:\n '%s'", res.URL, path)
	}
}

type testHeaders struct {
	Key   string
	Value string
}

var testMatrix = []struct {
	input      string
	output     string
	outputPath string
	headers    testHeaders
}{
	{
		input:      "/anything/standard",
		output:     "httpbin",
		outputPath: "/anything/standard",
		headers:    testHeaders{},
	},
	{
		input:      "/anything/foo",
		output:     "httpbin",
		outputPath: "/anything/foo",
		headers:    testHeaders{},
	},
	{
		input:      "/anything/bar",
		output:     "httpbin",
		outputPath: "/anything/bar",
		headers:    testHeaders{},
	},
	{
		input:      "/anything/foobar",
		output:     "httpbin",
		outputPath: "/anything/foobar",
		headers:    testHeaders{},
	},
	{
		input:      "/anything/foobar",
		output:     "httpbin",
		outputPath: "/anything/newthingfoobar",
		headers:    testHeaders{"X-BF-Testing", "integralist"},
	},
	{
		input:      "/double-checks",
		output:     "httpbin",
		outputPath: "/anything/toplevel-modified-double-checks",
		headers:    testHeaders{},
	},
	{
		input:      "/double-checks",
		output:     "httpbin",
		outputPath: "/anything/override-modified-double-checks",
		headers:    testHeaders{"X-BF-Testing", "integralist"},
	},
	{
		input:      "/anything/integralist",
		output:     "httpbin",
		outputPath: "/anything/integralist",
		headers:    testHeaders{},
	},
	{
		input:      "/anything/integralist",
		output:     "integralist",
		outputPath: "/about",
		headers:    testHeaders{"X-BF-Testing", "integralist"},
	},
	{
		input:      "/about",
		output:     "httpbin",
		outputPath: "/about",
		headers:    testHeaders{},
	},
	{
		input:      "/about?s=integralist",
		output:     "integralist",
		outputPath: "/about?s=integralist",
		headers:    testHeaders{},
	},
	{
		input:      "/anything/querytest",
		output:     "httpbin",
		outputPath: "/anything/querytest",
		headers:    testHeaders{},
	},
	{
		input:      "/anything/querytest?s=integralistabc",
		output:     "httpbin",
		outputPath: "/anything/querytest?s=integralistabc",
		headers:    testHeaders{},
	},
	{
		input:      "/anything/querytest?s=integralist123",
		output:     "httpbin",
		outputPath: "/anything/newthing123?s=integralist123",
		headers:    testHeaders{},
	},
	{
		input:      "/fooabc",
		output:     "httpbin",
		outputPath: "/anything/fooabc",
		headers:    testHeaders{},
	},
	{
		input:      "/beepboop",
		output:     "httpbin",
		outputPath: "/anything/beepboop",
		headers:    testHeaders{},
	},
}

func TestProxy(t *testing.T) {
	defer mockHTTPBin.Close()
	defer mockIntegralist.Close()
	defer server.Close()

	configureRouting()

	for _, tt := range testMatrix {
		t.Run(tt.input, func(t *testing.T) {
			endpoint := fmt.Sprintf("%s%s", server.URL, tt.input)

			client := &http.Client{}
			req, _ := http.NewRequest("GET", endpoint, nil)

			if tt.headers.Key != "" && tt.headers.Value != "" {
				req.Header.Add(tt.headers.Key, tt.headers.Value)
			}

			res, err := client.Do(req)
			if err != nil {
				log.Fatal(err)
			}

			body, err := ioutil.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				log.Fatal(err)
			}

			jsonResponse := &Response{}

			err = json.Unmarshal(body, jsonResponse)
			if err != nil {
				t.Errorf("Couldn't unmarshall the json response: %s", err)
			}

			verifyResponse(jsonResponse, tt.output, tt.outputPath, t)
		})
	}
}
