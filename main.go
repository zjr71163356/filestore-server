package main

import (
	"fmt"

	"filestore-server/config"
	"filestore-server/pkg/router"
)

func main() {
	cfg := config.MustLoad()
	addr := cfg.Server.Addr
	if addr == "" {
		addr = ":8080"
	}

	r := router.New(cfg)
	if err := r.Run(addr); err != nil {
		fmt.Printf("server exit: %v\n", err)
	}
}
