package memory

import (
	"context"
	"errors"
	"sync"

	"github.com/ButyrinIA/system/internal/models"
)

type MemoryStorage struct {
	posts    map[string]*models.Post
	comments map[string][]*models.Comment
	mu       sync.RWMutex
}

func New() *MemoryStorage {
	return &MemoryStorage{
		posts:    make(map[string]*models.Post),
		comments: make(map[string][]*models.Comment),
	}
}

func (s *MemoryStorage) CreatePost(ctx context.Context, post *models.Post) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.posts[post.ID] = post
	return nil
}

func (s *MemoryStorage) GetPost(ctx context.Context, id string) (*models.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	post, exists := s.posts[id]
	if !exists {
		return nil, errors.New("пост не найден")
	}

	return post, nil
}

func (s *MemoryStorage) ListPosts(ctx context.Context, limit int, cursor *string) (*models.PaginatedPosts, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var posts []models.Post
	for _, post := range s.posts {
		posts = append(posts, *post)
	}

	for i := 0; i < len(posts)-1; i++ {
		for j := i + 1; j < len(posts); j++ {
			if posts[i].CreatedAt.Before(posts[j].CreatedAt) {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}

	totalCount := len(posts)

	// Применение курсора
	startIdx := 0
	if cursor != nil {
		for i, post := range posts {
			if post.CreatedAt.String() == *cursor {
				startIdx = i + 1
				break
			}
		}
	}

	// Ограничение количества
	endIdx := startIdx + limit
	if endIdx > len(posts) {
		endIdx = len(posts)
	}

	result := posts[startIdx:endIdx]
	var nextCursor *string
	if endIdx < len(posts) {
		cursorVal := posts[endIdx-1].CreatedAt.String()
		nextCursor = &cursorVal
	}

	return &models.PaginatedPosts{
		Posts:      result,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil

}

func (s *MemoryStorage) CreateComment(ctx context.Context, comment *models.Comment) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.comments[comment.PostID] = append(s.comments[comment.PostID], comment)

	return nil
}

func (s *MemoryStorage) GetComments(ctx context.Context, postID string, parentID *string, limit int, cursor *string) (*models.PaginatedComments, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	comments, exists := s.comments[postID]
	if !exists {
		return &models.PaginatedComments{Comments: nil, TotalCount: 0, NextCursor: nil}, nil
	}

	// Фильтрация по parentID
	var filtered []*models.Comment
	for _, comment := range comments {
		if parentID == nil && comment.ParentID == nil {
			filtered = append(filtered, comment)
		} else if parentID != nil && comment.ParentID != nil && *comment.ParentID == *parentID {
			filtered = append(filtered, comment)
		}
	}
	// Сортировка по CreatedAt
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[i].CreatedAt.Before(filtered[j].CreatedAt) {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	totalCount := len(filtered)

	// Применение курсора
	startIdx := 0
	if cursor != nil {
		for i, comment := range filtered {
			if comment.CreatedAt.String() == *cursor {
				startIdx = i + 1
				break
			}
		}
	}

	// Ограничение количества
	endIdx := startIdx + limit
	if endIdx > len(filtered) {
		endIdx = len(filtered)
	}

	// Копирование в []Comment
	result := make([]models.Comment, endIdx-startIdx)
	for i, comment := range filtered[startIdx:endIdx] {
		result[i] = *comment
	}

	var nextCursor *string
	if endIdx < len(filtered) {
		cursorVal := filtered[endIdx-1].CreatedAt.String()
		nextCursor = &cursorVal
	}

	return &models.PaginatedComments{
		Comments:   result,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil
}

func (s *MemoryStorage) Close() error {
	return nil
}
