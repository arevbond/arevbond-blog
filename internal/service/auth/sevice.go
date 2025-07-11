package auth

import (
	"log/slog"

	"github.com/arevbond/arevbond-blog/internal/service/auth/service"
)

func NewAuthModule(log *slog.Logger, adminToken string, secretKeyJWT string) *service.Auth {
	return service.New(log, adminToken, secretKeyJWT)
}
