package lonelycat

import (
	"io/ioutil"
	"log"
	"net"
	"strings"
)

type FakeMsgServer struct {
	ln          net.Listener
	subscribers []chan string
}

func (s *FakeMsgServer) Subscribe(sub chan string) {
	s.subscribers = append(s.subscribers, sub)
}

func (s *FakeMsgServer) handleConn(conn net.Conn) {
	data, err := ioutil.ReadAll(conn)
	if err != nil {
		log.Println(err)
	}
	conn.Close()
	msg := strings.TrimSpace(string(data))
	for _, s := range s.subscribers {
		select {
		case s <- msg:
		default:
		}
	}
}

func (s *FakeMsgServer) ListenAndServe() error {
	ln, err := net.Listen("tcp", ":42042")
	if err != nil {
		return err
	}
	s.ln = ln
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go s.handleConn(conn)
	}
}
