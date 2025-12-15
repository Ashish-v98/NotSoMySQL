# Startup Guide - AI Query Platform

Complete step-by-step instructions to start all services and the dashboard.

## Prerequisites

Before starting, ensure you have the following installed:

- **Podman Desktop** (or Docker Desktop)
- **Ollama** - https://ollama.ai
- **Go 1.23+** (for local development only)
- **Node.js 18+** (for local development only)

---

## Step 1: Start Podman Machine

Open PowerShell or Command Prompt and start the Podman machine:

```bash
podman machine start
```

Wait for the message confirming the machine is running. You can verify with:

```bash
podman machine list
```

You should see `Currently running` status.

---

## Step 2: Start Ollama

### Option A: Start Ollama Service (Windows)

If Ollama is installed as a Windows application:

1. Open Ollama from Start Menu, OR
2. Run from PowerShell:
   ```bash
   ollama serve
   ```

### Option B: If Ollama is Already Running

Check if Ollama is running:

```bash
curl http://localhost:11434/api/tags
```

If you get a response, Ollama is running.

### Pull Required Model

Ensure you have the required model downloaded:

```bash
ollama pull llama3.2
```

Or if using a different model, pull that instead:

```bash
ollama pull codellama
ollama pull mistral
```

---

## Step 3: Navigate to Project Directory

```bash
cd C:\Users\Admin\source\repos\notsoMySQL
```

---

## Step 4: Build All Docker Images

Build all services using Podman Compose:

```bash
podman compose build
```

This will build:
- `mysql` - MySQL 8.0 database
- `phpmyadmin` - Database admin UI
- `sidecar` - AI query sidecar service
- `parent-service` - Central hub service
- `dashboard` - React frontend

Wait for all images to build successfully.

---

## Step 5: Start All Services

Start all services in detached mode:

```bash
podman compose up -d
```

This starts the services in the following order (due to dependencies):
1. MySQL (port 3306)
2. phpMyAdmin (port 8081)
3. Parent Service (ports 8090, 9090)
4. Sidecar (port 8080)
5. Dashboard (port 3000)

---

## Step 6: Wait for MySQL to Initialize

MySQL takes about 15-30 seconds to fully initialize. Check logs:

```bash
podman compose logs -f mysql
```

Wait until you see: `ready for connections`

Press `Ctrl+C` to exit log view.

---

## Step 7: Verify All Services Are Running

Check container status:

```bash
podman compose ps
```

All containers should show `Up` status.

---

## Step 8: Verify Health Endpoints

### Check Sidecar Health

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "healthy",
  "time": "2025-11-22T...",
  "ai_provider": "ollama",
  "ai_model": "llama3.2",
  "team_id": "team-alpha",
  "service": "ecommerce-db",
  "parent_connected": true,
  "sidecar_id": "..."
}
```

**Important**: `parent_connected` should be `true`. If it's `false`, see Troubleshooting section.

### Check Parent Service Health

```bash
curl http://localhost:8090/api/v1/health
```

Expected response:
```json
{
  "status": "healthy",
  "sidecars_count": 1,
  "teams_count": 1,
  "time": "2025-11-22T..."
}
```

**Important**: `sidecars_count` should be `1` (or more if multiple sidecars).

### Check Dashboard Health

```bash
curl http://localhost:3000
```

Should return HTML content.

---

## Step 9: Access the Dashboard

Open your web browser and navigate to:

```
http://localhost:3000
```

You should see:
1. **Connected** status in the header (green indicator)
2. **Team selector** dropdown showing "team-alpha"
3. **Query input** text area
4. **Execute** button

---

## Step 10: Test a Query

1. Select team "team-alpha" from dropdown
2. Enter a query like: `Show me all customers`
3. Click **Execute**
4. You should see:
   - Generated SQL query
   - Results table with data

---

## Stopping Services

To stop all services:

```bash
podman compose down
```

To stop and remove all data (including database):

```bash
podman compose down -v
```

---

## Restarting Services

If you need to restart after stopping:

```bash
# Start Podman machine first
podman machine start

# Start Ollama (if not running)
ollama serve

