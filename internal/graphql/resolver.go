package graphql

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ButyrinIA/system/internal/models"
	"github.com/ButyrinIA/system/internal/storage"
	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"
)

// PostResolver определяет резолверы для полей типа Post
type PostResolver interface {
	Comments(ctx context.Context, obj *Post, limit int, cursor *string) (*PaginatedComments, error)
}

// CommentResolver определяет резолверы для полей типа Comment
type CommentResolver interface {
	Replies(ctx context.Context, obj *Comment, limit int, cursor *string) (*PaginatedComments, error)
}

// Resolver - основная структура, реализующая ResolverRoot
type Resolver struct {
	Storage             storage.Storage
	SubscriptionHandler *subscriptionHandler
	CommentLoader       *dataloader.Loader[string, *models.PaginatedComments]
}

// queryResolver реализует QueryResolver
type queryResolver struct {
	*Resolver
}

// mutationResolver реализует MutationResolver
type mutationResolver struct {
	*Resolver
}

// postResolver реализует PostResolver
type postResolver struct {
	*Resolver
}

// commentResolver реализует CommentResolver
type commentResolver struct {
	*Resolver
}

// subscriptionHandler реализует SubscriptionResolver
type subscriptionHandler struct {
	commentChannels map[string][]chan *Comment
	mu              sync.RWMutex
}

// NewResolver создаёт новый Resolver
func NewResolver(storage storage.Storage, commentLoader *dataloader.Loader[string, *models.PaginatedComments]) *Resolver {
	log.Println("Создание нового Resolver")
	return &Resolver{
		Storage:             storage,
		SubscriptionHandler: newSubscriptionHandler(),
		CommentLoader:       commentLoader,
	}
}

// Query возвращает QueryResolver
func (r *Resolver) Query() QueryResolver {
	log.Println("Инициализация QueryResolver")
	return &queryResolver{r}
}

// Mutation возвращает MutationResolver
func (r *Resolver) Mutation() MutationResolver {
	log.Println("Инициализация MutationResolver")
	return &mutationResolver{r}
}

// Post возвращает PostResolver
func (r *Resolver) Post() PostResolver {
	log.Println("Инициализация PostResolver")
	return &postResolver{r}
}

// Comment возвращает CommentResolver
func (r *Resolver) Comment() CommentResolver {
	log.Println("Инициализация CommentResolver")
	return &commentResolver{r}
}

// Subscription возвращает SubscriptionResolver
func (r *Resolver) Subscription() SubscriptionResolver {
	log.Println("Инициализация SubscriptionResolver")
	return r.SubscriptionHandler
}

// newSubscriptionHandler создаёт новый subscriptionHandler
func newSubscriptionHandler() *subscriptionHandler {
	log.Println("Создание нового subscriptionHandler")
	return &subscriptionHandler{
		commentChannels: make(map[string][]chan *Comment),
	}
}

// Posts реализует запрос posts
func (r *queryResolver) Posts(ctx context.Context, limit int, cursor *string) (*PaginatedPosts, error) {
	log.Printf("Запрос posts с limit=%d, cursor=%v", limit, cursor)
	posts, err := r.Storage.ListPosts(ctx, limit, cursor)
	if err != nil {
		log.Printf("Ошибка при получении постов: %v", err)
		return nil, fmt.Errorf("failed to list posts: %v", err)
	}
	log.Printf("Получено постов: %d, TotalCount: %d, NextCursor: %v", len(posts.Posts), posts.TotalCount, posts.NextCursor)

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
		log.Printf("Конвертирован пост %d: ID=%s, Title=%s", i, p.ID, p.Title)
	}
	return result, nil
}

// Post реализует запрос post
func (r *queryResolver) Post(ctx context.Context, id string) (*Post, error) {
	log.Printf("Запрос post с ID=%s", id)
	post, err := r.Storage.GetPost(ctx, id)
	if err != nil {
		log.Printf("Ошибка при получении поста с ID=%s: %v", id, err)
		return nil, fmt.Errorf("failed to get post: %v", err)
	}
	log.Printf("Получен пост: ID=%s, Title=%s", post.ID, post.Title)
	return &Post{
		ID:            post.ID,
		Title:         post.Title,
		Content:       post.Content,
		AuthorID:      post.AuthorID,
		AllowComments: post.AllowComments,
		CreatedAt:     post.CreatedAt.Format(time.RFC3339),
	}, nil
}

