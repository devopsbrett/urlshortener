package web

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
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
}

func NewServer(bindAddr string, store store.Store, log zerolog.Logger) *Server {
	// router := denco.NewMux()
	// router.GlobalOPTIONS = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	if r.Header.Get("Access-Control-Request-Method") != "" {
	// 		// Set CORS headers
	// 		header := w.Header()
	// 		header.Set("Access-Control-Allow-Methods", header.Get("Allow"))
	// 		header.Set("Access-Control-Allow-Origin", "*")
	// 	}

	// 	// Adjust status code to 204
	// 	w.WriteHeader(http.StatusNoContent)
	// })

	return &Server{
		bindAddr: bindAddr,
		store:    store,
		log:      log,
	}
	// err := srv.addRoutes()
}

func (s *Server) buildHandlers() (http.Handler, error) {
	mux := denco.NewMux()
	handler, err := mux.Build([]denco.Handler{
		mux.GET("/:id", s.redirectHandler),
		mux.POST("/api/v1/shorten/*url", s.shortenHandler),
		mux.POST("/api/v1/shorten", s.postShortenHandler),
		mux.GET("/api/v1/lookup/:id", s.lookupHandler),
	})
	// s.handlers = append(s.handlerss.router.GET("/:id", s.redirectHandler)
	// s.router.POST("/api/v1/shorten/*url", s.shortenHandler)
	// // api.PathPrefix("/shorten").Handler(http.StripPrefix("/api/v1/shorten", s.shortenHandler))
	// s.router.GET("/api/v1/lookup/:id", s.lookupHandler)
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
	// md5int, err := strconv.ParseInt(string(h.Sum(nil)), 16, 64)
	// spew.Dump(md5int)
	// spew.Dump(err)
	s.urlDisplay(w, r, u)
}

func (s *Server) redirectHandler(w http.ResponseWriter, r *http.Request, ps denco.Params) {
	id := ps.Get("id")
	u, err := s.store.Fetch(id)
	spew.Dump(u)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	err = s.store.RegisterVisit(&u)
	spew.Dump(err)
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
	srv := &http.Server{
		Handler: handlers.LoggingHandler(os.Stdout, handler),
		Addr:    s.bindAddr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
	srv.SetKeepAlivesEnabled(false)

	return srv.ListenAndServe()
}
