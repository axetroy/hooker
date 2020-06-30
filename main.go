package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/axetroy/hooker/internal/app"
)

func main() {
	s := &http.Server{
		Addr:           net.JoinHostPort("0.0.0.0", "3000"),
		Handler:        app.Router,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 10M
	}

	var wg sync.WaitGroup
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	exit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(exit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		<-exit
		wg.Add(1)

		//使用context控制srv.Shutdown的超时时间
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := s.Shutdown(ctx)
		if err != nil {
			log.Println(err)
		}
		wg.Done()
	}()

	log.Printf("Listen on:  %s\n", s.Addr)
	if err := s.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			log.Println("HTTP 服务已被关闭")
		} else {
			log.Println(err)
		}
	}
}
