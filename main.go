package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	srv := &http.Server{
		Addr:         "0.0.0.0:8080",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong!\n"))
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