// Comments реализует поле comments в Post с использованием DataLoader
func (r *postResolver) Comments(ctx context.Context, obj *Post, limit int, cursor *string) (*PaginatedComments, error) {
	log.Printf("Запрос комментариев для postID=%s, limit=%d, cursor=%v", obj.ID, limit, cursor)
	commentLoader, ok := ctx.Value("commentLoader").(*dataloader.Loader[string, *models.PaginatedComments])
	if !ok {
		log.Println("Ошибка: CommentLoader не найден в контексте")
		return nil, fmt.Errorf("commentLoader not found in context")
	}

	thunk := commentLoader.Load(ctx, obj.ID)
	result, err := thunk()
	if err != nil {
		log.Printf("Ошибка при загрузке комментариев для postID=%s через DataLoader: %v", obj.ID, err)
		return nil, fmt.Errorf("failed to load comments: %v", err)
	}

	log.Printf("Получено комментариев для postID=%s: %d, TotalCount: %d, NextCursor: %v", obj.ID, len(result.Comments), result.TotalCount, result.NextCursor)
	paginatedComments := &PaginatedComments{
		TotalCount: result.TotalCount,
		NextCursor: result.NextCursor,
	}
	paginatedComments.Comments = make([]*Comment, len(result.Comments))
	for i, c := range result.Comments {
		paginatedComments.Comments[i] = &Comment{
			ID:        c.ID,
			PostID:    c.PostID,
			ParentID:  c.ParentID,
			AuthorID:  c.AuthorID,
			Content:   c.Content,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
		}
		log.Printf("Конвертирован комментарий %d: ID=%s, Content=%s", i, c.ID, c.Content)
	}
	return paginatedComments, nil
}

// Replies реализует поле replies в Comment
func (r *commentResolver) Replies(ctx context.Context, obj *Comment, limit int, cursor *string) (*PaginatedComments, error) {
	log.Printf("Запрос ответов для commentID=%s, postID=%s, limit=%d, cursor=%v", obj.ID, obj.PostID, limit, cursor)
	comments, err := r.Storage.GetComments(ctx, obj.PostID, &obj.ID, limit, cursor)
	if err != nil {
		log.Printf("Ошибка при получении ответов для commentID=%s: %v", obj.ID, err)
		return nil, fmt.Errorf("failed to load comment replies: %v", err)
	}
	log.Printf("Получено ответов для commentID=%s: %d, TotalCount: %d, NextCursor: %v", obj.ID, len(comments.Comments), comments.TotalCount, comments.NextCursor)

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
		log.Printf("Конвертирован ответ %d: ID=%s, Content=%s", i, c.ID, c.Content)
	}
	return result, nil
}

// CreatePost реализует мутацию createPost
func (r *mutationResolver) CreatePost(ctx context.Context, title string, content string, allowComments bool) (*Post, error) {
	log.Printf("Запуск мутации createPost: title=%s, allowComments=%t", title, allowComments)
	if len(title) > 200 {
		log.Println("Ошибка: заголовок превышает 200 символов")
		return nil, errors.New("title exceeds 200 characters")
	}
	if len(content) > 2000 {
		log.Println("Ошибка: содержимое поста превышает 2000 символов")
		return nil, errors.New("content exceeds 2000 characters")
	}
	userID, ok := ctx.Value("userID").(string)
	if !ok {
		log.Println("userID не найден в контексте, используется user1")
		userID = "user1"
	}
	post := &Post{
		ID:            uuid.New().String(),
		Title:         title,
		Content:       content,
		AuthorID:      userID,
		AllowComments: allowComments,
		CreatedAt:     time.Now().Format(time.RFC3339),
	}
	internalPost := &models.Post{
		ID:            post.ID,
		Title:         post.Title,
		Content:       post.Content,
		AuthorID:      post.AuthorID,
		AllowComments: post.AllowComments,
		CreatedAt:     time.Now(),
	}
	log.Printf("Создание поста: %+v", internalPost)
	if err := r.Storage.CreatePost(ctx, internalPost); err != nil {
		log.Printf("Ошибка при создании поста: %v", err)
		return nil, fmt.Errorf("failed to create post: %v", err)
	}
	log.Printf("Пост успешно создан: %s", post.ID)
	return post, nil
}

