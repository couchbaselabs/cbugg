package main

import (
	"time"
)

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

func (b Bug) Url() string {
	return "/static/app.html#/bug/" + b.Id
}
