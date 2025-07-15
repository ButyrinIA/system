package server

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/ButyrinIA/system/internal/config"
	"github.com/ButyrinIA/system/internal/graphql"
	"github.com/ButyrinIA/system/internal/storage"
	"github.com/gorilla/websocket"
)

type Server struct {
	cfg     *config.Config
	storage storage.Storage
}

func New(cfg *config.Config, storage storage.Storage) *Server {
	return &Server{cfg: cfg, storage: storage}
}

func (s *Server) Run() error {
	srv := handler.NewDefaultServer(graphql.NewExecutableSchema(graphql.Config{
		Resolvers: &graphql.Resolver{
			Storage:             s.storage,
			SubscriptionHandler: graphql.NewSubscriptionHandler(),
		},
	}))

	srv.AddTransport(transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	})

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)

	return http.ListenAndServe(":"+s.cfg.Server.Port, nil)
}
