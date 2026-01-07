# Variables
IMAGE_NAME := orgmcr.or-gm.com/osmargm1202/dota-discord-bot
TAG := latest
FULL_IMAGE := $(IMAGE_NAME):$(TAG)

.PHONY: build push all help

# Build de la imagen Docker
build:
	@echo "🔨 Construyendo imagen Docker: $(FULL_IMAGE)"
	docker build -t $(FULL_IMAGE) .

# Push de la imagen al registry
push:
	@echo "📤 Subiendo imagen Docker: $(FULL_IMAGE)"
	docker push $(FULL_IMAGE)

# Build y push en un solo comando
all: build push
	@echo "✅ Build y push completados"

# Ayuda
help:
	@echo "Comandos disponibles:"
	@echo "  make build  - Construir la imagen Docker"
	@echo "  make push   - Subir la imagen al registry"
	@echo "  make all    - Construir y subir la imagen"
	@echo "  make help   - Mostrar esta ayuda"

