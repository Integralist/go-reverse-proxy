package main

import (
	"log"
	"net/http"

	"github.com/integralist/go-reverse-proxy/proxy"
	"github.com/integralist/go-reverse-proxy/routing"
)

func main() {
	router := &routing.Handler{}

	for _, conf := range routing.Configuration {
		proxy := proxy.GenerateProxy(conf)

		router.HandleFunc(conf, func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		})
	}

	log.Fatal(http.ListenAndServe(":9001", router))
}
