package main

import (
	"os"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

func init() {
	loadEnv()
}

func main() {
	app := newApp()
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func loadEnv() {
	if err := godotenv.Overload(); err != nil {
		log.Fatal("failed to load .env file", err)
	}
}
