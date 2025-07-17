package main

import (
	"flag"
	"log"

	"github.com/ButyrinIA/system/internal/config"
	"github.com/ButyrinIA/system/internal/server"
	"github.com/ButyrinIA/system/internal/storage"
	"github.com/ButyrinIA/system/internal/storage/memory"
	"github.com/ButyrinIA/system/internal/storage/postgres"
)

func main() {
	configPath := flag.String("config", "config.yaml", "путь к файлу конфигурации")
	storageType := flag.String("storage", "memory", "тип хранилища: memory или postgres")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Не удалось загрузить конфигурацию: %v", err)
	}

	var store storage.Storage
	switch *storageType {
	case "postgres":
		log.Println("Инициализация хранилища PostgreSQL")
		store, err = postgres.New(cfg.Postgres.DSN)
		if err != nil {
			log.Fatalf("Не удалось инициализировать PostgreSQL: %v", err)
		}
	case "memory":
		log.Println("Инициализация хранилища Memory")
		store = memory.New()
	default:
		log.Fatalf("Неизвестный тип хранилища: %s", *storageType)
	}
	defer store.Close()

	srv := server.New(cfg, store)
	log.Println("Запуск сервера")
	if err := srv.Run(); err != nil {
		log.Fatalf("Не удалось запустить сервер: %v", err)
	}
}
