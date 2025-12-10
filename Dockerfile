# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copiar go mod files
COPY go.mod go.sum ./
RUN go mod tidy

# Copiar código fuente
COPY . .

# Compilar el binario
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dota-discord-bot .

# Stage 2: Runtime
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copiar el binario desde el builder
COPY --from=builder /app/dota-discord-bot .

# Copiar archivos JSON necesarios
COPY --from=builder /app/dota/heroes.json ./dota/
COPY --from=builder /app/dota/game_mode.json ./dota/
COPY --from=builder /app/dota/lobby_type.json ./dota/

# Crear directorios necesarios
RUN mkdir -p data logs

# Exponer puerto (aunque el bot no lo use directamente)
EXPOSE 8080

# Ejecutar el bot
CMD ["./dota-discord-bot"]

