package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
)

func serveTCP(address, message string) {
	l, err := net.Listen("tcp", address)
	defer l.Close()
	if err != nil {
		log.Println(err)
		return
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println(err)
			break
		}
		if _, err := conn.Write([]byte(message)); err != nil {
			log.Println(err)
		}
		if err := conn.Close(); err != nil {
			fmt.Println(err)
		}
	}
}

func mustInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return i
}

func serveUDP(address, port, message string) {
	buf := make([]byte, 16)
	l, err := net.ListenUDP("udp", &net.UDPAddr{net.ParseIP(address), mustInt(port), ""})
	if err != nil {
		log.Println(err)
		return
	}
	defer func() {
		l.Close()
		if f, err := l.File(); err == nil {
			f.Close()
		}
	}()
	for {
		i, addr, err := l.ReadFromUDP(buf)
		if err != nil {
			log.Println(err)
		} else if i > 0 {
			log.Println(string(buf[:i]))
			l.WriteToUDP([]byte(message), addr)
		}
	}
}

func main() {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go serveTCP("0.0.0.0:8000", "hello TCP Dialer")
	go serveUDP("0.0.0.0", "8001", "hello UDP Dialer")
	wg.Wait()
}
