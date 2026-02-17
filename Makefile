APP_NAME=ems

.PHONY: build run docker-up docker-down clean logs

build:
	go build -o $(APP_NAME)

run:
	go run main.go

docker-up:
	docker compose up --build

docker-down:
	docker compose down

docker-clean:
	docker compose down -v

logs:
	docker compose logs -f

clean:
	rm -f $(APP_NAME)
