package main

import (
	"bytes"
	"dht"
	"dht/bencode"
	"dht/crawler"
	"log"
	"net"
	"os"
	"sync"
)

const (
	addr             = "0.0.0.0"
	port             = 6881
	bufferSize       = 2 << 18
	messageQueueSize = 2 << 12
	network          = "udp4"
)

func createUDPConn() (*net.UDPConn, error) {
	return net.ListenUDP(network, &net.UDPAddr{
		IP:   net.ParseIP(addr),
		Port: port,
		Zone: "",
	})
}

func createCrawler(conn *net.UDPConn) (crawler.Crawler, error) {
	id, err := dht.RandID()
	if err != nil {
		return nil, err
	}
	return crawler.New(dht.NewUDPSender(messageQueueSize, conn), dht.NewDownloader(), id), nil
}

func startMessageHandler(c crawler.Crawler, conn *net.UDPConn) (dht.MessageHandler, error) {
	mh := dht.New()
	if err := mh.RegisterHandler(dht.ResponseType, c.HandleResponse); err != nil {
		return nil, err
	}
	if err := mh.RegisterHandler(dht.QueryType, c.HandleQuery); err != nil {
		return nil, err
	}
	if err := mh.RegisterHandler(dht.ErrorType, func(r dht.Requester, d bencode.Dict) error {
		log.Println(d.Pretty("", "    "))
		return nil
	}); err != nil {
		return nil, err
	}
	go func() {
		buf := make([]byte, bufferSize)
		for {
			n, r, err := conn.ReadFromUDP(buf)
			if err == nil {
				if err := mh.Handle(
					dht.UDPRequester{UDPAddr: r},
					bytes.NewReader(buf[:n]),
				); err != nil {
					log.Println("While handling request:", err)
				}
			} else {
				log.Println("While reading from udp:", err)
			}
		}
	}()
	return mh, nil
}

func startCrawler(c crawler.Crawler) error {
	routerBT, err := dht.ResolveNode(network, "router.bittorrent.com", port)
	if err != nil {
		return err
	}
	transBT, err := dht.ResolveNode(network, "dht.transmissionbt.com", port)
	if err != nil {
		return err
	}
	if err := c.Start([]dht.Node{routerBT, transBT}); err != nil {
		return err
	}
	return nil
}

func main() {
	log.Println("PID:", os.Getpid())
	// TODO: stick a cli in front of this for config options
	// need to wait forever
	wg := sync.WaitGroup{}
	wg.Add(1)

	conn, err := createUDPConn()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	c, err := createCrawler(conn)
	if err != nil {
		panic(err)
	}

	_, err = startMessageHandler(c, conn)
	if err != nil {
		panic(err)
	}

	if err := startCrawler(c); err != nil {
		panic(err)
	}

	// wait group will never be done, wait forever
	wg.Wait()
}
