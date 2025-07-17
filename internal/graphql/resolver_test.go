package graphql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ButyrinIA/system/internal/models"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// мок для интерфейса storage.Storage
type mockStorage struct {
	mock.Mock
}

func (m *mockStorage) ListPosts(ctx context.Context, limit int, cursor *string) (*models.PaginatedPosts, error) {
	args := m.Called(ctx, limit, cursor)
	return args.Get(0).(*models.PaginatedPosts), args.Error(1)
}

func (m *mockStorage) GetPost(ctx context.Context, id string) (*models.Post, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models.Post), args.Error(1)
}

func (m *mockStorage) CreatePost(ctx context.Context, post *models.Post) error {
	args := m.Called(ctx, post)
	return args.Error(0)
}

func (m *mockStorage) CreateComment(ctx context.Context, comment *models.Comment) error {
	args := m.Called(ctx, comment)
	return args.Error(0)
}

func (m *mockStorage) GetComments(ctx context.Context, postID string, parentID *string, limit int, cursor *string) (*models.PaginatedComments, error) {
	args := m.Called(ctx, postID, parentID, limit, cursor)
	return args.Get(0).(*models.PaginatedComments), args.Error(1)
}

func (m *mockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestPosts(t *testing.T) {
	storage := &mockStorage{}
	createdAt := time.Now()
	posts := &models.PaginatedPosts{
		Posts: []*models.Post{
			{
				ID:            "post1",
				Title:         "Тестовый пост",
				Content:       "Содержимое",
				AuthorID:      "user1",
				AllowComments: true,
				CreatedAt:     createdAt,
			},
		},
		TotalCount: 1,
		NextCursor: nil,
	}
	storage.On("ListPosts", mock.Anything, 10, (*string)(nil)).Return(posts, nil)

	resolver := NewResolver(storage, nil)
	query := resolver.Query()

	result, err := query.Posts(context.Background(), 10, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.TotalCount)
	assert.Len(t, result.Posts, 1)
	assert.Equal(t, "post1", result.Posts[0].ID)
	assert.Equal(t, "Тестовый пост", result.Posts[0].Title)
	assert.Equal(t, createdAt.Format(time.RFC3339), result.Posts[0].CreatedAt)
	storage.AssertExpectations(t)
}

func TestPosts_Error(t *testing.T) {
	storage := &mockStorage{}
	storage.On("ListPosts", mock.Anything, 10, (*string)(nil)).Return((*models.PaginatedPosts)(nil), errors.New("ошибка хранилища"))

	resolver := NewResolver(storage, nil)
	query := resolver.Query()

	result, err := query.Posts(context.Background(), 10, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "failed to list posts: ошибка хранилища", err.Error())
	storage.AssertExpectations(t)
}

func TestPost(t *testing.T) {
	storage := &mockStorage{}
	createdAt := time.Now()
	post := &models.Post{
		ID:            "post1",
		Title:         "Тестовый пост",
		Content:       "Содержимое",
		AuthorID:      "user1",
		AllowComments: true,
		CreatedAt:     createdAt,
	}
	storage.On("GetPost", mock.Anything, "post1").Return(post, nil)

	resolver := NewResolver(storage, nil)
	query := resolver.Query()

	result, err := query.Post(context.Background(), "post1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "post1", result.ID)
	assert.Equal(t, "Тестовый пост", result.Title)
	assert.Equal(t, createdAt.Format(time.RFC3339), result.CreatedAt)
	storage.AssertExpectations(t)
}

func TestPost_Error(t *testing.T) {
	storage := &mockStorage{}
	storage.On("GetPost", mock.Anything, "post1").Return((*models.Post)(nil), errors.New("пост не найден"))

	resolver := NewResolver(storage, nil)
	query := resolver.Query()

	result, err := query.Post(context.Background(), "post1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "failed to get post: пост не найден", err.Error())
	storage.AssertExpectations(t)
}

func TestComments(t *testing.T) {
	storage := &mockStorage{}
	createdAt := time.Now()
	commentLoader := dataloader.NewBatchedLoader(
		func(ctx context.Context, keys []string) []*dataloader.Result[*models.PaginatedComments] {
			results := make([]*dataloader.Result[*models.PaginatedComments], len(keys))
			for i, key := range keys {
				comments := &models.PaginatedComments{
					Comments: []models.Comment{
						{
							ID:        "comment1",
							PostID:    key,
							AuthorID:  "user1",
							Content:   "Тестовый комментарий",
							CreatedAt: createdAt,
						},
					},
					TotalCount: 1,
					NextCursor: nil,
				}
				results[i] = &dataloader.Result[*models.PaginatedComments]{Data: comments}
			}
			return results
		},
	)
	ctx := context.WithValue(context.Background(), "commentLoader", commentLoader)
	resolver := NewResolver(storage, commentLoader)
	postResolver := resolver.Post()

	post := &Post{ID: "post1"}
	result, err := postResolver.Comments(ctx, post, 10, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.TotalCount)
	assert.Len(t, result.Comments, 1)
	assert.Equal(t, "comment1", result.Comments[0].ID)
	assert.Equal(t, createdAt.Format(time.RFC3339), result.Comments[0].CreatedAt)
}

func TestComments_NoLoader(t *testing.T) {
	storage := &mockStorage{}
	resolver := NewResolver(storage, nil)
	postResolver := resolver.Post()

	result, err := postResolver.Comments(context.Background(), &Post{ID: "post1"}, 10, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "commentLoader not found in context", err.Error())
}

func TestReplies(t *testing.T) {
	storage := &mockStorage{}
	createdAt := time.Now()
	comments := &models.PaginatedComments{
		Comments: []models.Comment{
			{
				ID:        "comment2",
				PostID:    "post1",
				ParentID:  stringPtr("comment1"),
				AuthorID:  "user1",
				Content:   "Ответ",
				CreatedAt: createdAt,
			},
		},
		TotalCount: 1,
		NextCursor: nil,
	}
	storage.On("GetComments", mock.Anything, "post1", stringPtr("comment1"), 10, (*string)(nil)).Return(comments, nil)

	resolver := NewResolver(storage, nil)
	commentResolver := resolver.Comment()

	comment := &Comment{ID: "comment1", PostID: "post1"}
	result, err := commentResolver.Replies(context.Background(), comment, 10, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.TotalCount)
	assert.Len(t, result.Comments, 1)
	assert.Equal(t, "comment2", result.Comments[0].ID)
	assert.Equal(t, createdAt.Format(time.RFC3339), result.Comments[0].CreatedAt)
	storage.AssertExpectations(t)
}

func TestReplies_Error(t *testing.T) {
	storage := &mockStorage{}
	storage.On("GetComments", mock.Anything, "post1", stringPtr("comment1"), 10, (*string)(nil)).Return((*models.PaginatedComments)(nil), errors.New("ошибка хранилища"))

	resolver := NewResolver(storage, nil)
	commentResolver := resolver.Comment()

	comment := &Comment{ID: "comment1", PostID: "post1"}
	result, err := commentResolver.Replies(context.Background(), comment, 10, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "failed to load comment replies: ошибка хранилища", err.Error())
	storage.AssertExpectations(t)
}

func TestCreatePost(t *testing.T) {
	storage := &mockStorage{}
	storage.On("CreatePost", mock.Anything, mock.AnythingOfType("*models.Post")).Return(nil)

	resolver := NewResolver(storage, nil)
	mutation := resolver.Mutation()
	ctx := context.WithValue(context.Background(), "userID", "user1")

	result, err := mutation.CreatePost(ctx, "Тестовый пост", "Содержимое", true)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Тестовый пост", result.Title)
	assert.Equal(t, "user1", result.AuthorID)
	storage.AssertExpectations(t)
}

func TestCreatePost_ValidationError(t *testing.T) {
	storage := &mockStorage{}
	resolver := NewResolver(storage, nil)
	mutation := resolver.Mutation()

	// Слишком длинный заголовок
	result, err := mutation.CreatePost(context.Background(), string(make([]byte, 201)), "Содержимое", true)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "title exceeds 200 characters", err.Error())
}

func TestCreateComment(t *testing.T) {
	storage := &mockStorage{}
	post := &models.Post{
		ID:            "post1",
		AllowComments: true,
	}
	storage.On("GetPost", mock.Anything, "post1").Return(post, nil)
	storage.On("CreateComment", mock.Anything, mock.AnythingOfType("*models.Comment")).Return(nil)

	resolver := NewResolver(storage, nil)
	mutation := resolver.Mutation()
	ctx := context.WithValue(context.Background(), "userID", "user1")

	result, err := mutation.CreateComment(ctx, "post1", nil, "Тестовый комментарий")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "post1", result.PostID)
	assert.Equal(t, "Тестовый комментарий", result.Content)
	storage.AssertExpectations(t)
}

func TestCreateComment_CommentsDisabled(t *testing.T) {
	storage := &mockStorage{}
	post := &models.Post{
		ID:            "post1",
		AllowComments: false,
	}
	storage.On("GetPost", mock.Anything, "post1").Return(post, nil)

	resolver := NewResolver(storage, nil)
	mutation := resolver.Mutation()

	result, err := mutation.CreateComment(context.Background(), "post1", nil, "Тестовый комментарий")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "comments are disabled for this post", err.Error())
	storage.AssertExpectations(t)
}

func TestCommentAdded(t *testing.T) {
	resolver := NewResolver(nil, nil)
	subscription := resolver.Subscription()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	postID := "post1"
	ch, err := subscription.CommentAdded(ctx, postID)
	assert.NoError(t, err)
	assert.NotNil(t, ch)

	comment := &Comment{ID: "comment1", PostID: postID, Content: "Тестовый комментарий"}
	resolver.SubscriptionHandler.mu.Lock()
	resolver.SubscriptionHandler.commentChannels[postID] = append(resolver.SubscriptionHandler.commentChannels[postID])
	resolver.SubscriptionHandler.mu.Unlock()

	go func() {
		resolver.SubscriptionHandler.mu.Lock()
		for _, c := range resolver.SubscriptionHandler.commentChannels[postID] {
			c <- comment
		}
		resolver.SubscriptionHandler.mu.Unlock()
	}()

	select {
	case received := <-ch:
		assert.Equal(t, comment.ID, received.ID)
	case <-time.After(time.Second):
		t.Fatal("Таймаут ожидания подписки")
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
	_, open := <-ch
	assert.False(t, open, "Канал должен быть закрыт")
}

func stringPtr(s string) *string {
	return &s
}