// CreateComment реализует мутацию createComment
func (r *mutationResolver) CreateComment(ctx context.Context, postID string, parentID *string, content string) (*Comment, error) {
	log.Printf("Запуск мутации createComment: postID=%s, parentID=%v, content=%s", postID, parentID, content)
	if len(content) > 2000 {
		log.Println("Ошибка: содержимое комментария превышает 2000 символов")
		return nil, errors.New("comment content exceeds 2000 characters")
	}
	userID, ok := ctx.Value("userID").(string)
	if !ok {
		log.Println("userID не найден в контексте, используется user1")
		userID = "user1"
	}
	post, err := r.Storage.GetPost(ctx, postID)
	if err != nil {
		log.Printf("Ошибка при получении поста с ID=%s: %v", postID, err)
		return nil, fmt.Errorf("failed to get post: %v", err)
	}
	if !post.AllowComments {
		log.Printf("Ошибка: комментарии отключены для поста %s", postID)
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
	internalComment := &models.Comment{
		ID:        comment.ID,
		PostID:    comment.PostID,
		ParentID:  comment.ParentID,
		AuthorID:  comment.AuthorID,
		Content:   comment.Content,
		CreatedAt: time.Now(),
	}
	log.Printf("Создание комментария: %+v", internalComment)
	if err := r.Storage.CreateComment(ctx, internalComment); err != nil {
		log.Printf("Ошибка при создании комментария: %v", err)
		return nil, fmt.Errorf("failed to create comment: %v", err)
	}
	log.Printf("Комментарий успешно создан: %s", comment.ID)

	// Отправка уведомления подписчикам
	r.SubscriptionHandler.mu.Lock()
	channels, exists := r.SubscriptionHandler.commentChannels[postID]
	if exists {
		log.Printf("Отправка уведомления для postID=%s, количество каналов: %d", postID, len(channels))
		newChannels := make([]chan *Comment, 0, len(channels))
		for i, ch := range channels {
			select {
			case ch <- comment:
				log.Printf("Уведомление отправлено в канал %d для postID=%s", i, postID)
				newChannels = append(newChannels, ch)
			default:
				log.Printf("Канал %d занят для postID=%s, удаление канала", i, postID)
			}
		}
		r.SubscriptionHandler.commentChannels[postID] = newChannels
		if len(newChannels) == 0 {
			log.Printf("Все каналы удалены для postID=%s, удаление записи", postID)
			delete(r.SubscriptionHandler.commentChannels, postID)
		}
	} else {
		log.Printf("Нет подписчиков для postID=%s", postID)
	}
	r.SubscriptionHandler.mu.Unlock()
	return comment, nil
}

// CommentAdded реализует подписку commentAdded
func (s *subscriptionHandler) CommentAdded(ctx context.Context, postID string) (<-chan *Comment, error) {
	log.Printf("Запуск подписки commentAdded для postID=%s", postID)
	ch := make(chan *Comment, 1)
	s.mu.Lock()
	s.commentChannels[postID] = append(s.commentChannels[postID], ch)
	log.Printf("Канал добавлен для postID=%s, всего каналов: %d", postID, len(s.commentChannels[postID]))
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		log.Printf("Контекст подписки для postID=%s завершён", postID)
		s.mu.Lock()
		channels := s.commentChannels[postID]
		for i, c := range channels {
			if c == ch {
				s.commentChannels[postID] = append(channels[:i], channels[i+1:]...)
				log.Printf("Канал удалён для postID=%s, осталось каналов: %d", postID, len(s.commentChannels[postID]))
				break
			}
		}
		if len(s.commentChannels[postID]) == 0 {
			log.Printf("Все каналы удалены для postID=%s, удаление записи", postID)
			delete(s.commentChannels, postID)
		}
		s.mu.Unlock()
		log.Printf("Закрытие канала для postID=%s", postID)
		close(ch)
	}()
	return ch, nil
}
