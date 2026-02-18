package project

import (
	"database/sql"
	"log"
	"os"

	//Redis client
	"github.com/redis/go-redis/v9"
)

// ConnectDB intializes a connection to the MySQL database
func ConnectDB() {
	var err error

	//Read DSN (Data Source Name) from environment variables
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		//Fail if DSN is not set
		log.Fatal("MYSQL_DSN not set in environment")
	}

	//Open a connection using the MySQL driver
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}

	//Verify that the database is reachable
	err = db.Ping()
	if err != nil {
		log.Fatal("Database not reachable", err)
	}
	log.Println("Database connected successfully.")
}

// ConnectRedis initializes a connection to the Redis server
func ConnectRedis() {
	//Create a new Redis client with default options
	rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"), //Redis server address
		Password: "",                      //No password set
		DB:       0,                       //Use default DB
	})

	//Ping Redis to confirm connectivity
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis:%v", err)
	}
	log.Println("Redis connected successfully.")
}
