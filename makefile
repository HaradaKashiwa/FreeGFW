.PHONY: ui docker all run build

all: ui docker

ui:
	cd frontend && npm run build

docker:
	@echo "Building Docker image..."
	mkdir -p dist
	docker buildx build --no-cache --platform linux/amd64 --no-cache -t freegfw:latest --progress=plain ./
	docker save freegfw:latest > dist/freegfw.tar

run:
	go run -tags with_reality_server .

build:
	go build -tags with_reality_server -o freegfw .
