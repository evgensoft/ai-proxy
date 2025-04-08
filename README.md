[![Bugs](https://sonarcloud.io/api/project_badges/measure?project=evgensoft_ai-proxy&metric=bugs)](https://sonarcloud.io/summary/new_code?id=evgensoft_ai-proxy)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=evgensoft_ai-proxy&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=evgensoft_ai-proxy)

# Proxy for LLM Providers

## Overview
This service acts as a proxy for various Large Language Model (LLM) providers, including OpenAI, Groq, HuggingFace, and others. It allows users to seamlessly interact with multiple LLMs through a unified API, simplifying integration and request management.

## Features
- **Unified API**: Work with multiple LLM providers using a single API endpoint.
- **Provider Flexibility**: Easily switch between different LLM providers without changing your application code.
- **Request Management**: Handles authentication, routing, and error handling.
- **Rate Limiting**: Supports per-model request limits (minute/hour/day).
- **Simple Configuration**: YAML-based setup with support for multiple models.

## Getting Started

### Prerequisites
- Go (version 1.16 or higher)

### Installation

Clone the repository:
```bash
git clone https://github.com/evgensoft/ai-proxy.git
cd ai-proxy
```

### Configuration

Create a file named `config.yaml` in the root directory. Below is a **minimal working example** with one provider:

```yaml
models:
  - name: groq/llama-3.2-90b-vision-preview
    provider: groq
    priority: 1
    requests_per_minute: 10
    requests_per_hour: 100
    requests_per_day: 3500
    url: "https://api.groq.com/openai/v1/chat/completions"
    token: "your_groq_api_token"
    max_request_length: 128000
    model_size: BIG
```

> ✅ You can list multiple models from different providers in the same file.  
> 🛡️ Sensitive values like API tokens should be stored securely.

### Running the Service

To start the proxy server, run:

```bash
go run main.go
```
The server will start on `http://localhost:8080` by default. You can change the port with the following command:
```bash
go run main.go -port 9090
```
Replace `9090` with your desired port number.

## Example Usage

### Using cURL

```bash
curl -X POST http://localhost:8080/chat/completions \
     -H "Content-Type: application/json" \
     -d '{
           "model": "groq/llama-3.2-90b-vision-preview",
           "messages": [
             {
               "role": "system",
               "content": "You are a helpful assistant."
             },
             {
               "role": "user",
               "content": "Tell me a joke."
             }
           ]
         }'
```

## Contributing
Contributions are welcome! Please submit a pull request or open an issue to discuss improvements.

## License
This project is licensed under the MIT License.