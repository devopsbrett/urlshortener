package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"

	_ "github.com/devopsbrett/shortener/datastore"
	"github.com/devopsbrett/shortener/store"
	"github.com/devopsbrett/shortener/web"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var dbLocation string
var bindAddr string
var storageDB string
var jsonLogs bool

func init() {
	flag.StringVar(&dbLocation, "db", "/data/urlshortener.db", "File/URI location to store kv store. URI should be written as a DNS with embedded auth")
	flag.StringVar(&bindAddr, "bind", ":5000", "The address the web server should bind")
	flag.StringVar(&storageDB, "datastore", "badgerdb", "The datastore that should be used for persistence. (Supported: "+strings.Join(store.GetStores(), ", ")+")")
	flag.BoolVar(&jsonLogs, "json", false, "Enables logging in json format. Set this if you send your logs to Elasticsearch for example")
}

var cpuprofile = flag.String("cpuprofile", "", "File to write cpuprofile output")

func main() {
	flag.Parse()
	if !jsonLogs {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal().Err(err).Msg("Error creating cpuprofile file")
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if _, ok := store.AllStores[storageDB]; !ok {
		storageDB = "badgerdb"
	}

	s, err := store.AllStores[storageDB](dbLocation, log.With().Str("service", "datastore").Logger())
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to open kv store for writing. Set -db 'memory' if persistence is not required")
	}
	defer s.Close()

	webserv := web.NewServer(bindAddr, s, log.With().Str("service", "webserver").Logger())

	shutdown := make(chan bool, 1)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT)

	go func() {
		<-signalChan
		log.Info().Msg("Server is shutting down")
		if err := webserv.Shutdown(); err != nil {
			log.Fatal().Err(err).Msg("Unable to gracefully shutdown the webserver")
		}
		close(shutdown)
	}()
	// spew.Dump(store.AllStores)

	if err := webserv.Serve(); err != nil {
		log.Fatal().Err(err).Msg("Error from web server. Shutting down")
	}

	<-shutdown
	log.Info().Msg("Server Stopped")
}
