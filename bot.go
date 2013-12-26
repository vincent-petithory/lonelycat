package lonelycat

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
)

var (
	ErrMsgBadSyntax         = errors.New("Message has bad syntax")
	ErrMsgNoSuitableHandler = errors.New("No suitable handler for this message")
)

type LonelyCat struct {
	MessageHandlers []MessageHandler
	QuitChan        chan bool
	msgMutex        sync.Mutex
	currentSink     chan MessageOut
	Trello          *TrelloClient
	L               *log.Logger
}

func NewLonelyCat() *LonelyCat {
	return &LonelyCat{
		MessageHandlers: make([]MessageHandler, 0),
		QuitChan:        make(chan bool),
		L:               log.New(ioutil.Discard, "lonelycat:", log.LstdFlags),
	}
}

type MessageIn struct {
	Msg    string
	Source string
}

type MessageOut struct {
	Msg  string
	Dest string
}

func (lc *LonelyCat) Quit() {
	lc.L.Println("Request quit")
	lc.QuitChan <- true
}

func (lc *LonelyCat) SetLogger(l *log.Logger) {
	lc.L = l
}

func (lc *LonelyCat) Sayf(to string, format string, a ...interface{}) {
	lc.Say(to, fmt.Sprintf(format, a...))
}

func (lc *LonelyCat) Say(to string, msgs ...string) {
	if lc.currentSink != nil {
		msg := strings.Join(msgs, " ")
		lc.currentSink <- MessageOut{Msg: msg, Dest: to}
		return
	}
	panic("LonelyCat: no usable sink")
}

func (lc *LonelyCat) AddHandler(f func(MessageIn, *LonelyCat) error) {
	lc.L.Println("Registering handler")
	lc.MessageHandlers = append(lc.MessageHandlers, f)
}

func (lc *LonelyCat) ProcessMessage(msg MessageIn, sink chan MessageOut) error {
	lc.msgMutex.Lock()
	lc.currentSink = sink
	defer func() {
		lc.currentSink = nil
		lc.msgMutex.Unlock()
	}()
	for _, messageHandler := range lc.MessageHandlers {
		// TODO Separate handler support check
		//      from handler job processing
		err := messageHandler(msg, lc)
		if err == ErrMsgBadSyntax {
			continue
		}
		// Message is handled by this message handler
		if err != nil {
			return err
		}
		return nil
	}
	return ErrMsgNoSuitableHandler
}

type MessageHandler func(MessageIn, *LonelyCat) error
