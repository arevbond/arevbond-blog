package server

import (
	"context"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/arevbond/arevbond-blog/internal/middleware"
	"github.com/arevbond/arevbond-blog/internal/service/blog/domain"
)

type Blog interface {
	Posts(ctx context.Context, params domain.SelectPostsParams) ([]*domain.Post, error)
	Post(ctx context.Context, id int) (*domain.Post, error)
	PostBySlug(ctx context.Context, slug string) (*domain.Post, error)
	CreatePost(ctx context.Context, params domain.CreatePostParams) (*domain.Post, error)
	UpdatePost(ctx context.Context, params domain.UpdatePostParams) error
	DeletePost(ctx context.Context, id int) error
	ChangePublishStatus(ctx context.Context, id int, curPublishStatus bool) error

	Categories(ctx context.Context) ([]*domain.Category, error)

	MdToHTML(md []byte) []byte
}

func (s *Server) registerBlogRoutes(mux *http.ServeMux) {
	mux.Handle("GET /blog/posts", middleware.OptionalAuth(s.Auth, s.log)(http.HandlerFunc(s.postsPage)))
	mux.Handle("GET /blog/posts/more", middleware.OptionalAuth(s.Auth, s.log)(http.HandlerFunc(s.posts)))
	mux.Handle("GET /blog/posts/{slug}", middleware.OptionalAuth(s.Auth, s.log)(http.HandlerFunc(s.postPage)))

	mux.Handle("GET /blog/posts/form-create", middleware.RequireAuth(s.Auth, s.log)(http.HandlerFunc(s.createPostPage)))
	mux.Handle("POST /blog/posts", middleware.RequireAuth(s.Auth, s.log)(http.HandlerFunc(s.createPost)))
	mux.Handle("GET /blog/posts/form-update", middleware.RequireAuth(s.Auth, s.log)(http.HandlerFunc(s.updatePostPage)))
	mux.Handle("PUT /blog/posts", middleware.RequireAuth(s.Auth, s.log)(http.HandlerFunc(s.updatePost)))
	mux.Handle("DELETE /blog/posts/{id}", middleware.RequireAuth(s.Auth, s.log)(http.HandlerFunc(s.deletePost)))
	mux.Handle("PATCH /blog/posts/{id}/toggle-publication",
		middleware.RequireAuth(s.Auth, s.log)(http.HandlerFunc(s.togglePostPublication)))
}

func (s *Server) postsPage(w http.ResponseWriter, r *http.Request) {
	isAdmin := r.Context().Value(middleware.IsAdminKey) != nil

	categoryIDStr := r.URL.Query().Get("category_id")

	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		categoryID = 0
	}

	params := domain.SelectPostsParams{Limit: s.pageLimit + 1, Offset: 0, IsAdmin: isAdmin, CategoryID: categoryID}

	posts, err := s.Blog.Posts(r.Context(), params)
	if err != nil {
		s.log.Error("can't get posts from db", slog.Any("error", err))

		http.Error(w, "can't get posts from db", http.StatusInternalServerError)

		return
	}

	categories, err := s.Blog.Categories(r.Context())
	if err != nil {
		s.renderError(w, "can't get categories", err, http.StatusInternalServerError)

		return
	}

	tmplData := PostsPageData{
		Categories: categories,
		PostsData: PostsData{
			SelectedCategoryID: categoryID,
			Posts:              posts,
			IsAdmin:            isAdmin,
			HasNextPages:       false,
			NextOffset:         len(posts),
		},
	}

	if len(posts) == s.pageLimit+1 {
		tmplData.HasNextPages = true
		tmplData.Posts = tmplData.Posts[:len(tmplData.Posts)-1]
		tmplData.NextOffset = len(tmplData.Posts)
	}

	s.renderTemplate(w, "posts.html", tmplData)
}

