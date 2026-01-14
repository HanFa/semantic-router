---
title: Anthropic Claude Configuration
sidebar_label: Anthropic Models
---

# Anthropic Claude Configuration

This guide explains how to configure Anthropic Claude models as backend
inference providers. The semantic router accepts OpenAI-format requests and
automatically translates them to Anthropic's Messages API format, returning
responses in OpenAI format for seamless client compatibility.

## Environment Setup

### Setting the API Key

Anthropic API keys must be available as environment variables. Create a `.env`
file or export directly:

```bash
export ANTHROPIC_API_KEY=sk-ant-api03-xxxxxxxxxxxx
```

### Verifying the Environment

Before running the router, verify the API key is accessible:

```bash
# Should print your API key
echo $ANTHROPIC_API_KEY

# Test the key directly with Anthropic API
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-sonnet-4-5","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
```

## Basic Configuration

### Minimal Setup

Add Claude models to your `model_config` with `api_format: "anthropic"`:

```yaml
model_config:
  "claude-sonnet-4-5":
    api_format: "anthropic"
    access_key: "${ANTHROPIC_API_KEY}"

default_model: "claude-sonnet-4-5"
```

> The `${ANTHROPIC_API_KEY}` syntax references the environment variable. You can
> also hardcode the key directly (not recommended for security).

### Hybrid Configuration

Mix Anthropic models with local vLLM endpoints:

```yaml
vllm_endpoints:
  - name: "local-gpu"
    address: "127.0.0.1"
    port: 8000
    weight: 1

model_config:
  # Local model via vLLM
  "Qwen/Qwen2.5-7B-Instruct":
    reasoning_family: "qwen3"
    preferred_endpoints: [ "local-gpu" ]

  # Cloud model via Anthropic API
  "claude-sonnet-4-5":
    api_format: "anthropic"
    access_key: "${ANTHROPIC_API_KEY}"

decisions:
  - name: "local_processing"
    description: "Use local model for general queries"
    priority: 50
    modelRefs:
      - model: "Qwen/Qwen2.5-7B-Instruct"
        use_reasoning: true

  - name: "cloud_processing"
    description: "Use Claude for complex queries"
    priority: 100
    rules:
      operator: "OR"
      conditions:
        - type: "domain"
          name: "computer_science"
    modelRefs:
      - model: "claude-sonnet-4-5"
        use_reasoning: false
```

## Sending Requests

Once configured, send standard OpenAI-format requests:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Explain quantum computing in simple terms."}
    ],
    "max_tokens": 1024
  }'
```

The router will:

1. Parse the OpenAI-format request
2. Convert it to Anthropic Messages API format
3. Call the Anthropic API
4. Convert the response back to OpenAI format
5. Return the response to the client

## Response API Support

The router also supports the OpenAI Response API (`/v1/responses`) with Anthropic
backends. Requests are translated to Chat Completions format, sent to Claude, and
responses are converted back to Response API format.

### Enabling Response API

Add the `response_api` configuration to your config file:

```yaml
response_api:
  enabled: true
  store_backend: "memory"
  ttl_seconds: 86400
  max_responses: 1000
```

### Sending Response API Requests

```bash
curl -X POST http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "input": "What is the difference between threads and processes?",
    "instructions": "You are a helpful assistant"
  }'
```

### Example Response

```json
{
  "id": "resp_54a277277aa864ee6e18754d",
  "object": "response",
  "created_at": 1768803933,
  "model": "claude-sonnet-4-5",
  "status": "completed",
  "output": [
    {
      "type": "message",
      "id": "item_594ea35de68d22a8a3563cc7",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "# Threads vs Processes\n\n## **Process**\n- Independent execution unit with its own memory space..."
        }
      ],
      "status": "completed"
    }
  ],
  "output_text": "# Threads vs Processes...",
  "usage": {
    "input_tokens": 21,
    "output_tokens": 311,
    "total_tokens": 332
  },
  "instructions": "You are a helpful assistant"
}
```

## Supported Parameters

The following OpenAI parameters are translated to Anthropic equivalents:

| OpenAI Parameter        | Anthropic Equivalent  | Notes                                     |
|-------------------------|-----------------------|-------------------------------------------|
| `model`                 | `model`               | Model name passed directly                |
| `messages`              | `messages` + `system` | System messages extracted separately      |
| `max_tokens`            | `max_tokens`          | Required by Anthropic (defaults to 4096)  |
| `max_completion_tokens` | `max_tokens`          | Alternative to max_tokens                 |
| `temperature`           | `temperature`         | 0.0 to 1.0                                |
| `top_p`                 | `top_p`               | Nucleus sampling                          |
| `stop`                  | `stop_sequences`      | Stop sequences                            |
| `stream`                | —                     | **Not supported** (see limitations below) |

## Current Limitations

### Streaming Not Supported

The Anthropic backend currently only supports non-streaming responses. If you
send
a request with `stream: true`, the router will return an error:

```json
{
  "error": {
    "message": "Streaming is not supported for Anthropic models. Please set stream=false in your request.",
    "type": "invalid_request_error",
    "code": 400
  }
}
```

**Workaround:** Ensure all requests to Anthropic models use `stream: false` or
omit
the `stream` parameter entirely (defaults to `false`).

```bash
# Correct - non-streaming request
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'
```

:::tip
If your application requires streaming responses, consider using a local vLLM
endpoint or an OpenAI-compatible API that supports streaming, and configure
decision-based routing to direct streaming-critical workloads to those backends.
:::
