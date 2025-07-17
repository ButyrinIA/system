package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ButyrinIA/system/internal/models"
	"github.com/jackc/pgx/v5"
)

type PostgresStorage struct {
	conn *pgx.Conn
}

func New(dsn string) (*PostgresStorage, error) {
	log.Printf("Подключение к PostgreSQL с DSN: %s", dsn)
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		log.Printf("Ошибка подключения к PostgreSQL: %v", err)
		return nil, fmt.Errorf("failed to connect to postgres: %v", err)
	}

	log.Println("Создание таблиц posts и comments")
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
		log.Printf("Ошибка создания таблиц: %v", err)
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}
	log.Println("Таблицы успешно созданы или уже существуют")
	return &PostgresStorage{conn: conn}, nil
}

func (s *PostgresStorage) CreatePost(ctx context.Context, post *models.Post) error {
	log.Printf("Вставка поста: ID=%s, Title=%s, CreatedAt=%s", post.ID, post.Title, post.CreatedAt)
	_, err := s.conn.Exec(ctx, `
        INSERT INTO posts (id, title, content, author_id, allow_comments, created_at)
        VALUES ($1, $2, $3, $4, $5, $6)`,
		post.ID, post.Title, post.Content, post.AuthorID, post.AllowComments, post.CreatedAt)
	if err != nil {
		log.Printf("Ошибка при вставке поста ID=%s: %v", post.ID, err)
		return fmt.Errorf("failed to insert post: %v", err)
	}
	log.Printf("Пост успешно вставлен: %s", post.ID)
	return nil
}

func (s *PostgresStorage) GetPost(ctx context.Context, id string) (*models.Post, error) {
	log.Printf("Получение поста с ID=%s", id)
	var p models.Post
	err := s.conn.QueryRow(ctx, `
		SELECT id, title, content, author_id, allow_comments, created_at
		FROM posts
		WHERE id=$1`, id).Scan(&p.ID, &p.Title, &p.Content, &p.AuthorID, &p.AllowComments, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		log.Printf("Пост с ID=%s не найден", id)
		return nil, errors.New("post not found")
	}
	if err != nil {
		log.Printf("Ошибка при получении поста ID=%s: %v", id, err)
		return nil, fmt.Errorf("failed to get post: %v", err)
	}
	log.Printf("Пост успешно получен: ID=%s, Title=%s", p.ID, p.Title)
	return &p, nil
}

func (s *PostgresStorage) ListPosts(ctx context.Context, limit int, cursor *string) (*models.PaginatedPosts, error) {
	log.Printf("Запрос списка постов: limit=%d, cursor=%v", limit, cursor)
	// Подсчет общего количества
	var totalCount int
	err := s.conn.QueryRow(ctx, `SELECT COUNT(*) FROM posts`).Scan(&totalCount)
	if err != nil {
		log.Printf("Ошибка при подсчёте постов: %v", err)
		return nil, fmt.Errorf("failed to count posts: %v", err)
	}
	log.Printf("Общее количество постов: %d", totalCount)

	query := `
		SELECT id, title, content, author_id, allow_comments, created_at
		FROM posts
		WHERE ($1::TIMESTAMP IS NULL OR created_at < $1)
		ORDER BY created_at DESC
		LIMIT $2`
	rows, err := s.conn.Query(ctx, query, cursor, limit+1)
	if err != nil {
		log.Printf("Ошибка при запросе постов: %v", err)
		return nil, fmt.Errorf("failed to query posts: %v", err)
	}
	defer rows.Close()

	var posts []*models.Post // Changed from []models.Post to []*models.Post
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.AuthorID, &p.AllowComments, &p.CreatedAt); err != nil {
			log.Printf("Ошибка при сканировании поста: %v", err)
			return nil, fmt.Errorf("failed to scan post: %v", err)
		}
		posts = append(posts, &p) // Append pointer to p
		log.Printf("Получен пост: ID=%s, Title=%s", p.ID, p.Title)
	}

	var nextCursor *string
	if len(posts) > limit {
		nextCursor = new(string)
		*nextCursor = posts[limit-1].CreatedAt.String()
		posts = posts[:limit]
		log.Printf("Установлен nextCursor: %s", *nextCursor)
	}
	log.Printf("Возвращено постов: %d", len(posts))

	return &models.PaginatedPosts{
		Posts:      posts,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil
}

