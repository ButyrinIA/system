package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ButyrinIA/system/internal/config"
	"github.com/ButyrinIA/system/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Server: struct {
			Port string `yaml:"port"`
		}{Port: "8080"},
	}
	storage := &mockStorage{}
	server := New(cfg, storage)

	assert.NotNil(t, server)
	assert.Equal(t, cfg, server.cfg)
	assert.NotNil(t, server.handler)
}

func TestGenerateToken(t *testing.T) {
	token, err := generateToken("user1")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte("your-secret-key"), nil
	})
	assert.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	assert.True(t, ok)
	assert.Equal(t, "user1", claims["user_id"])
}

func TestValidateJWT(t *testing.T) {
	token, err := generateToken("user1")
	assert.NoError(t, err)

	userID, err := validateJWT(token)
	assert.NoError(t, err)
	assert.Equal(t, "user1", userID)
}

func TestValidateJWT_Invalid(t *testing.T) {
	_, err := validateJWT("invalid-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "пустой токен")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "user1",
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})
	wrongKeyToken, _ := token.SignedString([]byte("wrong-key"))
	_, err = validateJWT(wrongKeyToken)
	assert.Error(t, err)
}

func TestTokenHandler(t *testing.T) {
	cfg := &config.Config{
		Server: struct {
			Port string `yaml:"port"`
		}{Port: "8080"},
	}
	storage := &mockStorage{}
	New(cfg, storage)

	req, _ := http.NewRequest("GET", "/token", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := generateToken("user1")
		if err != nil {
			http.Error(w, "Ошибка генерации токена", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	}).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var response map[string]string
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.NotEmpty(t, response["token"])
}
