# AI Query Platform

AI-powered database query and visualization platform built as a SaaS solution. Teams deploy sidecars alongside their microservices, which connect to databases and use AI (AWS Bedrock or Ollama) for natural language to SQL conversion.

## Architecture

```
Dashboard (React) --> Parent Service (REST) --> Sidecar (gRPC) --> Database + AI
     :3000              :8090/:9090              :8080           MySQL + Bedrock/Ollama
```

## Components

| Component | Port | Description |
|-----------|------|-------------|
| Dashboard | 3000 | React frontend for querying data |
| Parent Service | 8090 (REST), 9090 (gRPC) | Central hub, routes queries to sidecars |
| Sidecar | 8080 (gRPC) | Connects to DB, generates SQL via AI, uploads large results to S3 |
| MySQL | 3306 | Sample database |
| phpMyAdmin | 8081 | Database admin UI |
| LocalStack | 4566 | Local AWS S3 emulator for development |

## Quick Start

### Option 1: Docker Compose (Recommended)

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f
```

Services available at:
- **Dashboard**: http://localhost:3000
- **Parent API**: http://localhost:8090/api/v1/health
- **Sidecar** (gRPC only): localhost:8080
- **phpMyAdmin**: http://localhost:8081
- **LocalStack S3**: http://localhost:4566

### Option 2: Local Development

```bash
# Terminal 1: Start infrastructure (MySQL, LocalStack)
docker-compose up -d mysql phpmyadmin localstack localstack-init

# Terminal 2: Start Parent Service
cd parent-service
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
go run cmd/server/main.go

# Terminal 3: Start Sidecar
cd sidecar
# Use config.yaml for main config, or override with env vars:
export PARENT_ENABLED=true
export PARENT_ADDRESS=localhost:9090
export AI_PROVIDER=ollama
export OLLAMA_URL=http://localhost:11434
export AWS_ENDPOINT_URL=http://localhost:4566
export S3_BUCKET=ai-query-results
go run cmd/sidecar/main.go

# Terminal 4: Start Dashboard
cd dashboard
npm install
npm run dev
```

## Prerequisites

### 1. Install Go (Version 1.23+)

**Windows:**
```bash
# Download and install from: https://go.dev/dl/
# Or use Chocolatey:
choco install golang

# Verify installation:
go version
```

**macOS:**
```bash
brew install go
go version
```

**Linux:**
```bash
# Download and install
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
go version
```

### 2. Install Docker

**Windows:**
- Download Docker Desktop from: https://www.docker.com/products/docker-desktop

**macOS:**
```bash
brew install --cask docker
```

**Linux:**
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install docker.io docker-compose

# Start Docker
sudo systemctl start docker
sudo systemctl enable docker
```

### 3. AWS Credentials Setup

You need AWS credentials with access to Bedrock.

**Option A: Use AWS CLI (Recommended)**
```bash
# Install AWS CLI
# Windows: Download from https://aws.amazon.com/cli/
# macOS: brew install awscli
# Linux: sudo apt-get install awscli

# Configure AWS CLI
aws configure
# Enter your:
# - AWS Access Key ID
# - AWS Secret Access Key
# - Default region (us-east-1 for Bedrock)
# - Output format (json)
```

**Option B: Manual .env File**
```bash
# Copy example file
cp .env.example .env

# Edit .env with your credentials
# Get credentials from AWS Console > IAM > Users > Security Credentials
```

### 4. Enable AWS Bedrock Access

1. Go to AWS Console > Bedrock
2. Navigate to "Model access"
3. Request access to Claude 3.5 Sonnet
4. Wait for approval (usually instant)

## Quick Start

### 1. Clone and Setup

```bash
cd notsoMySQL

# Copy environment variables
cp .env.example .env

# Edit .env with your AWS credentials
notepad .env  # Windows
nano .env     # Linux/macOS
```

### 2. Start Services with Docker Compose

```bash
# Start MySQL, phpMyAdmin, and the sidecar
docker-compose up -d

# View logs
docker-compose logs -f sidecar
```

Services will be available at:
- **Sidecar API**: http://localhost:8080
- **phpMyAdmin**: http://localhost:8081 (user: root, password: password)
- **MySQL**: localhost:3306

