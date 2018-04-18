package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
)

type ProxyServer struct {
	port int
}

func NewProxyServer(port int) *ProxyServer {
	proxyServer := &ProxyServer{
		port: port,
	}

	return proxyServer
}

func (proxyServer *ProxyServer) Start() {
	addr := fmt.Sprintf(":%d", proxyServer.port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Panic("[ERROR] failed to listen at "+addr, err)
	}

	log.Print("[INFO]: proxy server started at " + addr)

	for {
		client, err := l.Accept()
		if err != nil {
			log.Panic("[ERROR] failed to accept incoming request", err)
		}

		go proxyServer.handleProxyClientRequest(client)
	}

}

func (proxyServer *ProxyServer) handleProxyClientRequest(client net.Conn) {
	if client == nil {
		return
	}

	defer logRecoverAsWarning(recover())
	defer client.Close()

	b := make([]byte, 1024)
	n, err := client.Read(b)
	if err != nil {
		log.Printf("[ERROR] failed to read first block: %v", err)
		return
	}

	var method, host, address string
	firstLfPos := bytes.IndexByte(b[:n], '\n')
	if firstLfPos < 0 {
		log.Printf("[ERROR] failed to locate first line break pos from `%s`", string(b[:n]))
		return
	}

	firstLine := string(b[:firstLfPos])
	log.Printf("[PROXY] %s", firstLine)

	fmt.Sscanf(firstLine, "%s%s", &method, &host)
	hostPortURL, err := url.Parse(host)
	if err != nil {
		log.Println("[ERROR] failed to parse host: `%s`, error: %v", host, err)
		return
	}

	if hostPortURL.Opaque == "443" { //https访问
		address = hostPortURL.Scheme + ":443"
	} else { //http访问
		if strings.Index(hostPortURL.Host, ":") == -1 { //host不带端口， 默认80
			address = hostPortURL.Host + ":80"
		} else {
			address = hostPortURL.Host
		}
	}

	//获得了请求的host和port，就开始拨号吧
	server, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("[ERROR] Failed to connect to %v, detail: %v", address, err)
		return
	}

	defer server.Close()

	if method == "CONNECT" {
		_, err := fmt.Fprint(client, "HTTP/1.1 200 Connection established\r\n\r\n")
		if err != nil {
			log.Printf("[ERROR] failed to write response to %s", client.RemoteAddr().String())
			return
		}
	} else {
		written, err := server.Write(b[:n])
		if err != nil {
			log.Printf("[ERROR] failed to write data to %v, detail: %v", address, err)
			return
		}

		for written < n {
			w, err := server.Write(b[written:n])
			if err != nil {
				log.Printf("[ERROR] failed to write data to %v, detail: %v", address, err)
				return
			}

			written += w
		}
	}

	// log.Printf("Got proxy request: %v\n============\n%s========\n", hostPortURL, string(b[:n]))

	//进行转发
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer logRecoverAsWarning(recover())
		defer wg.Done()
		_, err := io.Copy(server, client)
		if err != nil {
			log.Printf("[WARN] failed to copy server response to client, detail: %v", err)
		}
	}()

	_, err = io.Copy(client, server)
	if err != nil {
		log.Printf("[WARN] failed to copy client request to server, detail: %v", err)
	}

	wg.Wait()
}

func logRecoverAsWarning(r interface{}) {
	if r != nil {
		log.Printf("[WARN] %v", r)
	}
}
