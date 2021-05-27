package storage

import (
	"time"
)

type Service interface {
	Save(string, time.Time) (string, error)
	Load(string) (string, error)
	LoadInfo(string) (*Item, error)
	Close() error
}

type Item struct {
	Id      uint64 `json:"id" redis:"id"`
	URL     string `json:"url" redis:"url"`
	Expires string `json:"expires" redis:"expires"`
	Visits  int    `json:"visits" redis:"visits"`
}

type NotFoundError string

func (e NotFoundError) Error() string {
	return string(e)
}

const ErrNoLink NotFoundError = "No Link Found"
