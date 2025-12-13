package dao

import (
	"context"
	"database/sql"
	"filestore-server/pkg/db"
	"fmt"
	"strings"
	"time"
)

type User struct {
	UserName   string
	Password   string
	Email      string
	Phone      string
	EmailValid bool
	PhoneValid bool
	SignupAt   sql.NullTime
	LastActive sql.NullTime
	Profile    sql.NullString
	Status     int
}

// CreateUser 插入新用户，假设 user_name 唯一。
func CreateUser(ctx context.Context, username, hashedPwd string) error {
	const sqlStr = "insert into tbl_user (`user_name`,`user_pwd`,`signup_at`,`status`) values (?,?,?,?)"

	conn := db.DBconn()
	if conn == nil {
		return fmt.Errorf("db connection is nil")
	}

	_, err := conn.ExecContext(ctx, sqlStr, username, hashedPwd, time.Now(), 1)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return fmt.Errorf("user already exists")
		}
		return fmt.Errorf("failed to insert user: %w", err)
	}
	return nil
}

// GetUserByName 返回用户记录。
func GetUserByName(ctx context.Context, username string) (User, error) {
	const sqlStr = `
select user_name, user_pwd, email, phone, email_validated, phone_validated,
       signup_at, last_active, profile, status
from tbl_user where user_name=? limit 1`

	conn := db.DBconn()
	if conn == nil {
		return User{}, fmt.Errorf("db connection is nil")
	}

	var u User
	err := conn.QueryRowContext(ctx, sqlStr, username).Scan(
		&u.UserName,
		&u.Password,
		&u.Email,
		&u.Phone,
		&u.EmailValid,
		&u.PhoneValid,
		&u.SignupAt,
		&u.LastActive,
		&u.Profile,
		&u.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, fmt.Errorf("user not found")
		}
		return User{}, fmt.Errorf("failed to query user: %w", err)
	}
	return u, nil
}
