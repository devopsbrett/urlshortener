package store

import "github.com/rs/zerolog"

type Store interface {
	Fetch(string) (URL, error)
	FetchURL(string) string
	Store(*URL) error
	Close() error
	RegisterVisit(*URL) error
}

var AllStores map[string]func(string, zerolog.Logger) (Store, error)

func init() {
	AllStores = make(map[string]func(string, zerolog.Logger) (Store, error))
}

func GetStores() []string {
	st := make([]string, 0, len(AllStores))
	for k := range AllStores {
		st = append(st, k)
	}
	return st
}
