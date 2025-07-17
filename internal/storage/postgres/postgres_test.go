package postgres

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/ButyrinIA/system/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresStorage(t *testing.T) {
	log.SetOutput(os.Stdout)

	// Запуск тестового контейнера PostgreSQL
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:13",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "user",
			"POSTGRES_PASSWORD": "password",
			"POSTGRES_DB":       "posts",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}
	postgresC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Не удалось запустить контейнер PostgreSQL: %v", err)
	}
	defer postgresC.Terminate(ctx)

	// Получение DSN
	host, err := postgresC.Host(ctx)
	if err != nil {
		t.Fatalf("Не удалось получить хост контейнера: %v", err)
	}
	port, err := postgresC.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Не удалось получить порт контейнера: %v", err)
	}
	dsn := "postgres://user:password@" + host + ":" + port.Port() + "/posts?sslmode=disable"

	// Инициализация хранилища
	store, err := New(dsn)
	if err != nil {
		t.Fatalf("Не удалось инициализировать PostgresStorage: %v", err)
	}
	defer store.Close()

	t.Run("CreatePost and GetPost", func(t *testing.T) {
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
		assert.Equal(t, post.ID, retrieved.ID, "ID поста не совпадает")
		assert.Equal(t, post.Title, retrieved.Title, "Заголовок поста не совпадает")
	})

	t.Run("GetPost Not Found", func(t *testing.T) {
		_, err := store.GetPost(ctx, "non-existent-id")
		assert.Error(t, err, "Ожидалась ошибка для несуществующего поста")
		assert.Equal(t, "post not found", err.Error(), "Неверное сообщение об ошибке")
	})

	t.Run("CreateComment and GetComments", func(t *testing.T) {
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
}
