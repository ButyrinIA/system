package storage

import (
	"context"

	"github.com/ButyrinIA/system/internal/models"
)

type Storage interface {
	CreatePost(ctx context.Context, post *models.Post) error
	GetPost(ctx context.Context, id string) (*models.Post, error)
	ListPosts(ctx context.Context, limit int, cursor *string) (*models.PaginatedPosts, error)
	CreateComment(ctx context.Context, comment *models.Comment) error
	GetComments(ctx context.Context, postID string, parentID *string, limit int, cursor *string) (*models.PaginatedComments, error)
	Close() error
}