### 3. Test the Health Endpoint

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "healthy",
  "time": "2025-11-20T10:30:00Z"
}
```

### 4. Test Natural Language Query

```bash
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{"query": "Show me all customers"}'
```

Expected response:
```json
{
  "generated_sql": "SELECT * FROM customers LIMIT 1000",
  "results": {
    "columns": ["id", "name", "email", "created_at"],
    "rows": [
      {"id": "1", "name": "John Doe", "email": "john@example.com", ...},
      ...
    ],
    "count": 5
  }
}
```

### More Example Queries

```bash
# Get total revenue
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What is the total revenue from all orders?"}'

# Get top customers
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{"query": "Show me the top 3 customers by total order value"}'

# Get product inventory
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{"query": "Which products have less than 50 items in stock?"}'
```

## Local Development (Without Docker)

### 1. Install Go Dependencies

```bash
cd sidecar
go mod download
```

### 2. Start MySQL Locally

```bash
# Using Docker for just MySQL
docker run -d \
  --name mysql-dev \
  -e MYSQL_ROOT_PASSWORD=password \
  -e MYSQL_DATABASE=testdb \
  -p 3306:3306 \
  mysql:8.0

# Initialize database
docker exec -i mysql-dev mysql -uroot -ppassword testdb < ../init-db.sql
```

### 3. Run Sidecar Locally

```bash
cd sidecar

# Set environment variables
export DB_HOST=localhost
export DB_PORT=3306
export DB_USER=root
export DB_PASSWORD=password
export DB_NAME=testdb
export AWS_REGION=us-east-1
export BEDROCK_MODEL_ID=anthropic.claude-3-5-sonnet-20241022-v2:0
export SERVER_PORT=8080

# Run the application
go run cmd/sidecar/main.go
```

## Project Structure

```
notsoMySQL/
├── sidecar/
│   ├── cmd/
│   │   └── sidecar/
│   │       └── main.go           # Entry point
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go         # Configuration management
│   │   ├── database/
│   │   │   └── mysql.go          # Database connection and queries
│   │   └── ai/
│   │       └── bedrock.go        # Bedrock AI client
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── docker-compose.yml             # Docker services setup
├── init-db.sql                    # Sample database schema
└── README.md                      # This file
```

## Troubleshooting

### "go: command not found"
- Install Go from https://go.dev/dl/
- Add Go to your PATH

### "docker: command not found"
- Install Docker Desktop from https://www.docker.com/products/docker-desktop

### "failed to invoke model: AccessDeniedException"
- Check AWS credentials in .env file
- Ensure Bedrock model access is enabled in AWS Console
- Verify your IAM user has `bedrock:InvokeModel` permission

### "failed to connect to database"
- Wait 10-15 seconds for MySQL to fully start
- Check logs: `docker-compose logs mysql`
- Verify MySQL is running: `docker-compose ps`

### MySQL container fails to start
- Check if port 3306 is already in use
- Change port in docker-compose.yml: `"3307:3306"`

## Stopping Services

```bash
# Stop all services
docker-compose down

# Stop and remove volumes (deletes database data)
docker-compose down -v
```

## Next Steps

1. **Add gRPC support** - For parent service communication
2. **Implement query caching** - Using Redis
3. **Add query validation** - Using SQL parser
4. **Implement schema caching** - To reduce database calls
5. **Add metrics** - CloudWatch integration
6. **Deploy to ECS** - Using existing infrastructure

## API Documentation

### POST /query

Execute a natural language query.

**Request:**
```json
{
  "query": "Show me all orders from last week"
}
```

**Response:**
```json
{
  "generated_sql": "SELECT * FROM orders WHERE order_date >= DATE_SUB(NOW(), INTERVAL 7 DAY) LIMIT 1000",
  "results": {
    "columns": ["id", "customer_id", "total", "status", "order_date"],
    "rows": [...],
    "count": 15
  }
}
```

**Error Response:**
```json
{
  "generated_sql": "SELECT ...",
  "error": "Query execution failed: Unknown column 'xyz'"
}
```

### GET /health

Check service health.

**Response:**
```json
{
  "status": "healthy",
  "time": "2025-11-20T10:30:00Z"
}
```

## Cost Considerations

- **Bedrock Claude 3.5 Sonnet**: ~$0.01-0.03 per query
- Consider using Claude 3 Haiku for simpler queries (90% cheaper)
- Implement caching to reduce repeated queries

## Security Notes

- Only SELECT queries are allowed (mutations blocked)
- Query timeout: 30 seconds
- Row limit: 1000 (configurable)
- Use read-only database user in production
- Never commit .env file with real credentials
