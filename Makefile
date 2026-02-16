.PHONY: build run clean up down logs

APP=ems

build:
	go build -o $(APP)

run:
	go run main.go

clean:
	rm -f $(APP)

up:
	docker compose up --build -d 

down:
	docker compose down

logs:
	docker compose logs -f