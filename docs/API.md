# API

## Official-compatible APIs

### List models

```http
GET /v1/models
```

### OpenAI image generation

```http
POST /v1/images/generations
Authorization: Bearer <UPSTREAM_PROVIDER_KEY>
Content-Type: application/json
```

Example:

```json
{
  "model": "gpt-image-2",
  "prompt": "a cute cat sitting on a windowsill",
  "n": 1,
  "size": "1024x1024",
  "quality": "medium",
  "output_format": "png"
}
```

After creating a mapping in the admin UI, this can be converted to MiniMax Canvas-20:

```http
POST https://api.minimax.io/v1/content/models/canvas-20/generations
```

### OpenAI image edits

```http
POST /v1/images/edits
Authorization: Bearer <UPSTREAM_PROVIDER_KEY>
Content-Type: multipart/form-data
```

OpenAI-style image fields such as `image` and `image[]` can be mapped to upstream fields such as `images[]`.

## Admin APIs

### Login

```http
POST /api/auth/login
```

```json
{
  "username": "admin",
  "password": "admin123456"
}
```

### Providers

```http
GET    /api/providers
POST   /api/providers
PUT    /api/providers/{id}
DELETE /api/providers/{id}
```

### Users

```http
GET    /api/users
POST   /api/users
PUT    /api/users/{id}
DELETE /api/users/{id}
```

### Mappings

```http
GET    /api/mappings
POST   /api/mappings
PUT    /api/mappings/{id}
DELETE /api/mappings/{id}
```

Any provider or mapping change refreshes the in-memory mapping cache.

### Official endpoint catalog

```http
GET /api/official/endpoints
GET /api/official/endpoints/{key}
```

The admin UI uses this catalog to show fixed official request and response fields, then stores third-party field mappings in `mapping_rules`.

### Logs

```http
GET /api/request-logs?limit=80
```
