# ⚙️ Shipr Backend — Control Plane & Worker Engine

Backend en **Go 1.23+** ultra-rapide et sobre en mémoire pour le PaaS **Shipr**.

## 🛠️ Stack Technique
- **Framework Web API** : Go Fiber v2
- **Data Layer** : PostgreSQL 16 + sqlc + pgx/v5 (type-safe sans injection SQL)
- **Job Queue** : hibiken/asynq + Redis 7
- **Conteneurs** : Docker Go SDK officiel (`moby/moby`)
- **Moteur Universel** : Nixpacks CLI & Docker build fallback
- **Ingress & TLS** : Client REST pour Caddy Server v2 Admin API (`:2019`)
- **Streaming Logs** : Server-Sent Events (SSE) via Redis Pub/Sub
- **Chiffrement** : AES-256-GCM pour les secrets d'environnement

## 🚀 Démarrage
```bash
# Lancer l'API Control Plane
go run ./cmd/api

# Lancer le Worker de build
go run ./cmd/worker

# Exécuter les tests
go test ./...
```
