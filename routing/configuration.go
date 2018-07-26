package routing

import "github.com/integralist/go-reverse-proxy/upstreams"

// Override defines the expected sub level routing override configuration
type Override struct {
	Header     string
	Query      string
	Match      string
	MatchType  string
	Upstream   *upstreams.Upstream
	ModifyPath string
}

// Config defines the expected top level routing configuration
type Config struct {
	Path       string
	Upstream   *upstreams.Upstream
	ModifyPath string
	Override   Override
}

// Configuration is the main routing logic that dictates how routing behaviours
// should be controlled and overriding behaviours determined.
var Configuration = []Config{
	Config{
		Path:     "/anything/standard",
		Upstream: upstreams.HTTPBin,
	},
	Config{
		Path:     "/anything/(?:foo|bar)$",
		Upstream: upstreams.HTTPBin,
	},
	Config{
		Path:     "/(?P<start>anything)/(?P<cap>foobar)$",
		Upstream: upstreams.HTTPBin,
		Override: Override{
			Header:     "X-BF-Testing",
			Match:      "integralist",
			ModifyPath: "/anything/newthing$cap",
		},
	},
	Config{
		Path:       "/(?P<cap>double-checks)$",
		Upstream:   upstreams.HTTPBin,
		ModifyPath: "/anything/toplevel-modified-$cap",
		Override: Override{
			Header:     "X-BF-Testing",
			Match:      "integralist",
			ModifyPath: "/anything/override-modified-$cap",
		},
	},
	Config{
		Path:     "/anything/(?P<cap>integralist)",
		Upstream: upstreams.HTTPBin,
		Override: Override{
			Header:     "X-BF-Testing",
			Match:      "integralist",
			ModifyPath: "/about",
			Upstream:   upstreams.Integralist,
		},
	},
	Config{
		Path:     "/about",
		Upstream: upstreams.HTTPBin,
		Override: Override{
			Query:    "s",
			Match:    "integralist",
			Upstream: upstreams.Integralist,
		},
	},
	Config{
		Path:     "/anything/querytest",
		Upstream: upstreams.HTTPBin,
		Override: Override{
			Query:      "s",
			Match:      `integralist(?P<cap>\d{1,3})$`,
			MatchType:  "regex",
			ModifyPath: "/anything/newthing$cap",
		},
	},
	Config{
		Path:       `/(?P<cap>foo\w{3})`,
		Upstream:   upstreams.HTTPBin,
		ModifyPath: "/anything/$cap",
	},
	Config{
		Path:       "/beep(?P<cap>boop)",
		Upstream:   upstreams.HTTPBin,
		ModifyPath: "/anything/beepboop",
	},
}
