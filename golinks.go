package main

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// you may want to take a regular backup of your db
const DB string = "db.gob"
const SYNC_DELAY = 5 * time.Second

// A golink entry in db
type Entry struct {
	Hash    string `json:"hash"`
	Url     string `json:"url"`
	Created int64  `json:"created"`
	Updated int64  `json:"updated"`
}

// A wrappr over Entry map to make it concurrent access safe
type DBStruct struct {
	sync.RWMutex
	entries map[string]Entry
}

var Records = DBStruct{entries: make(map[string]Entry)}

func Show(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	hash := vars["hash"]
	Records.RLock()
	defer Records.RUnlock()

	val, found := Records.entries[hash]
	if found {
		if strings.Contains(val.Url, "http://") || strings.Contains(val.Url, "https://") {
			http.Redirect(w, req, val.Url, http.StatusTemporaryRedirect)
		} else {
			http.Redirect(w, req, "http://"+val.Url, http.StatusTemporaryRedirect)
		}
		return
	}
	http.Error(w, "NOT FOUND", http.StatusNotFound)
}

// Method
func GetHash(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	hash := vars["hash"]
	response := json.NewEncoder(w)

	Records.RLock()
	defer Records.RUnlock()

	w.Header().Set("content-type", "application/json")
	if val, found := Records.entries[hash]; found {
		response.Encode(val)
	} else {
		w.WriteHeader(http.StatusNotFound)
		response.Encode(Entry{Hash: hash})
		//http.Error(w, "NOT FOUND", http.StatusNotFound)
	}
}

func mapWorker(file *os.File, entry <-chan Entry) {
	t := time.Now()
	var encoder *gob.Encoder
	for {
		e := <-entry
		Records.Lock()
		Records.entries[e.Hash] = e
		Records.Unlock()
		// Play around with this settings to see an optimum value
		// For an internal tool, 5 sec lazy persistences should be good enough
		// If you want to make it a production quality service, several things
		// needs to be taken into consideration, including scaling and choosing
		// a proper backend database which supports sharding/replication among
		// other things
		if time.Since(t) > time.Duration(SYNC_DELAY) {
			file.Seek(0, 0)
			Records.RLock()
			encoder = gob.NewEncoder(file)
			if err := encoder.Encode(Records.entries); err != nil {
				fmt.Println("Error persisting data", err)
			} else {
				fmt.Println("Succesfully written data back to disk")
			}
			Records.RUnlock()
			t = time.Now()
		}
	}
}

func SetHash(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	ctx := req.Context()
	ts := time.Now()
	record := ctx.Value("queue").(chan Entry)

	hash := vars["hash"]
	url := req.FormValue("url")

	Records.RLock()
	val, found := Records.entries[hash]
	Records.RUnlock()

	if found {
		// prevUrl := val.Url
		val.Url = url
		val.Updated = ts.Unix()
	} else {
		now := ts.Unix()
		val = Entry{hash, url, now, now}
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.Encode(val)
	record <- val
}

func main() {
	file, err := os.OpenFile(DB, os.O_RDWR, 0600)
	if err != nil {
		if !os.IsExist(err) {
			file, err = os.Create(DB)
			fmt.Println("Creating DB ...")
		} else {
			panic(err)
		}
	}
	defer file.Close()
	dec := gob.NewDecoder(file)
	enc := gob.NewEncoder(file)
	err = dec.Decode(&Records.entries)
	fmt.Printf("Found %d entries\n", len(Records.entries))
	if err != nil && err != io.EOF {
		panic(err)
	}

	// reset file pointer
	file.Seek(0, 0)
	// Channel where records will be pushed
	writeQueue := make(chan Entry)
	middleWare := func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// middleware logic
			ctx := context.WithValue(r.Context(), "encoder", enc)
			ctx = context.WithValue(ctx, "queue", writeQueue)
			start := time.Now()
			next.ServeHTTP(w, r.WithContext(ctx))
			fmt.Println(r.URL, "completed in", time.Since(start))
		})
	}

	// db update worker
	go mapWorker(file, writeQueue)

	router := mux.NewRouter()
	router.HandleFunc("/api/get/{hash}", middleWare(GetHash)).Methods("GET")
	router.HandleFunc("/api/set/{hash}", middleWare(SetHash)).Methods("POST")
	router.HandleFunc("/v/{hash}", middleWare(Show)).Methods("GET")

	server := &http.Server{
		Handler:      router,
		Addr:         ":8085",
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
	fmt.Println(server.ListenAndServe())
}
