package main

import (
	"encoding/json"
	"strings"
	"time"
)

type User string

type Bug struct {
	Id          string    `json:"id"`
	Type        string    `json:"type"`
	Parent      string    `json:"parent,omitempty"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status,omitempty"`
	Creator     string    `json:"creator,omitempty"`
	Owner       string    `json:"owner,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	ModifiedAt  time.Time `json:"modified_at,omitempty"`
	ModType     string    `json:"modify_type,omitempty"`
	ModBy       string    `json:"modified_by,omitempty"`
	Subscribers []string  `json:"subscribers,omitempty"`
}

type Comment struct {
	Id        string    `json:"id"`
	BugId     string    `json:"bugId"`
	Type      string    `json:"type"`
	Deleted   bool      `json:"deleted"`
	User      string    `json:"user"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
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

type BugState struct {
	Name    string   `json:"name"`
	Order   int      `json:"order"`
	Targets []string `json:"targets,omitempty"`
}

type APIComment Comment

type APIBug Bug

func (u User) MarshalJSON() ([]byte, error) {
	ushort := string(u)
	if x := strings.Index(ushort, "@"); x >= 0 {
		ushort = ushort[:x]
	}
	m := map[string]string{
		"email": ushort,
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

	subs := []User{}
	for _, u := range b.Subscribers {
		subs = append(subs, User(u))
	}

	m["creator"] = User(maybenil(m, "creator"))
	m["owner"] = User(maybenil(m, "owner"))
	m["modified_by"] = User(maybenil(m, "modified_by"))
	m["subscribers"] = subs

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

	m["user"] = User(maybenil(m, "user"))
	return json.Marshal(m)
}

func (b Bug) Url() string {
	return "/bug/" + b.Id
}
