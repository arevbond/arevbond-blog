package app

import (
	"fmt"
	"log/slog"

	"github.com/arevbond/arevbond-blog/internal/config"
	"github.com/arevbond/arevbond-blog/internal/server"
)

// App contains all application dependency and launch http server.
type App struct {
	Server *server.Server
}

func New(log *slog.Logger, cfg config.Config) *App {
	srv := server.New(log, cfg.Server).WithRoutes()

	return &App{
		Server: srv,
	}
}

func (a *App) Run() error {
	if err := a.Server.Run(); err != nil {
		return fmt.Errorf("app run: %w", err)
	}

	return nil
}