func (s *PostgresStorage) CreateComment(ctx context.Context, comment *models.Comment) error {
	log.Printf("Вставка комментария: ID=%s, PostID=%s, Content=%s", comment.ID, comment.PostID, comment.Content)
	_, err := s.conn.Exec(ctx, `
		INSERT INTO comments (id, post_id, parent_id, author_id, content, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		comment.ID, comment.PostID, comment.ParentID, comment.AuthorID, comment.Content, comment.CreatedAt)
	if err != nil {
		log.Printf("Ошибка при вставке комментария ID=%s: %v", comment.ID, err)
		return fmt.Errorf("failed to insert comment: %v", err)
	}
	log.Printf("Комментарий успешно вставлен: %s", comment.ID)
	return nil
}

func (s *PostgresStorage) GetComments(ctx context.Context, postID string, parentID *string, limit int, cursor *string) (*models.PaginatedComments, error) {
	log.Printf("Запрос комментариев: postID=%s, parentID=%v, limit=%d, cursor=%v", postID, parentID, limit, cursor)
	var totalCount int
	countQuery := `
        SELECT COUNT(*)
        FROM comments
        WHERE post_id=$1 AND parent_id IS NOT DISTINCT FROM $2`
	err := s.conn.QueryRow(ctx, countQuery, postID, parentID).Scan(&totalCount)
	if err != nil {
		log.Printf("Ошибка при подсчёте комментариев для postID=%s: %v", postID, err)
		// Возвращаем пустой результат вместо ошибки
		return &models.PaginatedComments{
			Comments:   []models.Comment{},
			TotalCount: 0,
			NextCursor: nil,
		}, nil
	}
	log.Printf("Общее количество комментариев для postID=%s: %d", postID, totalCount)

	query := `
        SELECT id, post_id, parent_id, author_id, content, created_at
        FROM comments
        WHERE post_id=$1 AND parent_id IS NOT DISTINCT FROM $2
        AND ($3::TIMESTAMP IS NULL OR created_at < $3)
        ORDER BY created_at DESC
        LIMIT $4`
	rows, err := s.conn.Query(ctx, query, postID, parentID, cursor, limit+1)
	if err != nil {
		log.Printf("Ошибка при запросе комментариев для postID=%s: %v", postID, err)
		return &models.PaginatedComments{
			Comments:   []models.Comment{},
			TotalCount: totalCount,
			NextCursor: nil,
		}, nil
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var c models.Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.ParentID, &c.AuthorID, &c.Content, &c.CreatedAt); err != nil {
			log.Printf("Ошибка при сканировании комментария: %v", err)
			return &models.PaginatedComments{
				Comments:   []models.Comment{},
				TotalCount: totalCount,
				NextCursor: nil,
			}, nil
		}
		comments = append(comments, c)
		log.Printf("Получен комментарий: ID=%s, Content=%s", c.ID, c.Content)
	}

	var nextCursor *string
	if len(comments) > limit {
		nextCursor = new(string)
		*nextCursor = comments[limit-1].CreatedAt.Format(time.RFC3339)
		comments = comments[:limit]
		log.Printf("Установлен nextCursor: %s", *nextCursor)
	}
	log.Printf("Возвращено комментариев: %d", len(comments))

	return &models.PaginatedComments{
		Comments:   comments,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil
}

func (s *PostgresStorage) Close() error {
	log.Println("Закрытие соединения с PostgreSQL")
	err := s.conn.Close(context.Background())
	if err != nil {
		log.Printf("Ошибка при закрытии соединения: %v", err)
		return fmt.Errorf("failed to close connection: %v", err)
	}
	log.Println("Соединение с PostgreSQL успешно закрыто")
	return nil
}
