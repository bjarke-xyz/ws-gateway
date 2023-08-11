package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/bjarke-xyz/ws-gateway/internal/cmd"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	err := cmd.ServerCmd(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
