package web

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/devopsbrett/shortener/store"
	"github.com/gorilla/handlers"
	"github.com/naoina/denco"
	"github.com/rs/zerolog"
)

type Server struct {
	bindAddr string
	store    store.Store
	handlers []denco.Handler
	log      zerolog.Logger
	srv      *http.Server
}

func NewServer(bindAddr string, store store.Store, log zerolog.Logger) *Server {
	return &Server{
		bindAddr: bindAddr,
		store:    store,
		log:      log,
	}
}

func (s *Server) buildHandlers() (http.Handler, error) {
	mux := denco.NewMux()
	handler, err := mux.Build([]denco.Handler{
		mux.GET("/:id", s.redirectHandler),
		mux.POST("/api/v1/shorten/*url", s.shortenHandler),
		mux.POST("/api/v1/shorten", s.postShortenHandler),
		mux.GET("/api/v1/lookup/:id", s.lookupHandler),
	})
	return handler, err
}

func (s *Server) postShortenHandler(w http.ResponseWriter, r *http.Request, ps denco.Params) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	url := r.PostFormValue("url")
	s.newURL(w, r, url)
}

func (s *Server) newURL(w http.ResponseWriter, r *http.Request, url string) {
	if url[0:4] != "http" {
		url = "http://" + url
	}

	u, err := store.NewURL(s.store, url, r.RemoteAddr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%s\n", err)
		return
	}
	s.log.Info().Str("Original URL", u.URL).Str("Shortened", string(u.ID)).Msg("Stored shortened url")
	s.urlDisplay(w, r, u)
}

func (s *Server) redirectHandler(w http.ResponseWriter, r *http.Request, ps denco.Params) {
	id := ps.Get("id")
	u, err := s.store.Fetch(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	err = s.store.RegisterVisit(&u)
	w.Header().Add("location", u.URL)
	w.WriteHeader(http.StatusPermanentRedirect)
}

func (s *Server) urlDisplay(w http.ResponseWriter, r *http.Request, u store.URL) {
	u.ShortURL = fmt.Sprintf("http://%s/%s", r.Host, u.ID)
	u.CreatorIP = ""
	switch getOutputFormat(r) {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(u)
	case "xml":
		w.Header().Set("Content-Type", "text/xml")
		xml.NewEncoder(w).Encode(u)
	default:
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, u.ShortURL)
	}
}

func (s *Server) lookupHandler(w http.ResponseWriter, r *http.Request, ps denco.Params) {
	id := ps.Get("id")
	u, err := s.store.Fetch(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.urlDisplay(w, r, u)
}

func getOutputFormat(r *http.Request) string {
	for _, h := range strings.Split(r.Header.Get("Accept"), ",") {
		if len(h) >= 4 && h[len(h)-4:] == "json" {
			return "json"
		}
		if len(h) >= 3 && h[len(h)-3:] == "xml" {
			return "xml"
		}
	}
	return "plain"
}

func (s *Server) shortenHandler(w http.ResponseWriter, r *http.Request, ps denco.Params) {
	url := ps.Get("url")[:]
	query := r.URL.RawQuery
	if query != "" {
		url += "?" + query
	}
	s.newURL(w, r, url)
}

func (s *Server) Serve() error {
	handler, err := s.buildHandlers()
	if err != nil {
		return err
	}
	s.srv = &http.Server{
		Handler:           handlers.LoggingHandler(os.Stdout, handler),
		Addr:              s.bindAddr,
		WriteTimeout:      10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
	}

	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.srv.SetKeepAlivesEnabled(false)
	return s.srv.Shutdown(ctx)
}
