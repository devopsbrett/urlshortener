package datastore

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/devopsbrett/shortener/base62"
	"github.com/devopsbrett/shortener/store"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog"
)

func init() {
	store.AllStores["redis"] = NewRedisStore
}

type RedisStore struct {
	db  *redis.Client
	log zerolog.Logger
}

var ctx = context.Background()

func NewRedisStore(location string, log zerolog.Logger) (store.Store, error) {
	u, err := url.Parse(location)
	if err != nil {
		return nil, err
	}
	pass, _ := u.User.Password()
	var db int
	if len(u.Path) > 1 {
		if dbnum, err := strconv.Atoi(u.Path[1:]); err != nil {
			db = dbnum
		}
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     u.Host,
		Password: pass,
		DB:       db,
	})
	redisStore := &RedisStore{
		db:  rdb,
		log: log,
	}
	return redisStore, nil
}

func (r *RedisStore) Close() error {
	return r.db.Close()
}

func (r *RedisStore) fetchKey(id string, u *store.URL) error {
	uhash, err := r.db.HGetAll(ctx, "id:"+id).Result()
	if err != nil {
		return err
	}
	if len(uhash) < 1 {
		return fmt.Errorf("Cannot find URL with code '%s'", id)
	}
	u.ID = id

	if uurl, ok := uhash["url"]; ok {
		u.URL = uurl
	}
	if ucip, ok := uhash["creator_ip"]; ok {
		u.CreatorIP = ucip
	}
	if uadded, ok := uhash["date_added"]; ok {
		u.DateAdded.UnmarshalText([]byte(uadded))
	}
	u.Visits = int(r.db.IncrBy(ctx, "visits:"+id, 0).Val())
	return nil
}

func (r *RedisStore) Fetch(id string) (store.URL, error) {
	var u store.URL
	err := r.fetchKey(id, &u)
	u.ID = id
	return u, err
}

func (r *RedisStore) RegisterVisit(u *store.URL) error {
	return r.db.Incr(ctx, "visits:"+u.ID).Err()
}

func (r *RedisStore) FetchURL(id string) string {
	url, err := r.Fetch(id)
	if err != nil {
		return ""
	}
	return url.URL
}

func (r *RedisStore) Store(u *store.URL) error {
	prefix := base62.PBKey([]byte(u.URL))
	if id, err := r.db.Get(ctx, "bket:"+prefix.String()+":urlid:"+u.URL).Result(); err != redis.Nil {
		r.log.Info().Str("key", id).Msg("Found an existing entry for URL")
		return r.fetchKey(id, u)
	}
	collisions := r.db.Incr(ctx, "bket:"+prefix.String()+":urlcount").Val()
	u.ID = prefix.String() + base62.Itoa(collisions-1)
	u.DateAdded = time.Now()
	r.db.Set(ctx, "bket:"+prefix.String()+":urlid:"+u.URL, u.ID, 0)
	r.db.HSet(ctx, "id:"+u.ID, mapKeyVals(u))
	return nil
}

func mapKeyVals(u *store.URL) map[string]interface{} {
	retMap := make(map[string]interface{})
	retMap["id"] = u.ID
	retMap["url"] = u.URL
	retMap["date_added"] = u.DateAdded
	retMap["creator_ip"] = u.CreatorIP
	return retMap
}
