package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/ButyrinIA/system/internal/models"
	"github.com/ButyrinIA/system/internal/storage"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Server представляет HTTP-сервер для обработки GraphQL-запросов
type Server struct {
	cfg     *config.Config
	storage storage.Storage
	handler *handler.Server
}

// New создаёт новый сервер с заданной конфигурацией и хранилищем
func New(cfg *config.Config, storage storage.Storage) *Server {
	log.Printf("Создание нового сервера с портом: %s", cfg.Server.Port)

	// Инициализация DataLoader для пакетной загрузки комментариев
	commentLoader := dataloader.NewBatchedLoader(
		func(ctx context.Context, keys []string) []*dataloader.Result[*models.PaginatedComments] {
			results := make([]*dataloader.Result[*models.PaginatedComments], len(keys))
			for i, postID := range keys {
				comments, err := storage.GetComments(ctx, postID, nil, 10, nil)
				if err != nil {
					log.Printf("Ошибка загрузки комментариев для postID=%s: %v", postID, err)
					results[i] = &dataloader.Result[*models.PaginatedComments]{Error: err}
				} else {
					log.Printf("Получено комментариев для postID=%s: %d", postID, len(comments.Comments))
					results[i] = &dataloader.Result[*models.PaginatedComments]{Data: comments}
				}
			}
			return results
		},
		dataloader.WithCache[string, *models.PaginatedComments](&dataloader.NoCache[string, *models.PaginatedComments]{}),
	)

	// Создание GraphQL-сервера с резолвером
	resolver := mygraphql.NewResolver(storage, commentLoader)
	executableSchema := mygraphql.NewExecutableSchema(mygraphql.Config{
		Resolvers: resolver,
	})
	srv := handler.NewDefaultServer(executableSchema)
	log.Println("Сервер GraphQL успешно инициализирован")

	// Конфигурация WebSocket-транспорта
	srv.AddTransport(&transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				log.Printf("Проверка происхождения WebSocket: %s", r.Header.Get("Origin"))
				return true
			},
		},
		KeepAlivePingInterval: 30 * time.Second, // Увеличенный таймаут для стабильности
		InitFunc: func(ctx context.Context, initPayload transport.InitPayload) (context.Context, *transport.InitPayload, error) {
			log.Printf("Инициализация WebSocket-соединения, payload: %+v", initPayload)
			authHeader, ok := initPayload["Authorization"].(string)
			if ok && authHeader != "" {
				if !strings.HasPrefix(authHeader, "Bearer ") {
					log.Printf("Неверный формат заголовка авторизации в WebSocket: %s", authHeader)
					return ctx, nil, gqlerror.Errorf("Неверный формат заголовка авторизации")
				}
				token := strings.TrimPrefix(authHeader, "Bearer ")
				userID, err := validateJWT(token)
				if err != nil {
					log.Printf("Недействительный токен в WebSocket: %v", err)
					return ctx, nil, gqlerror.Errorf("Недействительный токен: %v", err)
				}
				log.Printf("Успешная аутентификация WebSocket: %s", userID)
				ctx = context.WithValue(ctx, "userID", userID)
				return ctx, nil, nil
			}
			log.Println("Заголовок авторизации отсутствует в WebSocket")
			return ctx, nil, nil
		},
	})

	// Middleware для аутентификации HTTP-запросов
	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		oc := graphql.GetOperationContext(ctx)
		log.Printf("Обработка операции: %s", oc.OperationName)
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
		// Передача commentLoader в контекст
		ctx = context.WithValue(ctx, "commentLoader", commentLoader)
		return next(ctx)
	})

	return &Server{cfg: cfg, storage: storage, handler: srv}
}

// Run запускает сервер
func (s *Server) Run() error {
	http.Handle("/", playground.Handler("GraphQL Playground", "/query"))
	http.Handle("/query", s.handler)
	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Запрос на генерацию токена")
		token, err := generateToken("user1")
		if err != nil {
			log.Printf("Ошибка генерации токена: %v", err)
			http.Error(w, "Ошибка генерации токена", http.StatusInternalServerError)
			return
		}
		log.Printf("Токен успешно сгенерирован: %s", token)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	})

	log.Printf("Сервер запущен на порту :%s", s.cfg.Server.Port)
	return http.ListenAndServe(":"+s.cfg.Server.Port, nil)
}

func validateJWT(token string) (string, error) {
	log.Printf("Валидация токена: %s", token)
	if token == "" {
		log.Println("Ошибка: пустой токен")
		return "", errors.New("пустой токен")
	}
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("Ошибка: неожиданный метод подписи: %v", token.Header["alg"])
			return nil, fmt.Errorf("неожиданный метод подписи: %v", token.Header["alg"])
		}
		return []byte("your-secret-key"), nil
	})
	if err != nil {
		log.Printf("Ошибка парсинга токена: %v", err)
		return "", err
	}
	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok && parsedToken.Valid {
		userID, ok := claims["user_id"].(string)
		if !ok {
			log.Println("Ошибка: user_id не найден в токене")
			return "", errors.New("user_id не найден в токене")
		}
		log.Printf("Токен валиден, userID: %s", userID)
		return userID, nil
	}
	log.Println("Ошибка: недействительный токен")
	return "", errors.New("недействительный токен")
}

func generateToken(userID string) (string, error) {
	log.Printf("Генерация токена для userID: %s", userID)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, err := token.SignedString([]byte("your-secret-key"))
	if err != nil {
		log.Printf("Ошибка при подписи токена: %v", err)
		return "", err
	}
	log.Printf("Токен успешно создан: %s", tokenString)
	return tokenString, nil
}
