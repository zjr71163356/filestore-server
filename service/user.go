package service

import (
	"context"
	"filestore-server/pkg/dao"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// RegisterUser 创建新用户并存储哈希密码。
func RegisterUser(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	return dao.CreateUser(ctx, username, string(hashed))
}

// AuthenticateUser 校验用户名密码。
func AuthenticateUser(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}

	u, err := dao.GetUserByName(ctx, username)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return fmt.Errorf("invalid credentials")
	}
	return nil
}
