package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// 1 MB
const maxMemory = 1 * 1024 * 1024

type queueadt interface {
	enqueueHead(data []byte) *queuenode
	enqueue(data []byte) *queuenode
	dequeue() *queuenode
	size() int
}

type queue struct {
	head, tail *queuenode
	count      int
}

type queuenode struct {
	next *queuenode
	prev *queuenode
	data []byte
	key  string
}

func (q *queue) enqueueHead(key string, data []byte) *queuenode {
	newNode := &queuenode{
		next: q.head,
		prev: nil,
		key:  key,
		data: data,
	}
	q.head = newNode
	if q.tail == nil {
		q.tail = q.head
	}
	q.count++
	return q.head
}

func (q *queue) enqueue(key string, data []byte) *queuenode {
	if q.tail != nil {
		newNode := &queuenode{
			next: nil,
			prev: q.tail,
			key:  key,
			data: data,
		}
		q.tail.next = newNode
		q.tail = newNode
	} else {
		newNode := &queuenode{
			next: nil,
			prev: nil,
			key:  key,
			data: data,
		}
		q.tail = newNode
		q.head = q.tail
	}

	q.count++

	return q.tail
}

func (q *queue) dequeue() *queuenode {
	if q.head == nil {
		return nil
	}

	oldNode := q.head
	q.head = q.head.next
	q.count--
	return oldNode
}

func (q *queue) size() int {
	return q.count
}

// add mutex if feeling unsafe
type lrucache struct {
	hash     map[string]*queuenode
	list     *queue
	capacity int
}

func (lru *lrucache) set(key string, val []byte) {
	if node, ok := lru.hash[key]; ok {
		// overwrite data
		node.data = val
		// update lru status
		// update references & move to head
		if node.prev != nil {
			node.prev.next = node.next
		}
		if node.next != nil {
			node.next.prev = node.prev
		}

		node.next = lru.list.head
		node.prev = nil
		lru.list.head = node
		return
	}

	lru.hash[key] = lru.list.enqueueHead(key, val)
	if lru.capacity != 0 && lru.list.size() > lru.capacity {
		deletedNode := lru.list.dequeue()
		delete(lru.hash, deletedNode.key)
	}
}

func (lru *lrucache) get(key string) []byte {
	if node, ok := lru.hash[key]; ok {
		data := node.data

		if node.prev != nil {
			node.prev.next = node.next
		}
		if node.next != nil {
			node.next.prev = node.prev
		}

		node.next = lru.list.head
		node.prev = nil
		lru.list.head = node

		return data
	}
	return nil
}

func newLRUCache(capacity int) *lrucache {
	return &lrucache{
		hash:     map[string]*queuenode{},
		list:     &queue{},
		capacity: capacity,
	}
}

func main() {
	srv := &http.Server{
		Addr:         "0.0.0.0:8080",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}

	cache := newLRUCache(3)

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong!\n"))
	})

	http.Handle("/", http.FileServer(http.Dir("./static")))

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")

			// this totally doesn't work lol
			if err := r.ParseMultipartForm(maxMemory); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{ "error": "file size too large" }`))
				return
			}

			file, handler, err := r.FormFile("file")
			if err != nil {
				log.Println("Error retrieving file from form-data")
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest) // find better status
				w.Write([]byte(`{ "error": "file lost" }`))
				return
			}
			defer file.Close()
			log.Printf("Uploaded File: %+v\n", handler.Filename)
			log.Printf("Uploaded File Size: %+v\n", handler.Size)
			log.Printf("MIME Header: %+v\n", handler.Header)

			fileBytes, err := ioutil.ReadAll(file)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest) // find better status
				w.Write([]byte(`{ "error": "file lost" }`))
				return
			}

			// cache it
			cache.set(handler.Filename, fileBytes)

			jsonResponse, err := json.Marshal(struct {
				Data []byte `json:"data"`
			}{Data: fileBytes})
			if err != nil {
				w.WriteHeader(http.StatusBadRequest) // find better status
				w.Write([]byte(`{ "error": "json failure" }`))
				return
			}

			w.WriteHeader(http.StatusCreated)
			w.Write(jsonResponse)
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("request not found"))
			return
		}
	})

	http.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")

			keys, ok := r.URL.Query()["key"]
			if !ok || len(keys) < 1 {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{ "error": "key is missing" }`))
				return
			}
			key := keys[0]
			data := cache.get(key)
			if data == nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{ "error": "no file found for key" }`))
				return
			}

			jsonResponse, err := json.Marshal(struct {
				Data []byte `json:"data"`
			}{Data: data})
			if err != nil {
				w.WriteHeader(http.StatusBadRequest) // find better status
				w.Write([]byte(`{ "error": "json failure" }`))
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write(jsonResponse)
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("request not found"))
		}
	})

	// start server without blocking
	go func() {
		log.Fatal(srv.ListenAndServe())
	}()

	log.Println("Server running on port 8080. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	// Block until signal received
	<-sc

	// Create a deadline to wait for
	gracefulTimeout := time.Second * 7
	ctx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
	defer cancel()

	// Doesn't block if srv has no connections
	srv.Shutdown(ctx)
	log.Println("Shutting down server...")
	os.Exit(0)
}
