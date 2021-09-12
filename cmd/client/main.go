package main

import (
	"io"
	"log"
	"net"
)

func dialTCP() {
	for {
		if conn, err := net.Dial("tcp", "0.0.0.0:8000"); err == nil {
			data, err := io.ReadAll(conn)
			if err != nil {
				log.Println(err)
			} else {
				log.Println("Message:", string(data))
				return
			}
		}
	}
}

func dialUDP() {
	for {
		conn, err := net.Dial("udp", "0.0.0.0:8001")
		if err != nil {
			log.Println(err)
		} else {
			defer conn.Close()
			conn.Write([]byte("hello server"))
			buf := make([]byte, 16)
			for {
				i, err := conn.Read(buf)
				if err != nil {
					panic(err)
				}
				if i > 0 {
					log.Println("Message:", string(buf))
					return
				}
			}
		}
	}
}

func main() {
	dialTCP()
	dialUDP()
}
