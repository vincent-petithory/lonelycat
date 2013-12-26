package main

import (
	"github.com/vincent-petithory/lonelycat"
	"strings"
)

func RegisterHandlers(lc *lonelycat.LonelyCat) {
	lc.AddHandler(Die)
	lc.AddHandler(MarkAllNotificationsRead)
	lc.AddHandler(Echo)
	lc.AddHandler(Help)
}

func Help(msgIn lonelycat.MessageIn, lc *lonelycat.LonelyCat) error {
	if msgIn.Msg != "!help" {
		return lonelycat.ErrMsgBadSyntax
	}
	lc.Say(msgIn.Source, "I'm just a lonely cat who talks about art.")
	lc.Say(msgIn.Source, "Meow!")
	return nil
}

func Echo(msgIn lonelycat.MessageIn, lc *lonelycat.LonelyCat) error {
	if !strings.HasPrefix(msgIn.Msg, "!echo ") {
		return lonelycat.ErrMsgBadSyntax
	}
	parts := strings.SplitN(msgIn.Msg, " ", 2)
	lc.Say(msgIn.Source, strings.TrimSpace(parts[1]))
	return nil
}

func Die(msgIn lonelycat.MessageIn, lc *lonelycat.LonelyCat) error {
	if msgIn.Msg != "die" {
		return lonelycat.ErrMsgBadSyntax
	}
	lc.Say(msgIn.Source, "Bye!")
	lc.Quit()
	return nil
}

func MarkAllNotificationsRead(msgIn lonelycat.MessageIn, lc *lonelycat.LonelyCat) error {
	if msgIn.Msg != "!notifications read" {
		return lonelycat.ErrMsgBadSyntax
	}
	err := lc.Trello.MarkAllNotificationsRead()
	if err != nil {
		return err
	}
	lc.Say(msgIn.Source, "Done.")
	return nil
}
