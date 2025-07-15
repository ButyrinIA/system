package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ButyrinIA/system/internal/models"
	"github.com/jackc/pgx/v5"
)

type PostgresStorage struct {
	conn *pgx.Conn
}

func New(dsn string) (*PostgresStorage, error) {
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %v", err)
	}

	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS posts (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			author_id TEXT NOT NULL,
			allow_comments BOOLEAN NOT NULL,
			created_at TIMESTAMP NOT NULL
		);
		CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			post_id TEXT REFERENCES posts(id),
			parent_id TEXT,
			author_id TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_comments_post_id ON comments(post_id);
		CREATE INDEX IF NOT EXISTS idx_comments_parent_id ON comments(parent_id);
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return &PostgresStorage{conn: conn}, nil
}

func (s *PostgresStorage) CreatePost(ctx context.Context, post *models.Post) error {
	_, err := s.conn.Exec(ctx, `
		INSERT INTO posts (id, title, content, author_id, allow_comments, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		post.ID, post.Title, post.Content, post.AuthorID, post.AllowComments, post.CreatedAt)
	return err
}

func (s *PostgresStorage) GetPost(ctx context.Context, id string) (*models.Post, error) {
	var p models.Post
	err := s.conn.QueryRow(ctx, `
		SELECT id, title, content, author_id, allow_comments, created_at
		FROM posts
		WHERE id=$1`, id).Scan(&p.ID, &p.Title, &p.Content, &p.AuthorID, &p.AllowComments, &p.CreatedAt)

	if err == pgx.ErrNoRows {
		return nil, errors.New("post not found")
	}
	return &p, err
}

func (s *PostgresStorage) ListPosts(ctx context.Context, limit int, cursor *string) (*models.PaginatedPosts, error) {
	// Подсчет общего количества
	var totalCount int
	err := s.conn.QueryRow(ctx, `SELECT COUNT(*) FROM posts`).Scan(&totalCount)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, title, content, author_id, allow_comments, created_at
		FROM posts
		WHERE ($1::TIMESTAMP IS NULL OR created_at < $1)
		ORDER BY created_at DESC
		LIMIT $2`
	rows, err := s.conn.Query(ctx, query, cursor, limit+1)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.AuthorID, &p.AllowComments, &p.CreatedAt); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}

	var nextCursor *string
	if len(posts) > limit {
		nextCursor = new(string)
		*nextCursor = posts[limit-1].CreatedAt.String()
		posts = posts[:limit]
	}

	return &models.PaginatedPosts{
		Posts:      posts,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil
}

func (s *PostgresStorage) CreateComment(ctx context.Context, comment *models.Comment) error {
	_, err := s.conn.Exec(ctx, `
		INSERT INTO comments (id, post_id, parent_id, author_id, content, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		comment.ID, comment.PostID, comment.ParentID, comment.AuthorID, comment.Content, comment.CreatedAt)
	return err
}

func (s *PostgresStorage) GetComments(ctx context.Context, postID string, parentID *string, limit int, cursor *string) (*models.PaginatedComments, error) {
	// Подсчет общего количества
	var totalCount int
	countQuery := `
		SELECT COUNT(*)
		FROM comments
		WHERE post_id=$1 AND parent_id IS NOT DISTINCT FROM $2`
	err := s.conn.QueryRow(ctx, countQuery, postID, parentID).Scan(&totalCount)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, post_id, parent_id, author_id, content, created_at
		FROM comments
		WHERE post_id=$1 AND parent_id IS NOT DISTINCT FROM $2
		AND ($3::TIMESTAMP IS NULL OR created_at < $3)
		ORDER BY created_at DESC
		LIMIT $4`
	rows, err := s.conn.Query(ctx, query, postID, parentID, cursor, limit+1)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var c models.Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.ParentID, &c.AuthorID, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}

	var nextCursor *string
	if len(comments) > limit {
		nextCursor = new(string)
		*nextCursor = comments[limit-1].CreatedAt.String()
		comments = comments[:limit]
	}

	return &models.PaginatedComments{
		Comments:   comments,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil
}

func (s *PostgresStorage) Close() error {
	return s.conn.Close(context.Background())
}
