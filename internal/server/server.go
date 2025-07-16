package server

import (
	"context"
	"errors"
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

	// Configure WebSocket transport
	srv.AddTransport(&transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		KeepAlivePingInterval: 10 * time.Second,
	})

	// Authentication middleware
	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		oc := graphql.GetOperationContext(ctx)
		if oc == nil {
			return next(ctx)
		}

		authHeader := oc.Headers.Get("Authorization")
		if authHeader != "" {
			if !strings.HasPrefix(authHeader, "Bearer ") {
				oc.Error(ctx, gqlerror.Errorf("Invalid authorization header format"))
				return next(ctx) // Продолжаем выполнение, ошибка уже добавлена
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			userID, err := validateJWT(token)
			if err != nil {
				oc.Error(ctx, gqlerror.Errorf("Invalid token: %v", err))
				return next(ctx)
			}

			ctx = context.WithValue(ctx, "userID", userID)
		}

		return next(ctx)
	})

	http.Handle("/", playground.Handler("GraphQL Playground", "/query"))
	http.Handle("/query", srv)

	return http.ListenAndServe(":"+s.cfg.Server.Port, nil)
}

func validateJWT(token string) (string, error) {
	if token == "" {
		return "", errors.New("empty token")
	}
	return "user1", nil
}
