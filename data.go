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
	Creator     User      `json:"creator,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	ModifiedAt  time.Time `json:"modified_at,omitempty"`
	ModType     string    `json:"modify_type,omitempty"`
	ModBy       User      `json:"modified_by,omitempty"`
}

func (u User) MarshalJSON() ([]byte, error) {
	m := map[string]string{
		"email": string(u),
		"md5":   md5string(string(u)),
	}

	return json.Marshal(m)
}

func (b Bug) Url() string {
	return "/static/app.html#/bug/" + b.Id
}