func (s *Server) posts(w http.ResponseWriter, r *http.Request) {
	isAdmin := r.Context().Value(middleware.IsAdminKey) != nil

	offsetStr := r.URL.Query().Get("offset")

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		s.log.Error("can't convert offset to int", slog.Any("error", err))

		offset = 0
	}

	categoryIDStr := r.URL.Query().Get("category_id")

	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		categoryID = 0
	}

	params := domain.SelectPostsParams{Limit: s.pageLimit + 1, Offset: offset, IsAdmin: isAdmin, CategoryID: categoryID}

	posts, err := s.Blog.Posts(r.Context(), params)
	if err != nil {
		s.log.Error("can't get posts from db", slog.Any("error", err))

		http.Error(w, "can't get posts from db", http.StatusInternalServerError)

		return
	}

	tmplData := PostsData{
		SelectedCategoryID: categoryID,
		Posts:              posts,
		IsAdmin:            isAdmin,
		HasNextPages:       false,
		NextOffset:         offset + len(posts),
	}

	if len(posts) == s.pageLimit+1 {
		tmplData.HasNextPages = true
		tmplData.Posts = tmplData.Posts[:len(tmplData.Posts)-1]
		tmplData.NextOffset = offset + len(tmplData.Posts)
	}

	s.renderTemplate(w, "pagination-posts", tmplData)
}

func (s *Server) postPage(w http.ResponseWriter, r *http.Request) {
	isAdmin := r.Context().Value(middleware.IsAdminKey) != nil

	slug := r.PathValue("slug")

	post, err := s.Blog.PostBySlug(r.Context(), slug)
	if err != nil {
		s.log.Error("can't process service post method", slog.Any("error", err))

		http.Error(w, "can't find post by slug", http.StatusBadRequest)

		return
	}

	if !isAdmin && !post.IsPublished {
		s.log.Warn("user find hidden post")

		http.Error(w, "can't find post by slug", http.StatusNotFound)

		return
	}

	content := s.Blog.MdToHTML(post.Content)

	// #nosec G203 - Content is from trusted markdown stored in database
	tmplContent := template.HTML(content)

	tmplData := struct {
		ID           int
		Title        string
		Description  string
		Content      template.HTML
		Slug         string
		CategoryName string
		CreatedAt    string
		UpdatedAt    string
		IsPublished  bool
		IsAdmin      bool
	}{
		ID:           post.ID,
		Title:        post.Title,
		Description:  post.Description,
		Content:      tmplContent,
		Slug:         post.Slug,
		CategoryName: post.CategoryName,
		CreatedAt:    post.CreatedAt.Format("02.01.2006"),
		UpdatedAt:    post.UpdatedAt.Format("02.01.2006"),
		IsPublished:  post.IsPublished,
		IsAdmin:      isAdmin,
	}

	w.WriteHeader(http.StatusOK)

	s.renderTemplate(w, "post.html", tmplData)
}

func (s *Server) createPostPage(w http.ResponseWriter, r *http.Request) {
	categories, err := s.Blog.Categories(r.Context())
	if err != nil {
		s.renderError(w, "can't get categories", err, http.StatusInternalServerError)

		return
	}

	tmplData := struct {
		Categories []*domain.Category
	}{
		Categories: categories,
	}

	s.renderTemplate(w, "create_post.html", tmplData)
}

func (s *Server) updatePostPage(w http.ResponseWriter, r *http.Request) {
	postIDStr := r.URL.Query().Get("post_id")

	postID, err := strconv.Atoi(postIDStr)
	if err != nil {
		s.renderError(w, "invalid post id", err, http.StatusBadRequest)

		return
	}

	post, err := s.Blog.Post(r.Context(), postID)
	if err != nil {
		s.renderError(w, "can't get post", err, http.StatusInternalServerError)

		return
	}

	categories, err := s.Blog.Categories(r.Context())
	if err != nil {
		s.renderError(w, "can't get categories", err, http.StatusInternalServerError)

		return
	}

	tmplData := struct {
		Categories []*domain.Category
		Post       *domain.Post
	}{
		Categories: categories,
		Post:       post,
	}

	s.renderTemplate(w, "update_post.html", tmplData)
}

