package lonelycat

import (
	"io/ioutil"
	"log"
	"net"
	"strings"
)

type NetMsgListener struct {
	ln          net.Listener
	subscribers []chan MessageIn
}

const NetMsgName = "_NetMsg_"

func (s *NetMsgListener) Subscribe() chan MessageIn {
	ch := make(chan MessageIn)
	s.SubscribeWith(ch)
	return ch
}

func (s *NetMsgListener) SubscribeWith(ch chan MessageIn) {
	if ch == nil {
		panic("nil chan")
	}
	s.subscribers = append(s.subscribers, ch)
}

func (s *NetMsgListener) handleConn(conn net.Conn) {
	data, err := ioutil.ReadAll(conn)
	if err != nil {
		log.Println(err)
	}
	conn.Close()
	msg := strings.TrimSpace(string(data))
	msgIn := MessageIn{Msg: msg, Source: NetMsgName}
	for _, s := range s.subscribers {
		select {
		case s <- msgIn:
		default:
		}
	}
}

func (s *NetMsgListener) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
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
