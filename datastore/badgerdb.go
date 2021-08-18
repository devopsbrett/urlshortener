package datastore

import (
	"encoding/json"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/devopsbrett/shortener/base62"
	"github.com/devopsbrett/shortener/store"
	badger "github.com/dgraph-io/badger/v3"
	"github.com/rs/zerolog"
)

type BadgerStore struct {
	db  *badger.DB
	log zerolog.Logger
}

func init() {
	store.AllStores["badgerdb"] = NewBadgerDBStore
}

func NewBadgerDBStore(location string, log zerolog.Logger) (store.Store, error) {
	var opts badger.Options
	if location == "memory" {
		opts = badger.DefaultOptions("").WithInMemory(true)
	} else {
		opts = badger.DefaultOptions(location)
	}
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	bStore := &BadgerStore{
		db:  db,
		log: log,
	}
	return bStore, nil
}

func (b *BadgerStore) RegisterVisit(u *store.URL) error {
	u.Visits += 1
	spew.Dump(u)
	return b.db.Update(func(txn *badger.Txn) error {
		urlBytes, err := json.Marshal(u)
		if err != nil {
			return err
		}
		return txn.Set([]byte(u.ID), urlBytes)
	})
}

func (b *BadgerStore) Store(u *store.URL) error {
	var urlMarshal store.URL
	idPrefix := base62.PBKey([]byte(u.URL))
	b.log.Debug().Str("prefix", idPrefix.String()).Msg("Scanning datastore with prefix")
	err := b.db.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		var counter uint64
		for it.Seek(idPrefix.Bytes()); it.ValidForPrefix(idPrefix.Bytes()); it.Next() {
			counter += 1
			err := it.Item().Value(func(v []byte) error {
				return json.Unmarshal(v, &urlMarshal)
			})
			if err != nil {
				return err
			}
			if urlMarshal.URL == u.URL {
				u.ID = string(it.Item().Key())
				u.DateAdded = urlMarshal.DateAdded
				u.Visits = urlMarshal.Visits
				b.log.Info().Str("key", u.ID).Msg("Found an existing entry for URL")
				return nil
			}
		}
		if counter > 0 {
			b.log.Info().Str("prefix", idPrefix.String()).Uint64("collisions", counter).Msg("Collisions detected")
		}
		newid := append(idPrefix.Bytes(), base62.Itob(counter)...)
		u.DateAdded = time.Now()
		urlBytes, err := json.Marshal(u)
		if err != nil {
			return err
		}
		e := badger.NewEntry(newid, urlBytes)
		err = txn.SetEntry(e)
		if err != nil {
			return err
		}
		b.log.Debug().Bytes("key", newid).Str("url", u.URL).Msg("New URL stored with key")
		u.ID = string(newid)
		return nil
	})
	return err
}

func createPrefix(u string) []byte {

	prefix := base62.PBKey([]byte(u))
	// var p byte
	// var offset uint64
	// for i := range prefix {
	// 	p, offset = base62.EncodeWithOffset((hashstr[i] + byte(offset)) ^ hashstr[i+8])
	// 	prefix[i] = p
	// }
	return prefix.Bytes()
}

func (b *BadgerStore) Fetch(id string) (store.URL, error) {
	var u store.URL
	err := b.fetchKey([]byte(id), &u)
	u.ID = id
	return u, err
}

func (b *BadgerStore) FetchURL(id string) string {
	var u store.URL
	err := b.fetchKey([]byte(id), &u)
	if err != nil {
		return ""
	}
	return u.URL
}

func (b *BadgerStore) fetchKey(id []byte, u *store.URL) error {
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(id)
		if err != nil {
			return err
		}
		// var urlBytes []byte
		err = item.Value(func(val []byte) error {
			// urlBytes = append([]byte{}, val...)
			return json.Unmarshal(val, u)
		})
		return err
	})
	return err
}

func (b *BadgerStore) Close() error {
	return b.db.Close()
}
