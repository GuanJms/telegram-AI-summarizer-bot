# Telegram Trader Coder Bot

A Go-based Telegram bot that provides AI-powered text summarization and Yahoo Finance stock charts.

## Features

- **AI Text Summarization**: Uses OpenAI GPT-4o-mini to summarize chat messages
- **Stock Charts**: Generates 5-minute stock charts using Yahoo Finance data
- **SQLite Storage**: Stores chat messages for summarization
- **Webhook Support**: Handles Telegram webhooks for real-time message processing
- **Docker Support**: Containerized deployment with Docker and Docker Compose
- **Multi-platform Builds**: Cross-compilation support via Makefile

## Commands

- `/summary [hours]` - Summarize chat messages from the last N hours (default: 1 hour, max: 48 hours)
- `/stock SYMBOL` - Get a 5-minute chart for the specified stock symbol (e.g., `/stock AAPL`)

## Quick Start

### Using Docker (Recommended)

1. **Clone the repository:**

```bash
git clone <repository-url>
cd telegram-tradercoder-bot
```

2. **Set environment variables:**

```bash
export TELEGRAM_BOT_TOKEN=your_telegram_bot_token
export WEBHOOK_PUBLIC_URL=https://your-domain.com/telegram/webhook
export OPENAI_API_KEY=your_openai_api_key
```

3. **Run with Docker Compose:**

```bash
# Development
docker-compose up --build

# Production (with nginx)
docker-compose --profile production up --build
```

### Using Makefile

1. **Install dependencies:**

```bash
make deps
```

2. **Build the application:**

```bash
make build
```

3. **Run in development mode:**

```bash
make dev
```

4. **Build for all platforms:**

```bash
make build-all
```

5. **Check environment variables:**

```bash
make check-env
```

### Manual Setup

#### Prerequisites

- Go 1.24.4 or later
- SQLite3
- OpenAI API key
- Telegram Bot Token

#### Environment Variables

Create a `.env` file or set these environment variables:

```bash
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
WEBHOOK_PUBLIC_URL=https://your-domain.com/telegram/webhook
OPENAI_API_KEY=your_openai_api_key
PORT=9095  # Optional, defaults to 9095
```

#### Installation

1. **Clone the repository:**

```bash
git clone <repository-url>
cd telegram-tradercoder-bot
```

2. **Install dependencies:**

```bash
go mod tidy
```

3. **Build the application:**

```bash
go build ./cmd/bot
```

4. **Run the bot:**

```bash
./bot
```

## Docker Deployment

### Development

```bash
# Build and run
docker-compose up --build

# Run in background
docker-compose up -d --build
```

### Production

```bash
# With nginx reverse proxy and SSL
docker-compose --profile production up -d --build
```

### Custom Docker Image

```bash
# Build image (uses linux/amd64 by default)
make docker-build

# Run container
make docker-run

# Push to registry
make docker-login   # first time only
make docker-push
```

## Makefile Commands

```bash
make help                    # Show all available commands
make build                   # Build the application
make build-all              # Build for multiple platforms
make run                    # Build and run
make dev                    # Run in development mode
make clean                  # Clean build artifacts
make test                   # Run tests
make test-coverage          # Run tests with coverage
make deps                   # Install dependencies
make deps-update            # Update dependencies
make fmt                    # Format code
make lint                   # Lint code
make docker-build           # Build Docker image
make docker-run             # Build and run Docker container
make install-tools          # Install development tools
make check-env              # Check environment variables
```

## Telegram Bot Setup

1. Create a new bot via [@BotFather](https://t.me/botfather) on Telegram
2. Get your bot token
3. Set up a webhook URL (must be HTTPS)
4. Configure the webhook URL in your environment variables

## Architecture

```
cmd/bot/
├── main.go              # Application entry point

internal/
├── config/
│   └── config.go        # Configuration management
├── finance/
│   └── yahoo.go         # Yahoo Finance chart generation
├── openai/
│   └── summarizer.go    # OpenAI API integration
├── server/
│   └── http.go          # HTTP server setup
├── storage/
│   └── sqlite.go        # SQLite database operations
└── telegram/
    ├── bot.go           # Telegram bot core
    └── handlers.go      # Message handlers

Docker/
├── Dockerfile           # Multi-stage Docker build
├── docker-compose.yml   # Development and production setup
├── nginx.conf          # Nginx configuration for production
└── .dockerignore       # Docker build optimization
```

## Database Schema

The bot uses SQLite to store chat messages for summarization:

```sql
CREATE TABLE messages (
    chat_id INTEGER,
    user_id INTEGER,
    text TEXT,
    ts INTEGER
);
```

## API Endpoints

- `POST /telegram/webhook` - Telegram webhook endpoint
- `GET /healthz` - Health check endpoint

## Dependencies

- `github.com/go-telegram-bot-api/telegram-bot-api/v5` - Telegram Bot API
- `github.com/openai/openai-go` - OpenAI API client
- `github.com/vicanso/go-charts/v2` - Chart generation
- `github.com/mattn/go-sqlite3` - SQLite driver

## Production Deployment

### With Docker Compose

```bash
# Set up SSL certificates
mkdir ssl
# Copy your SSL certificates to ssl/cert.pem and ssl/key.pem

# Deploy with nginx
docker-compose --profile production up -d --build
```

### With Kubernetes

```bash
# Apply Kubernetes manifests
kubectl apply -f k8s/
```

### Manual Server Deployment

1. Set up a VPS with Ubuntu/Debian
2. Install Docker and Docker Compose
3. Clone the repository
4. Set environment variables
5. Run `docker-compose --profile production up -d --build`

## Usage Examples

### Summarizing Chat Messages

Send `/summary` to get a summary of the last hour of messages, or `/summary 6` for the last 6 hours.

### Getting Stock Charts

Send `/stock AAPL` to get a 5-minute chart for Apple stock.

## Development

### Local Development

```bash
make dev
```

### Testing

```bash
make test
make test-coverage
```

### Code Quality

```bash
make fmt
make lint
```

### Building for Different Platforms

```bash
make build-all
# Creates binaries for Linux, macOS, and Windows
```

## Troubleshooting

### Common Issues

1. **Webhook URL must be HTTPS**: Use ngrok for development or set up SSL for production
2. **Database permissions**: Ensure the bot has write permissions to the data directory
3. **Environment variables**: Use `make check-env` to verify all required variables are set

### Logs

```bash
# Docker logs
docker-compose logs -f telegram-bot

# Application logs
./bot 2>&1 | tee bot.log
```

## License

This project is licensed under the MIT License.
