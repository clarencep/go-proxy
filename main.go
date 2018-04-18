package main

import (
	"flag"
)

func main() {
	var port int

	flag.IntVar(&port, "port", 1080, "specify the port of the proxy server")
	flag.Parse()

	proxyServer := NewProxyServer(port)
	proxyServer.Start()

}
