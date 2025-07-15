package graphql

// THIS CODE WILL BE UPDATED WITH SCHEMA CHANGES. PREVIOUS IMPLEMENTATION FOR SCHEMA CHANGES WILL BE KEPT IN THE COMMENT SECTION. IMPLEMENTATION FOR UNCHANGED SCHEMA WILL BE KEPT.

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ButyrinIA/system/internal/models"
	"github.com/ButyrinIA/system/internal/storage"
	"github.com/google/uuid"
)

// Resolver - основная структура, реализующая ResolverRoot
type Resolver struct {
	Storage storage.Storage
}

// queryResolver реализует QueryResolver
type queryResolver struct {
	Storage storage.Storage
}

// mutationResolver реализует MutationResolver
type mutationResolver struct {
	Storage             storage.Storage
	SubscriptionHandler *subscriptionHandler
}

// subscriptionHandler реализует SubscriptionResolver
type subscriptionHandler struct {
	commentChannels map[string]chan *Comment
	mu              sync.RWMutex
}

// NewResolver создает новый Resolver
func NewResolver(storage storage.Storage) *Resolver {
	return &Resolver{Storage: storage}
}

// Query возвращает QueryResolver
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{Storage: r.Storage}
}

// Mutation возвращает MutationResolver
func (r *Resolver) Mutation() MutationResolver {
	return &mutationResolver{
		Storage:             r.Storage,
		SubscriptionHandler: newSubscriptionHandler(),
	}
}

// Subscription возвращает SubscriptionResolver
func (r *Resolver) Subscription() SubscriptionResolver {
	return newSubscriptionHandler()
}

// newSubscriptionHandler создает новый subscriptionHandler
func newSubscriptionHandler() *subscriptionHandler {
	return &subscriptionHandler{
		commentChannels: make(map[string]chan *Comment),
	}
}

// Posts реализует запрос posts
func (r *queryResolver) Posts(ctx context.Context, limit int, cursor *string) (*PaginatedPosts, error) {
	posts, err := r.Storage.ListPosts(ctx, limit, cursor)
	if err != nil {
		return nil, err
	}

	// Конвертация internal/models.PaginatedPosts в graphql.PaginatedPosts
	result := &PaginatedPosts{
		TotalCount: posts.TotalCount,
		NextCursor: posts.NextCursor,
	}
	result.Posts = make([]*Post, len(posts.Posts))
	for i, p := range posts.Posts {
		result.Posts[i] = &Post{
			ID:            p.ID,
			Title:         p.Title,
			Content:       p.Content,
			AuthorID:      p.AuthorID,
			AllowComments: p.AllowComments,
			CreatedAt:     p.CreatedAt.Format(time.RFC3339),
		}
	}
	return result, nil
}

