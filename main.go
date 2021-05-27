package main

import (
	"fmt"
	"log"

	"github.com/Nikhil12894/url_shortener/config"
	"github.com/Nikhil12894/url_shortener/handler"
	"github.com/Nikhil12894/url_shortener/storage/redis"
	"github.com/valyala/fasthttp"
)

func main() {
	configuration, err := config.FromFile("configuration.json")
	if err != nil {
		log.Fatal(err)
	}

	service, err := redis.New(configuration.Redis.Host, configuration.Redis.Port, configuration.Redis.Password, configuration.Redis.DbName)
	if err != nil {
		log.Fatal(err)
	}
	defer service.Close()

	router := handler.New(configuration.Options.Schema, configuration.Options.Prefix, service)
	fmt.Println("Server is runing on : ", configuration.Options.Prefix, "...")
	log.Fatal(fasthttp.ListenAndServe(":"+configuration.Server.Port, router.Handler))
	service.Close()
}
