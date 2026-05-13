# Channel Adapter Gateway

This project is a standalone gateway for converting third-party model APIs into official protocol shapes that NewAPI already supports.

The first built-in target protocol is OpenAI image API:

- External endpoint: `POST /v1/images/generations`
- External endpoint: `POST /v1/images/edits`
- Example public model: `gpt-image-2`
- Example upstream model: MiniMax `canvas-20`

NewAPI can configure this gateway as an OpenAI channel:

```text
Channel type: OpenAI
Base URL: http://channel-adapter-gateway:8088
Model: gpt-image-2
API Key: upstream provider key, such as MiniMax key
```

By default, the gateway forwards NewAPI's `Authorization: Bearer xxx` header to upstream. If a provider has a fixed `api_key` configured in the admin UI, that key takes priority.

## Core idea

The gateway is not a hardcoded Canvas-20 adapter. Official protocol shapes are built into code, while third-party provider and conversion rules are stored in the database:

- public model name
- target official protocol and endpoint
- upstream provider
- upstream URL path and method
- field mapping
- file field mapping
- default values
- ignored fields
- response usage normalization

Mappings are loaded into memory on startup. When providers or mappings are changed in the admin UI, the memory cache is refreshed immediately.

## Files

- `configs/config.yaml`: service, database, and initial admin configuration
- `internal/official`: built-in official endpoint definitions, currently OpenAI image generation and image edits
- `internal/router`: route registration split by responsibility and official channel
- `web/`: React admin console source and production build output
- `docs/ARCHITECTURE.md`: architecture notes
- `docs/API.md`: API documentation
- `Dockerfile` and `docker-compose.yml`: deployment examples

Default admin account:

```text
admin / admin123456
```

Change `server.jwt_secret` and the admin password before production use.
