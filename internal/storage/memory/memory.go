package memory

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/ButyrinIA/system/internal/models"
)

// MemoryStorage представляет in-memory хранилище
type MemoryStorage struct {
	posts    map[string]*models.Post
	comments map[string][]*models.Comment
	mu       sync.RWMutex
}

// New создаёт новое in-memory хранилище
func New() *MemoryStorage {
	log.Println("Инициализация нового MemoryStorage")
	return &MemoryStorage{
		posts:    make(map[string]*models.Post),
		comments: make(map[string][]*models.Comment),
	}
}

// CreatePost создаёт новый пост
func (s *MemoryStorage) CreatePost(ctx context.Context, post *models.Post) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Printf("Вставка поста в Memory: ID=%s, Title=%s, CreatedAt=%v", post.ID, post.Title, post.CreatedAt)
	s.posts[post.ID] = post
	log.Printf("Пост успешно вставлен в Memory: %s", post.ID)
	return nil
}

// GetPost получает пост по ID
func (s *MemoryStorage) GetPost(ctx context.Context, id string) (*models.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	log.Printf("Получение поста с ID=%s из Memory", id)
	post, exists := s.posts[id]
	if !exists {
		log.Printf("Пост с ID=%s не найден в Memory", id)
		return nil, errors.New("post not found")
	}
	log.Printf("Пост успешно получен из Memory: ID=%s, Title=%s", post.ID, post.Title)
	return post, nil
}

// ListPosts возвращает список постов
func (s *MemoryStorage) ListPosts(ctx context.Context, limit int, cursor *string) (*models.PaginatedPosts, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	log.Printf("Запрос списка постов из Memory: limit=%d, cursor=%v", limit, cursor)

	var posts []*models.Post
	for _, post := range s.posts {
		posts = append(posts, post)
	}

	// Сортировка по createdAt (от новых к старым)
	for i := 0; i < len(posts)-1; i++ {
		for j := i + 1; j < len(posts); j++ {
			if posts[i].CreatedAt.Before(posts[j].CreatedAt) {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}

	totalCount := len(posts)
	log.Printf("Общее количество постов в Memory: %d", totalCount)

	startIdx := 0
	if cursor != nil {
		for i, post := range posts {
			if post.CreatedAt.String() == *cursor {
				startIdx = i + 1
				break
			}
		}
		log.Printf("Курсор применён, startIdx=%d", startIdx)
	}

	endIdx := startIdx + limit
	if endIdx > len(posts) {
		endIdx = len(posts)
	}
	log.Printf("Возвращено постов: %d", len(posts[startIdx:endIdx]))

	result := posts[startIdx:endIdx]
	var nextCursor *string
	if endIdx < len(posts) {
		cursorVal := posts[endIdx-1].CreatedAt.String()
		nextCursor = &cursorVal
		log.Printf("Установлен nextCursor: %s", *nextCursor)
	}

	return &models.PaginatedPosts{
		Posts:      result,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil
}

// CreateComment создаёт новый комментарий
func (s *MemoryStorage) CreateComment(ctx context.Context, comment *models.Comment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Printf("Вставка комментария в Memory: ID=%s, PostID=%s, Content=%s", comment.ID, comment.PostID, comment.Content)
	if _, exists := s.posts[comment.PostID]; !exists {
		log.Printf("Ошибка: пост с ID=%s не найден в Memory", comment.PostID)
		return errors.New("post not found")
	}
	s.comments[comment.PostID] = append(s.comments[comment.PostID], comment)
	log.Printf("Комментарий успешно вставлен в Memory: %s", comment.ID)
	return nil
}

// GetComments получает комментарии для поста
func (s *MemoryStorage) GetComments(ctx context.Context, postID string, parentID *string, limit int, cursor *string) (*models.PaginatedComments, error) {
	log.Printf("Запрос комментариев из Memory: postID=%s, parentID=%v, limit=%d, cursor=%v", postID, parentID, limit, cursor)
	s.mu.RLock()
	defer s.mu.RUnlock()

	comments, exists := s.comments[postID]
	if !exists {
		log.Printf("Комментарии для postID=%s не найдены в Memory", postID)
		return &models.PaginatedComments{Comments: []models.Comment{}, TotalCount: 0, NextCursor: nil}, nil
	}

	// Фильтрация по parentID
	var filtered []models.Comment
	for _, comment := range comments {
		if parentID == nil && comment.ParentID == nil || (parentID != nil && comment.ParentID != nil && *comment.ParentID == *parentID) {
			filtered = append(filtered, *comment)
			log.Printf("Добавлен комментарий: ID=%s, Content=%s", comment.ID, comment.Content)
		}
	}

	// Сортировка по createdAt (от новых к старым)
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[i].CreatedAt.Before(filtered[j].CreatedAt) {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	totalCount := len(filtered)
	log.Printf("Общее количество комментариев для postID=%s: %d", postID, totalCount)

	startIdx := 0
	if cursor != nil {
		for i, comment := range filtered {
			if comment.CreatedAt.String() == *cursor {
				startIdx = i + 1
				break
			}
		}
		log.Printf("Курсор применён, startIdx=%d", startIdx)
	}

	endIdx := startIdx + limit
	if endIdx > len(filtered) {
		endIdx = len(filtered)
	}
	log.Printf("Возвращено комментариев: %d", len(filtered[startIdx:endIdx]))

	result := filtered[startIdx:endIdx]
	var nextCursor *string
	if endIdx < len(filtered) {
		cursorVal := filtered[endIdx-1].CreatedAt.String()
		nextCursor = &cursorVal
		log.Printf("Установлен nextCursor: %s", *nextCursor)
	}

	return &models.PaginatedComments{
		Comments:   result,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil
}

// Close очищает in-memory хранилище
func (s *MemoryStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Println("Закрытие MemoryStorage")
	s.posts = make(map[string]*models.Post)
	s.comments = make(map[string][]*models.Comment)
	log.Println("MemoryStorage успешно очищено")
	return nil
}
