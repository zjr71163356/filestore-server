package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

var conn *sql.DB

const maxOpenConns = 1000

func init() {
	var err error
	conn, err = sql.Open("mysql", "root:master_root_password@tcp(127.0.0.1:3306)/filestore?parseTime=true")

	if err != nil {
		fmt.Println("Failed to Open sql:", err.Error())
		return
	}

	conn.SetMaxOpenConns(maxOpenConns)
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
