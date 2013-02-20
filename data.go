package main

import (
	"encoding/json"
	"strings"
	"time"
)

type Email string

type Bug struct {
	Id            string    `json:"id"`
	Type          string    `json:"type"`
	Parent        string    `json:"parent,omitempty"`
	Title         string    `json:"title,omitempty"`
	Description   string    `json:"description,omitempty"`
	Status        string    `json:"status,omitempty"`
	Creator       string    `json:"creator,omitempty"`
	Owner         string    `json:"owner,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	ModifiedAt    time.Time `json:"modified_at,omitempty"`
	ModType       string    `json:"modify_type,omitempty"`
	ModBy         string    `json:"modified_by,omitempty"`
	Subscribers   []string  `json:"subscribers,omitempty"`
	AlsoVisibleTo []string  `json:"also_visible_to,omitempty"`
	Private       bool      `json:"private"`
}

type Comment struct {
	Id        string    `json:"id"`
	BugId     string    `json:"bugId"`
	Type      string    `json:"type"`
	Deleted   bool      `json:"deleted"`
	User      string    `json:"user"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
	Private   bool      `json:"private"`
}

func (c Comment) changeObjectFor(u User) (Change, error) {
	bug, err := getBugFor(c.BugId, u)
	if err != nil {
		return Change{}, err
	}

	rv := Change{
		User:    Email(c.User),
		Action:  "commented on",
		Bug:     APIBug(bug),
		BugID:   c.BugId,
		Time:    c.CreatedAt,
		Title:   bug.Title,
		Private: c.Private || bug.Private,
	}

	return rv, nil
}

func (c Comment) IsVisibleTo(u User) bool {
	return u.Internal || (!c.Private)
}

func (c APIComment) IsVisibleTo(u User) bool {
	return u.Internal || (!c.Private)
}

type Attachment struct {
	Id          string    `json:"id"`
	BugId       string    `json:"bugId"`
	Type        string    `json:"type"`
	Url         string    `json:"url"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	Filename    string    `json:"filename"`
	User        string    `json:"user"`
	CreatedAt   time.Time `json:"created_at"`
}

type Ping struct {
	BugId     string    `json:"bugId"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	From      string    `json:"from"`
	To        string    `json:"to"`
}

type BugState struct {
	Name    string   `json:"name"`
	Order   int      `json:"order"`
	Targets []string `json:"targets,omitempty"`
}

type User struct {
	Id        string                 `json:"id"`
	Type      string                 `json:"type"`
	Admin     bool                   `json:"admin"`
	AuthToken string                 `json:"auth_token,omitmepty"`
	Internal  bool                   `json:"internal"`
	Prefs     map[string]interface{} `json:"prefs"`
}

type Reminder struct {
	BugId     string    `json:"bugid"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	When      time.Time `json:"when"`
	User      string    `json:"user"`
}

type Tag struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Subscribers []string `json:"subscribers,omitempty"`
	FGColor     string   `json:"fgcolor,omitempty"`
	BGColor     string   `json:"bgcolor,omitempty"`
}

type APIComment Comment

type APIBug Bug

type APIPing Ping

func (u Email) shortEmail() string {
	ushort := string(u)
	if x := strings.Index(ushort, "@"); x >= 0 {
		ushort = ushort[:x]
	}
	return ushort
}

func (u Email) MarshalJSON() ([]byte, error) {
	m := map[string]string{
		"email": u.shortEmail(),
		"md5":   md5string(string(u)),
	}

	return json.Marshal(m)
}

func maybenil(m map[string]interface{}, k string) string {
	s, _ := m[k].(string)
	return s
}

func (b APIBug) MarshalJSON() ([]byte, error) {
	d, err := json.Marshal(Bug(b))
	if err != nil {
		return nil, err
	}
	m := map[string]interface{}{}
	err = json.Unmarshal(d, &m)
	if err != nil {
		return nil, err
	}

	subs := []Email{}
	for _, u := range b.Subscribers {
		subs = append(subs, Email(u))
	}

	avt := []Email{}
	for _, u := range b.AlsoVisibleTo {
		avt = append(avt, Email(u))
	}

	m["creator"] = Email(maybenil(m, "creator"))
	m["owner"] = Email(maybenil(m, "owner"))
	m["modified_by"] = Email(maybenil(m, "modified_by"))
	m["subscribers"] = subs
	m["also_visible_to"] = avt

	return json.Marshal(m)
}

func (c APIComment) MarshalJSON() ([]byte, error) {
	d, err := json.Marshal(Comment(c))
	if err != nil {
		return nil, err
	}
	m := map[string]interface{}{}
	err = json.Unmarshal(d, &m)
	if err != nil {
		return nil, err
	}

	m["user"] = Email(maybenil(m, "user"))
	return json.Marshal(m)
}

func (p APIPing) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":       p.Type,
		"created_at": p.CreatedAt,
		"from":       Email(p.From),
		"to":         Email(p.To),
	})
}

func (b Bug) Url() string {
	return "/bug/" + b.Id
}

func contains(hay []string, need string) bool {
	for _, straw := range hay {
		if straw == need {
			return true
		}
	}
	return false
}

func (b Bug) IsVisibleTo(u User) bool {
	return u.Internal || !(b.Private && !contains(b.AlsoVisibleTo, u.Id))
}

func (b APIBug) IsVisibleTo(u User) bool {
	return Bug(b).IsVisibleTo(u)
}

func (a Attachment) DownloadUrl() string {
	return "/api/bug/" + a.BugId + "/attachments/att-" +
		a.Id + "/" + a.Filename
}
