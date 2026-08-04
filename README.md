# SmartLock — Tuya online temporary passwords + Pulsar unlock webhook

This repository is the canonical SmartLock service for AKSCK (GitLab).

## Features

- gRPC: create / delete **online** temporary door passwords (`device_id` in request)
- Auth: API key via gRPC metadata `x-api-key` (or `Authorization: Bearer`)
- Pulsar: `StatusReport` / `DeviceProperty` → HTTP webhook (config DP filter)

## Local run

```bash
cp .env.example .env   # or use deploy/us/docker-compose.yml env
go run .
```

## Deploy

GitLab CI builds the image and deploys on the US runner (`deploy-us`), same pattern as RentBot telegram.
