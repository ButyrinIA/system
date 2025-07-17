package memory

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/ButyrinIA/system/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMemoryStorage(t *testing.T) {
	// Отключение логирования для тестов
	log.SetOutput(os.Stdout)

	t.Run("CreatePost and GetPost", func(t *testing.T) {
		store := New()
		ctx := context.Background()

		post := &models.Post{
			ID:            uuid.New().String(),
			Title:         "Тестовый пост",
			Content:       "Содержимое",
			AuthorID:      "user1",
			AllowComments: true,
			CreatedAt:     time.Now(),
		}

		err := store.CreatePost(ctx, post)
		assert.NoError(t, err, "Ошибка при создании поста")

		retrieved, err := store.GetPost(ctx, post.ID)
		assert.NoError(t, err, "Ошибка при получении поста")
		assert.Equal(t, post, retrieved, "Полученный пост не совпадает с созданным")
	})

	t.Run("GetPost Not Found", func(t *testing.T) {
		store := New()
		ctx := context.Background()

		_, err := store.GetPost(ctx, "non-existent-id")
		assert.Error(t, err, "Ожидалась ошибка для несуществующего поста")
		assert.Equal(t, "post not found", err.Error(), "Неверное сообщение об ошибке")
	})

	t.Run("ListPosts", func(t *testing.T) {
		store := New()
		ctx := context.Background()

		// Создаем два поста
		post1 := &models.Post{
			ID:            uuid.New().String(),
			Title:         "Пост 1",
			Content:       "Содержимое 1",
			AuthorID:      "user1",
			AllowComments: true,
			CreatedAt:     time.Now().Add(-2 * time.Hour),
		}
		post2 := &models.Post{
			ID:            uuid.New().String(),
			Title:         "Пост 2",
			Content:       "Содержимое 2",
			AuthorID:      "user1",
			AllowComments: true,
			CreatedAt:     time.Now().Add(-1 * time.Hour),
		}

		assert.NoError(t, store.CreatePost(ctx, post1))
		assert.NoError(t, store.CreatePost(ctx, post2))

		// Тестируем пагинацию
		result, err := store.ListPosts(ctx, 1, nil)
		assert.NoError(t, err, "Ошибка при получении списка постов")
		assert.Len(t, result.Posts, 1, "Ожидался один пост")
		assert.Equal(t, post2.ID, result.Posts[0].ID, "Ожидался более новый пост")
		assert.Equal(t, 2, result.TotalCount, "Неверное общее количество постов")
		assert.NotNil(t, result.NextCursor, "Ожидался ненулевой курсор")

		// Тестируем с курсором
		result, err = store.ListPosts(ctx, 1, result.NextCursor)
		assert.NoError(t, err, "Ошибка при получении постов с курсором")
		assert.Len(t, result.Posts, 1, "Ожидался один пост")
		assert.Equal(t, post1.ID, result.Posts[0].ID, "Ожидался более старый пост")
	})

	t.Run("CreateComment and GetComments", func(t *testing.T) {
		store := New()
		ctx := context.Background()

		post := &models.Post{
			ID:            uuid.New().String(),
			Title:         "Тестовый пост",
			Content:       "Содержимое",
			AuthorID:      "user1",
			AllowComments: true,
			CreatedAt:     time.Now(),
		}
		assert.NoError(t, store.CreatePost(ctx, post))

		comment := &models.Comment{
			ID:        uuid.New().String(),
			PostID:    post.ID,
			AuthorID:  "user1",
			Content:   "Тестовый комментарий",
			CreatedAt: time.Now(),
		}
		err := store.CreateComment(ctx, comment)
		assert.NoError(t, err, "Ошибка при создании комментария")

		comments, err := store.GetComments(ctx, post.ID, nil, 10, nil)
		assert.NoError(t, err, "Ошибка при получении комментариев")
		assert.Len(t, comments.Comments, 1, "Ожидался один комментарий")
		assert.Equal(t, comment.ID, comments.Comments[0].ID, "Полученный комментарий не совпадает")
	})

	t.Run("GetComments with ParentID", func(t *testing.T) {
		store := New()
		ctx := context.Background()

		post := &models.Post{
			ID:            uuid.New().String(),
			Title:         "Тестовый пост",
			Content:       "Содержимое",
			AuthorID:      "user1",
			AllowComments: true,
			CreatedAt:     time.Now(),
		}
		assert.NoError(t, store.CreatePost(ctx, post))

		parentComment := &models.Comment{
			ID:        uuid.New().String(),
			PostID:    post.ID,
			AuthorID:  "user1",
			Content:   "Родительский комментарий",
			CreatedAt: time.Now(),
		}
		reply := &models.Comment{
			ID:        uuid.New().String(),
			PostID:    post.ID,
			ParentID:  &parentComment.ID,
			AuthorID:  "user2",
			Content:   "Ответ",
			CreatedAt: time.Now().Add(1 * time.Hour),
		}

		assert.NoError(t, store.CreateComment(ctx, parentComment))
		assert.NoError(t, store.CreateComment(ctx, reply))

		comments, err := store.GetComments(ctx, post.ID, &parentComment.ID, 10, nil)
		assert.NoError(t, err, "Ошибка при получении ответов")
		assert.Len(t, comments.Comments, 1, "Ожидался один ответ")
		assert.Equal(t, reply.ID, comments.Comments[0].ID, "Полученный ответ не совпадает")
	})

	t.Run("Close", func(t *testing.T) {
		store := New()
		ctx := context.Background()

		post := &models.Post{
			ID:            uuid.New().String(),
			Title:         "Тестовый пост",
			Content:       "Содержимое",
			AuthorID:      "user1",
			AllowComments: true,
			CreatedAt:     time.Now(),
		}
		assert.NoError(t, store.CreatePost(ctx, post))

		err := store.Close()
		assert.NoError(t, err, "Ошибка при закрытии хранилища")

		_, err = store.GetPost(ctx, post.ID)
		assert.Error(t, err, "Ожидалась ошибка после очистки хранилища")
	})
}