func (s *Server) updatePost(w http.ResponseWriter, r *http.Request) {
	postIDStr := r.URL.Query().Get("post_id")

	postID, err := strconv.Atoi(postIDStr)
	if err != nil {
		s.renderError(w, "invalid post id", err, http.StatusBadRequest)

		return
	}

	post, err := s.Blog.Post(r.Context(), postID)
	if err != nil {
		s.renderError(w, "can't get post", err, http.StatusBadRequest)

		return
	}

	const maxRequestSize = 1_000_000 // 1MB
	if err = r.ParseMultipartForm(maxRequestSize); err != nil {
		s.renderError(w, "can't parse file", err, http.StatusBadRequest)

		return
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")
	description := r.FormValue("description")
	categoryIDStr := r.FormValue("category_id")

	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		s.renderError(w, "invalid category id", err, http.StatusBadRequest)

		return
	}

	var content []byte

	file, _, err := r.FormFile("file")
	if err == nil {
		defer file.Close()

		content, err = io.ReadAll(file)
		if err != nil {
			s.renderError(w, "can't read file", err, http.StatusInternalServerError)

			return
		}
	} else {
		content = post.Content
	}

	postParms := domain.UpdatePostParams{
		ID:          post.ID,
		Title:       title,
		Slug:        slug,
		Description: description,
		CategoryID:  categoryID,
		Content:     content,
	}

	err = s.Blog.UpdatePost(r.Context(), postParms)
	if err != nil {
		s.renderError(w, "can't update post", err, http.StatusInternalServerError)

		return
	}

	w.Header().Set("HX-Redirect", "/blog/posts/"+slug)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	// 1MB
	const maxRequestSize = 1_000_000

	if err := r.ParseMultipartForm(maxRequestSize); err != nil {
		s.renderError(w, "can't parse file", err, http.StatusBadRequest)

		return
	}

	title := r.FormValue("title")
	slug := r.FormValue("slug")
	description := r.FormValue("description")
	categoryIDStr := r.FormValue("category_id")

	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		s.renderError(w, "invalid category id", err, http.StatusBadRequest)

		return
	}

	s.log.Debug("create post handler", slog.String("title", title), slog.String("desc", description))

	file, header, err := r.FormFile("file")
	if err != nil {
		s.renderError(w, "can't get file", err, http.StatusBadRequest)

		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		s.renderError(w, "can't read file", err, http.StatusInternalServerError)

		return
	}

	postParms := domain.CreatePostParams{
		Title:       title,
		Slug:        slug,
		Description: description,
		Filename:    header.Filename,
		CategoryID:  categoryID,
		Content:     content,
		IsPublished: false,
	}

	post, err := s.Blog.CreatePost(r.Context(), postParms)
	if err != nil {
		s.renderError(w, "can't create post", err, http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/blog/posts/"+post.Slug, http.StatusFound)
}

func (s *Server) deletePost(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	postID, err := strconv.Atoi(idStr)
	if err != nil {
		s.renderError(w, "invalid post id", err, http.StatusBadRequest)

		return
	}

	err = s.Blog.DeletePost(r.Context(), postID)
	if err != nil {
		s.renderError(w, "can't delete post", err, http.StatusInternalServerError)

		return
	}

	w.Header().Set("HX-Redirect", "/blog/posts")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) togglePostPublication(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	postID, err := strconv.Atoi(idStr)
	if err != nil {
		s.renderError(w, "invalid post id", err, http.StatusBadRequest)

		return
	}

	curPublishStatusStr := r.URL.Query().Get("is_published")
	slug := r.URL.Query().Get("slug")

	curPublishStatus, err := strconv.ParseBool(curPublishStatusStr)
	if err != nil {
		s.renderError(w, "invalid publish status", err, http.StatusBadRequest)

		return
	}

	err = s.Blog.ChangePublishStatus(r.Context(), postID, curPublishStatus)
	if err != nil {
		s.renderError(w, "can't change publish status", err, http.StatusInternalServerError)

		return
	}

	w.Header().Set("HX-Redirect", "/blog/posts/"+slug)
	w.WriteHeader(http.StatusOK)
}
