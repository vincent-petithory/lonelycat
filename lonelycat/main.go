package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	irc "github.com/thoj/go-ircevent"
	"github.com/vincent-petithory/lonelycat"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"text/template"
	"time"
)

var configPath = flag.String("config", "config.json", "Path to the config file.")

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: lonelycat [OPTIONS]
This program is an IRC Bot which uses Trello APIs to
retrieve changes from boards and cards.`)
		flag.PrintDefaults()
	}
	flag.Parse()
}

var config struct {
	Trello struct {
		ApiKey string `json:"api_key,"`
		Token  string `json:"token,"`
	} `json:"trello,"`
	LonelyCat struct {
		Addr                    string            `json:"addr,"`
		NotificationsFetchDelay int               `json:"notificationsFetchDelay,"`
		LogFile                 string            `json:"logFile,"`
		TemplatesGlob           string            `json:"templatesGlob,"`
		BoardIrcChanMap         map[string]string `json:"boardIrcChanMap,"`
	} `json:"lonelycat,"`
	Irc struct {
		Nick            string   `json:"nick,"`
		User            string   `json:"user,"`
		Addr            string   `json:"addr,"`
		AuthorizedNicks []string `json:"authorizedNicks,"`
	} `json:"irc,"`
}

func parseConfig() error {
	data, err := ioutil.ReadFile(*configPath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		return err
	}
	return nil
}

func FetchAndRelayNewNotifications(lc *lonelycat.LonelyCat, sink chan lonelycat.Notification) error {
	defer close(sink)
	notifications, err := lc.Trello.Notifications()
	if err != nil {
		return err
	}
	var revNotifications []lonelycat.Notification
	for _, notification := range notifications {
		revNotifications = append([]lonelycat.Notification{notification}, revNotifications...)
	}

	for _, notification := range revNotifications {
		sink <- notification
	}
	err = lc.Trello.MarkAllNotificationsRead()
	if err != nil {
		return err
	}
	return nil
}

var ErrNoTemplate = errors.New("No template defined for this notification type")

func FormatNotification(t *template.Template, n lonelycat.Notification) (string, error) {
	tpl := t.Lookup(n.Type)
	if tpl == nil {
		return "", ErrNoTemplate
	}
	var buf bytes.Buffer
	err := tpl.Execute(&buf, n)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func loadNotificationTemplates() (*template.Template, error) {
	t, err := template.New("notification").ParseGlob(config.LonelyCat.TemplatesGlob)
	if err != nil {
		return nil, err
	}
	for _, typ := range lonelycat.NotificationTypes {
		if t.Lookup(typ) != nil {
			log.Printf("Template for type \"%s\": Yes.\n", typ)
		} else {
			log.Printf("Template for type \"%s\": No.\n", typ)
		}
	}
	return t, nil
}

func main() {
	if err := parseConfig(); err != nil {
		log.Fatal(err)
	}
	notificationTemplates, err := loadNotificationTemplates()
	if err != nil {
		log.Fatal(err)
	}

	var logSink io.Writer
	if config.LonelyCat.LogFile != "" {
		logFile, err := os.OpenFile(config.LonelyCat.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
		if err != nil {
			log.Fatal(err)
		}
		defer logFile.Close()
		logSink = io.MultiWriter(os.Stdout, logFile)
	} else {
		logSink = os.Stdout
	}

	s := &lonelycat.NetMsgListener{}
	go s.Listen(config.LonelyCat.Addr)

	// Where to write msg replies
	replyChan := make(chan lonelycat.MessageOut)

	lc := lonelycat.NewLonelyCat()

	tc := lonelycat.NewTrelloClient()
	tc.ApiKey = config.Trello.ApiKey
	tc.Token = config.Trello.Token
	tc.BaseURL = "https://api.trello.com"

	lc.Trello = tc
	lc.L = log.New(logSink, "lonelycat: ", log.LstdFlags)
	log.SetOutput(logSink)

	RegisterHandlers(lc)

	var quitCounter int
	// Listen for messages coming from msg server
	msgChan := make(chan lonelycat.MessageIn)
	s.SubscribeWith(msgChan)
	go func() {
		log.Println("Listening for local messages...")
		for {
			select {
			case msg := <-msgChan:
				lc.L.Printf("Local msg: %s\n", msg)
				err := lc.ProcessMessage(msg, replyChan)
				switch {
				case err == lonelycat.ErrMsgNoSuitableHandler:
					lc.L.Println(err)
				case err != nil:
					lc.L.Println(err)
					replyChan <- lonelycat.MessageOut{
						Msg:  "An error occurred.",
						Dest: msg.Source,
					}
				}
			case v := <-lc.QuitChan:
				quitCounter--
				lc.QuitChan <- v
				return
			}
		}
	}()
	quitCounter++

	// Start trello notifications fetcher
	delay := time.Duration(int64(time.Second) * int64(config.LonelyCat.NotificationsFetchDelay))
	go func() {
		log.Println("Started notifications poller...")
		var m sync.Mutex
		for {
			select {
			case <-time.After(delay):
				m.Lock()
				go func() {
					defer func() {
						m.Unlock()
						log.Println("Fetch done")
					}()
					log.Println("Starting fetch")
					relayChan := make(chan lonelycat.Notification)
					go func() {
						err := FetchAndRelayNewNotifications(lc, relayChan)
						if err != nil {
							lc.L.Println(err)
							return
						}
					}()
					for n := range relayChan {
						s, err := FormatNotification(notificationTemplates, n)
						if err == ErrNoTemplate {
							continue
						}
						if err != nil {
							lc.L.Println(err)
							continue
						}
						ch, ok := config.LonelyCat.BoardIrcChanMap[n.Data.Board.Name]
						if !ok {
							lc.L.Printf("No irc chan for board \"%s\"\n", n.Data.Board.Name)
							continue
						}
						msg := lonelycat.MessageOut{
							Msg:  s,
							Dest: ch,
						}
						replyChan <- msg
					}
				}()
			case v := <-lc.QuitChan:
				quitCounter--
				lc.QuitChan <- v
				return
			}
		}
	}()
	quitCounter++

	irccon := irc.IRC(config.Irc.Nick, config.Irc.User)
	err = irccon.Connect(config.Irc.Addr)
	if err != nil {
		log.Fatal(err)
	}
	irccon.AddCallback("001", func(e *irc.Event) {
		for _, ch := range config.LonelyCat.BoardIrcChanMap {
			irccon.Join(ch)
		}
	})
	irccon.AddCallback("PRIVMSG", func(e *irc.Event) {
		isAuthorizedNick := false
		for _, u := range config.Irc.AuthorizedNicks {
			if u == e.Nick {
				isAuthorizedNick = true
				break
			}
		}
		if !isAuthorizedNick {
			return
		}
		msgChan <- lonelycat.MessageIn{Msg: e.Message, Source: e.Nick}
	})
	go irccon.Loop()

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt)
	log.Println("Listening for reply messages...")
	for {
		select {
		case <-sigCh:
			lc.Quit()
		case v := <-lc.QuitChan:
			if quitCounter > 0 {
				lc.QuitChan <- v
			} else {
				irccon.Quit()
				return
			}
		case msg := <-replyChan:
			cleanMsg := strings.Replace(msg.Msg, "\n", " ", -1)
			if len(cleanMsg) == 0 || msg.Dest == "" {
				break
			}
			log.Printf("[OUT] ->%s: %s\n", msg.Dest, cleanMsg)
			switch msg.Dest {
			case lonelycat.NetMsgName:
			case "":
			default:
				irccon.Privmsg(msg.Dest, cleanMsg)
				time.Sleep(time.Millisecond * 100)
			}
		}
	}
}
