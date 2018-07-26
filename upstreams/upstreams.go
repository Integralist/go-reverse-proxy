package upstreams

// Upstream defines the backend origin to be proxied
type Upstream struct {
	Name string
	Host string
}

// HTTPBin is used for testing
var HTTPBin = &Upstream{
	Name: "httpbin",
	Host: "httpbin.org",
}

// Google is used for more testing
var Google = &Upstream{
	Name: "google",
	Host: "google.com",
}

// Integralist is used for even more testing
var Integralist = &Upstream{
	Name: "integralist",
	Host: "integralist.co.uk",
}