// Post реализует запрос post
func (r *queryResolver) Post(ctx context.Context, id string) (*Post, error) {
	post, err := r.Storage.GetPost(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Post{
		ID:            post.ID,
		Title:         post.Title,
		Content:       post.Content,
		AuthorID:      post.AuthorID,
		AllowComments: post.AllowComments,
		CreatedAt:     post.CreatedAt.Format(time.RFC3339),
	}, nil
}

// Post_comments реализует поле comments в Post
func (r *queryResolver) Post_comments(ctx context.Context, obj *Post, limit int, cursor *string) (*PaginatedComments, error) {
	comments, err := r.Storage.GetComments(ctx, obj.ID, nil, limit, cursor)
	if err != nil {
		return nil, err
	}

	// Конвертация internal/models.PaginatedComments в graphql.PaginatedComments
	result := &PaginatedComments{
		TotalCount: comments.TotalCount,
		NextCursor: comments.NextCursor,
	}
	result.Comments = make([]*Comment, len(comments.Comments))
	for i, c := range comments.Comments {
		result.Comments[i] = &Comment{
			ID:        c.ID,
			PostID:    c.PostID,
			ParentID:  c.ParentID,
			AuthorID:  c.AuthorID,
			Content:   c.Content,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
		}
	}
	return result, nil
}

// Comment_replies реализует поле replies в Comment
func (r *queryResolver) Comment_replies(ctx context.Context, obj *Comment, limit int, cursor *string) (*PaginatedComments, error) {
	comments, err := r.Storage.GetComments(ctx, obj.PostID, &obj.ID, limit, cursor)
	if err != nil {
		return nil, err
	}

	// Конвертация internal/models.PaginatedComments в graphql.PaginatedComments
	result := &PaginatedComments{
		TotalCount: comments.TotalCount,
		NextCursor: comments.NextCursor,
	}
	result.Comments = make([]*Comment, len(comments.Comments))
	for i, c := range comments.Comments {
		result.Comments[i] = &Comment{
			ID:        c.ID,
			PostID:    c.PostID,
			ParentID:  c.ParentID,
			AuthorID:  c.AuthorID,
			Content:   c.Content,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
		}
	}
	return result, nil
}

// CreatePost реализует мутацию createPost
func (r *mutationResolver) CreatePost(ctx context.Context, title string, content string, allowComments bool) (*Post, error) {
	if len(title) > 200 {
		return nil, errors.New("title exceeds 200 characters")
	}

	userID, ok := ctx.Value("userID").(string)
	if !ok {
		userID = "user1" // Mock user
	}

	post := &Post{
		ID:            uuid.New().String(),
		Title:         title,
		Content:       content,
		AuthorID:      userID,
		AllowComments: allowComments,
		CreatedAt:     time.Now().Format(time.RFC3339),
	}

	// Конвертация в internal/models.Post для сохранения в хранилище
	internalPost := &models.Post{
		ID:            post.ID,
		Title:         post.Title,
		Content:       post.Content,
		AuthorID:      post.AuthorID,
		AllowComments: post.AllowComments,
		CreatedAt:     time.Now(),
	}
	if err := r.Storage.CreatePost(ctx, internalPost); err != nil {
		return nil, err
	}
	return post, nil
}

// CreateComment реализует мутацию createComment
func (r *mutationResolver) CreateComment(ctx context.Context, postID string, parentID *string, content string) (*Comment, error) {
	if len(content) > 2000 {
		return nil, errors.New("comment content exceeds 2000 characters")
	}

	userID, ok := ctx.Value("userID").(string)
	if !ok {
		userID = "user1" // Mock user
	}

	post, err := r.Storage.GetPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if !post.AllowComments {
		return nil, errors.New("comments are disabled for this post")
	}

	comment := &Comment{
		ID:        uuid.New().String(),
		PostID:    postID,
		ParentID:  parentID,
		AuthorID:  userID,
		Content:   content,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	// Конвертация в internal/models.Comment для сохранения в хранилище
	internalComment := &models.Comment{
		ID:        comment.ID,
		PostID:    comment.PostID,
		ParentID:  comment.ParentID,
		AuthorID:  comment.AuthorID,
		Content:   comment.Content,
		CreatedAt: time.Now(),
	}
	if err := r.Storage.CreateComment(ctx, internalComment); err != nil {
		return nil, err
	}

	// Уведомление подписчиков
	r.SubscriptionHandler.mu.RLock()
	if ch, exists := r.SubscriptionHandler.commentChannels[postID]; exists {
		select {
		case ch <- comment:
		default:
		}
	}
	r.SubscriptionHandler.mu.RUnlock()

	return comment, nil
}

// CommentAdded реализует подписку commentAdded
func (s *subscriptionHandler) CommentAdded(ctx context.Context, postID string) (<-chan *Comment, error) {
	s.mu.Lock()
	if _, exists := s.commentChannels[postID]; !exists {
		s.commentChannels[postID] = make(chan *Comment, 1)
	}
	ch := s.commentChannels[postID]
	s.mu.Unlock()

	// Очистка канала после завершения подписки
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		if ch, exists := s.commentChannels[postID]; exists {
			close(ch)
			delete(s.commentChannels, postID)
		}
		s.mu.Unlock()
	}()

	return ch, nil
}
