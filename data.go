package main

import (
	"time"
)

type Bug struct {
	Id          string    `json:"-"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Creator     string    `json:"creator"`
	Tags        []string  `json:"tags"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
	ModifiedAt  time.Time `json:"modified_at"`
}

func (b Bug) Url() string {
	return "/bug/" + b.Id
}
