[![Bugs](https://sonarcloud.io/api/project_badges/measure?project=evgensoft_ai-proxy&metric=bugs)](https://sonarcloud.io/summary/new_code?id=evgensoft_ai-proxy)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=evgensoft_ai-proxy&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=evgensoft_ai-proxy)

# Proxy for LLM Providers

## Overview
This service acts as a proxy for various Large Language Model (LLM) providers, including OpenAI. It allows users to seamlessly interact with multiple LLMs through a unified API, simplifying the process of integrating AI capabilities into applications. The proxy handles requests, manages authentication, and provides a consistent interface for different LLM providers.

## Features
- **Unified API**: Interact with multiple LLM providers using a single API endpoint.
- **Provider Flexibility**: Easily switch between different LLM providers without changing your application code.
- **Request Management**: Handles request formatting, authentication, and error management.
- **Rate Limiting**: Implement rate limiting to manage usage across different providers.
- **Logging and Monitoring**: Track usage and performance metrics for each provider.

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
Create a `.env` file in the root directory and add your LLM provider API keys:
```plaintext
OPENAI_API_KEY=your_openai_api_key
GROQ_TOKEN=your_groq_api_key
CLOUDFLARE_TOKEN=your_cloudflare_api_key
GEMINI_TOKEN=your_google_api_key
```
You can use a package like `godotenv` to load environment variables from the `.env` file.

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
You can also use `cURL` to interact with the proxy service. Here is an example:

```bash
curl -X POST http://localhost:8080/groq/chat/completions \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer YOUR_GROQ_TOKEN" \
     -d '{
           "messages": [
             {
               "role": "system",
               "content": "You are a helpful assistant."
             },
             {
               "role": "user",
               "content": "Tell me a joke."
             }
           ],
           "model": "llama-3.1-70b-versatile"
         }'
```
Replace `YOUR_GROQ_TOKEN` with your actual GROQ API token.

## Contributing
Contributions are welcome! Please submit a pull request or open an issue to discuss improvements.

## License
This project is licensed under the MIT License.