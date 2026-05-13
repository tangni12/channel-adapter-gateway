# Architecture

## Positioning

The gateway sits between NewAPI and third-party providers:

```text
Client
  -> NewAPI official channel configuration
  -> Channel Adapter Gateway
  -> Third-party upstream provider
```

NewAPI only sees an official-compatible API. The gateway handles the third-party URL, model name, parameters, multipart field names, defaults, and response usage normalization.

## Abstraction

The code supports target official protocols. The first target protocol is:

```text
openai
```

The first target endpoints are:

```text
openai.images.generations -> POST /v1/images/generations
openai.images.edits       -> POST /v1/images/edits
```

Official endpoint definitions are stored in `internal/official`, because the OpenAI/Gemini-style public protocol is stable and should be maintained by code. Third-party details are stored in `providers` and `mapping_rules`, because those rules change often and should be configured from the UI.

## Main tables

### providers

Stores upstream providers.

| Field | Meaning |
|---|---|
| code | Provider code, for example `minimax` |
| type | Provider type |
| base_url | Upstream base URL |
| api_key | Optional fixed upstream API key |
| timeout_seconds | Upstream timeout |
| enabled | Whether this provider can be used |

### users

Stores admin console accounts. The admin console is a React app served by the backend from `web/dist`.

### mapping_rules

Stores official-to-upstream conversion rules.

| Field | Meaning |
|---|---|
| public_model | Model exposed to NewAPI, for example `gpt-image-2` |
| target_protocol | Official protocol, for example `openai` |
| target_endpoint | Official endpoint key, for example `openai.images.generations` |
| provider_code | Which upstream provider to use |
| upstream_model | Real upstream model, for example `canvas-20` |
| upstream_model_field | Field name used to write the upstream model, usually `model` |
| upstream_method | Upstream HTTP method |
| upstream_path | Upstream URL path |
| body_mode | `json` or `multipart` |
| field_map_json | JSON field name mapping |
| file_field_map_json | Multipart file field mapping |
| defaults_json | Default upstream parameters |
| ignore_fields_json | Fields to drop |
| normalize_openai_usage | Whether to add `prompt_tokens` and `completion_tokens` from `input_tokens` and `output_tokens` |

### request_logs

Stores request traces, including upstream URL, status code, latency, provider trace ID, and normalized usage.

## Canvas-20 example

External NewAPI request:

```http
POST /v1/images/generations
model=gpt-image-2
```

Mapping result:

```http
POST https://api.minimax.io/v1/content/models/canvas-20/generations
model=canvas-20
```

The mapping is configured in the admin UI and saved to database fields equivalent to:

```yaml
public_model: gpt-image-2
target_protocol: openai
target_endpoint: openai.images.generations
upstream_model: canvas-20
upstream_path: /v1/content/models/canvas-20/generations
ignore_fields:
  - response_format
defaults:
  stream: false
```

No Canvas-specific adapter code is required for this conversion.
