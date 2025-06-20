package blog

import (
	"log/slog"

	"github.com/arevbond/arevbond-blog/internal/service/blog/service"
	"github.com/arevbond/arevbond-blog/internal/service/blog/storage"
	"github.com/jmoiron/sqlx"
)

func NewBlogModule(log *slog.Logger, db *sqlx.DB) *service.Blog {
	repo := storage.NewPostsRepo(log, db)

	return service.New(log, repo)
}
