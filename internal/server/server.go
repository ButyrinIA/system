package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/ButyrinIA/system/internal/config"
	mygraphql "github.com/ButyrinIA/system/internal/graphql"
	"github.com/ButyrinIA/system/internal/storage"
	"github.com/gorilla/websocket"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type Server struct {
	cfg     *config.Config
	storage storage.Storage
}

func New(cfg *config.Config, storage storage.Storage) *Server {
	return &Server{cfg: cfg, storage: storage}
}

func (s *Server) Run() error {
	resolver := mygraphql.NewResolver(s.storage)
	executableSchema := mygraphql.NewExecutableSchema(mygraphql.Config{
		Resolvers: resolver,
	})
	srv := handler.NewDefaultServer(executableSchema)

	// Конфигурация WebSocket-транспорта
	srv.AddTransport(&transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		KeepAlivePingInterval: 10 * time.Second,
	})

	// Middleware для аутентификации
	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		oc := graphql.GetOperationContext(ctx)
		authHeader := oc.Headers.Get("Authorization")
		if authHeader != "" {
			if !strings.HasPrefix(authHeader, "Bearer ") {
				log.Printf("Неверный формат заголовка авторизации: %s", authHeader)
				oc.Error(ctx, gqlerror.Errorf("Неверный формат заголовка авторизации"))
				return next(ctx)
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			userID, err := validateJWT(token)
			if err != nil {
				log.Printf("Недействительный токен: %v", err)
				oc.Error(ctx, gqlerror.Errorf("Недействительный токен: %v", err))
				return next(ctx)
			}

			log.Printf("Успешная аутентификация пользователя: %s", userID)
			ctx = context.WithValue(ctx, "userID", userID)
		} else {
			log.Println("Заголовок авторизации отсутствует")
		}

		return next(ctx)
	})

	http.Handle("/", playground.Handler("GraphQL Playground", "/query"))
	http.Handle("/query", srv)

	log.Printf("Сервер запущен на порту :%s", s.cfg.Server.Port)
	return http.ListenAndServe(":"+s.cfg.Server.Port, nil)
}

func validateJWT(token string) (string, error) {
	if token == "" {
		return "", errors.New("пустой токен")
	}
	// Заменить на реальную проверку JWT github.com/golang-jwt/jwt/v5
	return "user1", nil // Заглушка
}
