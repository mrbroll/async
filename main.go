package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/caarlos0/env"
	"github.com/mrbroll/async/registry"
	"github.com/rs/xid"
)

type config struct {
	Host string `env:"HOST"`
	Port int    `env:"PORT"`
}

type message struct {
	Callback string `json:"callback"`
	ID       string `json:"id"`
	Body     string `json:"body"`
}

func (m *message) Read(p []byte) (int, error) {
	data, err := json.Marshal(m)
	if err != nil {
		log.Println("found an error: %s", err)
		return 0, err
	}
	dslice := data
	if len(p) < len(data) {
		dslice = data[0:len(p)]
	}
	copy(p, dslice)
	var e error
	if len(dslice) >= len(data) {
		e = io.EOF
	}
	return len(dslice), e
}

func (m *message) GetID() string {
	return m.ID
}

func main() {
	// get some entropy
	rand.Seed(time.Now().UnixNano())
	reg := registry.Get()
	cfg := new(config)

	if err := env.Parse(cfg); err != nil {
		log.Fatal(err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	// handlers
	requestHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("in main request handler")
		defer r.Body.Close()
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Fatal(err)
		}
		msg := new(message)
		if err := json.Unmarshal(data, msg); err != nil {
			log.Fatal(err)
		}

		msg.Body += fmt.Sprintf(", async request at %s", addr)
		payload, err := json.Marshal(msg)
		if err != nil {
			log.Fatal(err)
		}
		resp, err := http.Post(msg.Callback, "application/json", bytes.NewReader(payload))
		if err != nil {
			log.Fatal(err)
		}
		log.Println(resp)
	})

	callbackHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("in callback handler")
		defer r.Body.Close()
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Fatal(err)
		}
		msg := new(message)
		if err := json.Unmarshal(data, msg); err != nil {
			log.Fatal(err)
		}
		if err := reg.HandleCallback(msg); err != nil {
			log.Fatal(err)
		}
	})

	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("in api handler")
		defer r.Body.Close()
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Fatalf("unable to read request body: %s", err)
		}

		// register callback
		id := xid.New()
		resCh, err := reg.CreateCallback(id.String())

		if err != nil {
			log.Fatalf("unable to create callback: %s", err)
		}
		payload := &message{
			Callback: fmt.Sprintf("http://%s/callback", addr),
			ID:       id.String(),
			Body:     fmt.Sprintf("%s, api request %s", string(data), addr),
		}

		// forward to a random service
		portMap := map[int][]int{
			3000: []int{3001, 3002},
			3001: []int{3000, 3002},
			3002: []int{3000, 3001},
		}
		port := portMap[cfg.Port][rand.Intn(2)]

		url := fmt.Sprintf("http://localhost:%d", port)
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			resp, err := http.Post(url, "application/json", bytes.NewReader(payloadBytes))
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("got response in api handler: %+v\n", resp)
		}()

		log.Println("waiting for response")
		// wait for a response
		resReader := <-resCh
		log.Println("got a response!!!")
		res, err := ioutil.ReadAll(resReader)
		log.Printf("response %s\n", res)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("writing response")
		w.Write(res)
	})

	// mux
	mux := http.NewServeMux()
	mux.Handle("/", requestHandler)
	mux.Handle("/api", apiHandler)
	mux.Handle("/callback", callbackHandler)

	// create http server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	log.Printf("listening on localhost:%d", cfg.Port)

	// start http server
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
