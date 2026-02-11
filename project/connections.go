package project

import (
	"database/sql"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

func ConnectDB() {
	var err error

	dsn := os.Getenv("MySQL_DSN")
	if dsn == "" {
		log.Fatal("MySQL_DSN not set in environment")
	}

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("Database not reachable", err)
	}
	log.Println("Database connected successfully.")
}

func ConnectRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis:%v", err)
	}
	log.Println("Redis connected successfully.")
}
