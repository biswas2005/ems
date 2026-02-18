.PHONY: run build docker-build docker-up docker-down logs clean

run:
	go run main.go

build:
	go build -o ems

docker-build:
	docker compose build

docker-up:
	docker compose up -d 

docker-down:
	docker compose down

logs:
	docker compose logs -f

clean:
	docker compose down -v