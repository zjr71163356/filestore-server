package db

import (
	"database/sql"
	"fmt"

	"filestore-server/config"

	_ "github.com/go-sql-driver/mysql"
)

var conn *sql.DB

const maxOpenConns = 1000

func init() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Failed to load config:", err.Error())
		return
	}

	conn, err = sql.Open("mysql", cfg.DB.DSN())

	if err != nil {
		fmt.Println("Failed to Open sql:", err.Error())
		return
	}

	maxConns := cfg.DB.MaxOpenConns
	if maxConns <= 0 {
		maxConns = maxOpenConns
	}
	conn.SetMaxOpenConns(maxConns)
	err = conn.Ping()
	if err != nil {
		fmt.Println("Failed to connect to mysql, err:" + err.Error())
		conn = nil
		return
	}

}

func DBconn() *sql.DB {
	return conn
}
