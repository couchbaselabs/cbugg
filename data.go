package main

import (
	"encoding/json"
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
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	ModifiedAt  time.Time `json:"modified_at,omitempty"`
	ModType     string    `json:"modify_type,omitempty"`
	ModBy       string    `json:"modified_by,omitempty"`
}

type APIBug Bug

func (u User) MarshalJSON() ([]byte, error) {
	m := map[string]string{
		"email": string(u),
		"md5":   md5string(string(u)),
	}

	return json.Marshal(m)
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

	maybenil := func(k string) string {
		s, _ := m[k].(string)
		return s
	}

	m["creator"] = User(maybenil("creator"))
	m["modified_by"] = User(maybenil("modified_by"))
	return json.Marshal(m)
}

func (b Bug) Url() string {
	return "/static/app.html#/bug/" + b.Id
}
