package lonelycat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var NotificationTypes = []string{
	"addedAttachmentToCard",
	"addedToBoard",
	"addedToCard",
	"addedToOrganization",
	"addedMemberToCard",
	"addAdminToBoard",
	"addAdminToOrganization",
	"changeCard",
	"closeBoard",
	"commentCard",
	"createdCard",
	"invitedToBoard",
	"invitedToOrganization",
	"removedFromBoard",
	"removedFromCard",
	"removedMemberFromCard",
	"removedFromOrganization",
	"mentionedOnCard",
	"unconfirmedInvitedToBoard",
	"unconfirmedInvitedToOrganization",
	"updateCheckItemStateOnCard",
	"makeAdminOfBoard",
	"makeAdminOfOrganization",
	"cardDueSoon",
	"declinedInvitationToBoard",
	"declinedInvitationToOrganization",
	"memberJoinedTrello",
}

type Notification struct {
	MemberCreator struct {
		Initials string `json:"initials,"`
		FullName string `json:"fullName,"`
		//AvatarHash: null
		Id       string `json:"id,"`
		Username string `json:"username,"`
	} `json:"memberCreator,"`
	Member struct {
		Initials string `json:"initials,"`
		FullName string `json:"fullName,"`
		//AvatarHash: null
		Id       string `json:"id,"`
		Username string `json:"username,"`
	} `json:"member,"`
	Unread          bool      `json:"unread,"`
	Date            time.Time `json:"date,"`
	IdMemberCreator string    `json:"idMemberCreator,"`
	Data            struct {
		Attachment struct {
			Name string `json:"name,"`
			Url  string `json:"url,"`
			Id   string `json:"id,"`
		} `json:"attachment,"`
		Card struct {
			Name      string `json:"name,"`
			Id        string `json:"id,"`
			ShortLink string `json:"shortLink,"`
			IdShort   int    `json:"idShort"`
		} `json:"card,"`
		ListBefore struct {
			Name string `json:"name,"`
			Id   string `json:"id,"`
		} `json:"listBefore,"`
		ListAfter struct {
			Name string `json:"name,"`
			Id   string `json:"id,"`
		} `json:"listAfter,"`
		CheckItem struct {
			Name  string `json:"name,"`
			Id    string `json:"id,"`
			State string `json:"state,"`
		} `json:"checkItem,"`
		Text  string `json:"text,"`
		Name  string `json:"name,"`
		Url   string `json:"url,"`
		State string `json:"state,"`
		Board struct {
			Name      string `json:"name,"`
			Id        string `json:"id,"`
			ShortLink string `json:"shortLink,"`
		} `json:"board,"`
	} `json:"data,"`
	Type string `json:"type,"`
	Id   string `json:"id,"`
}

type CacheEntry struct {
	ETag  string
	Value interface{}
}

type TrelloClient struct {
	ApiKey  string
	Token   string
	BaseURL string
	client  http.Client
	cache   map[string]CacheEntry
}

func NewTrelloClient() *TrelloClient {
	return &TrelloClient{
		cache: make(map[string]CacheEntry),
	}
}

func (tc *TrelloClient) Request(method, path string, body io.Reader) (*http.Request, error) {
	u, err := url.Parse(tc.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path
	v := u.Query()
	v.Set("key", tc.ApiKey)
	v.Set("token", tc.Token)
	u.RawQuery = v.Encode()

	r, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	return r, err
}

func (tc *TrelloClient) Notifications() (notifications []Notification, err error) {
	r, err := tc.Request("GET", "/1/members/me/notifications", nil)
	if err != nil {
		return
	}
	v := r.URL.Query()
	v.Set("read_filter", "unread")
	r.URL.RawQuery = v.Encode()

	resp, err := tc.client.Do(r)
	if err != nil {
		return
	}
	switch resp.StatusCode {
	case http.StatusOK:
		err = json.NewDecoder(resp.Body).Decode(&notifications)
		defer resp.Body.Close()
		if err != nil {
			return
		}
	default:
		err = fmt.Errorf("HTTP Error: %d %s", resp.StatusCode, resp.Status)
	}
	return
}

func (tc *TrelloClient) MarkAllNotificationsRead() error {
	r, err := tc.Request("POST", "/1/notifications/all/read", nil)
	if err != nil {
		return err
	}

	resp, err := tc.client.Do(r)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP Error: %d %s", resp.StatusCode, resp.Status)
	}
	return nil
}
