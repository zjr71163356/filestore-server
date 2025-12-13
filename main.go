package main

import "filestore-server/pkg/router"

func main() {
	r := router.New()
	r.Run(":8080")
}