# Start all services
cd C:\Users\Admin\source\repos\notsoMySQL
podman compose up -d
```

---

## Viewing Logs

### All Services
```bash
podman compose logs -f
```

### Specific Service
```bash
podman compose logs -f sidecar
podman compose logs -f parent-service
podman compose logs -f dashboard
podman compose logs -f mysql
```

---

## Troubleshooting

### Dashboard Shows "Disconnected"

1. Check if parent-service is running:
   ```bash
   podman compose ps parent-service
   ```

2. Check parent-service logs:
   ```bash
   podman compose logs parent-service
   ```

3. Restart the dashboard:
   ```bash
   podman compose restart dashboard
   ```

### Sidecar Shows `parent_connected: false`

1. Check if parent-service gRPC is listening:
   ```bash
   podman compose logs parent-service | grep "gRPC"
   ```

2. Restart sidecar after parent is running:
   ```bash
   podman compose restart sidecar
   ```

3. If still not working, restart both:
   ```bash
   podman compose restart parent-service
   # Wait 5 seconds
   podman compose restart sidecar
   ```

### Query Returns Error "dial tcp: lookup..."

This happens when container hostnames are stale. Restart both services:

```bash
podman compose restart parent-service sidecar
```

### Dashboard Shows Multiple Sidecars

This can happen if sidecar restarts with a new ID. Restart parent to clear registry:

```bash
podman compose restart parent-service
# Wait 5 seconds
podman compose restart sidecar
```

### MySQL Connection Refused

MySQL may not be ready yet. Wait 30 seconds and try again, or check logs:

```bash
podman compose logs mysql
```

### Ollama Connection Failed

1. Verify Ollama is running:
   ```bash
   curl http://localhost:11434/api/tags
   ```

2. Check if required model is available:
   ```bash
   ollama list
   ```

3. Pull the model if missing:
   ```bash
   ollama pull llama3.2
   ```

### Query Fails Multiple Times

The system has 3-retry logic. If all 3 fail:
1. Try simplifying the query
2. Check the generated SQL in the error message
3. Verify table/column names exist in the database

### Cannot Access phpMyAdmin

Navigate to http://localhost:8081
- Username: `root`
- Password: `password`

---

## Environment Variables

You can customize behavior with environment variables in a `.env` file or by exporting them:

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_TYPE` | mysql | Database type (mysql - more types coming) |
| `AI_PROVIDER` | ollama | AI provider (ollama, bedrock) |
| `AI_MODEL_ID` | qwen2.5-coder:7b | Model to use for SQL generation |
| `TEAM_ID` | demo-team | Team identifier for sidecar |
| `SERVICE_NAME` | demo-service | Service name for sidecar |

Example with custom settings:
```bash
AI_MODEL_ID=llama3.2 TEAM_ID=my-team podman compose up --build -d
```

---

## Service Ports Summary

| Service | Port | URL |
|---------|------|-----|
| Dashboard | 3000 | http://localhost:3000 |
| Parent Service (REST) | 8090 | http://localhost:8090/api/v1/health |
| Parent Service (gRPC) | 9090 | - |
| Sidecar | 8080 | http://localhost:8080/health |
| phpMyAdmin | 8081 | http://localhost:8081 |
| MySQL | 3306 | localhost:3306 |
| Ollama | 11434 | http://localhost:11434 |

---

## Quick Start Commands Summary

```bash
# 1. Start Podman
podman machine start

# 2. Start Ollama (in separate terminal or background)
ollama serve

# 3. Navigate to project
cd C:\Users\Admin\source\repos\notsoMySQL

# 4. Build and start all services
podman compose build
podman compose up -d

# 5. Verify
podman compose ps
curl http://localhost:8080/health
curl http://localhost:8090/api/v1/health

# 6. Open dashboard
start http://localhost:3000
```

---

## Example Queries to Try

After starting all services, try these queries in the dashboard:

1. **Simple**: `Show me all customers`

2. **Filtered**: `Show orders from the last 30 days`

3. **Aggregation**: `What is the total revenue by product category?`

4. **Complex**: `Show me top 5 customers by total order value with their email addresses`

5. **Join**: `List all products with their category names and supplier information`

6. **Analytics**: `Show average order value per customer who has placed more than 2 orders`
