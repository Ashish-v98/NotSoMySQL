# AI Query Platform - Development Context

**Purpose**: This file provides quick context for continuing development. See [ARCHITECTURE.md](./ARCHITECTURE.md) for full details.

---

## Project Summary

Building an AI-powered database query and visualization platform as a SaaS solution. Teams deploy sidecars alongside their microservices in AWS ECS. Sidecars connect to team's RDS databases, use AWS Bedrock for natural language to SQL conversion, and communicate with a central parent service via gRPC. A single dashboard allows all teams to query their data and create visualizations.

**Key Innovation**: Hub-and-spoke architecture where sidecars handle AI+DB work locally, parent service aggregates for unified dashboard.

---

## Critical Architecture Decisions

### Technology Choices
- **Backend**: Go 1.23+ (both sidecar and parent service)
- **AI**: AWS Bedrock with Claude 3.5 Sonnet (NOT OpenAI/external APIs)
- **Communication**: gRPC between sidecar ↔ parent, REST for dashboard ↔ parent
- **Frontend**: React + TypeScript + Recharts/ECharts
- **Infrastructure**: AWS ECS Fargate, no Kubernetes
- **Databases**:
  - Client databases: RDS (Postgres/MySQL) - Phase 1, DynamoDB/MongoDB - Future
  - Platform storage: DynamoDB + Redis

### Why These Choices
- **Bedrock**: Stays in AWS, IAM auth, no API keys, compliance-friendly, VPC endpoints
- **Go**: Performance, small binaries, good for sidecars, excellent AWS SDK
- **gRPC**: Efficient binary protocol for service-to-service communication
- **Sidecar pattern**: Isolation (each team's sidecar only accesses their DB), zero impact on main apps

---

## Component Responsibilities

### Sidecar (per team/service)
- Load config from SSM Parameter Store
- Connect to team's database (RDS, DynamoDB, or MongoDB)
- Introspect schema/structure (cache for 24h)
- Call Bedrock for NL → Query conversion (SQL/PartiQL/MQL)
- Validate & execute query (read-only, timeouts, limits)
- Send results to parent via gRPC
- Register with parent on startup
- Send heartbeats every 30s

**Database Abstraction Layer**:
- Interface-based design supports multiple database types
- Database-specific implementations (SQL, DynamoDB, MongoDB)
- AI prompt selection based on database type

**Key Files to Create**:
```
sidecar/
├── cmd/sidecar/main.go              # Entry point
├── internal/
│   ├── config/loader.go             # SSM + Secrets Manager
│   ├── ai/
│   │   ├── bedrock.go               # Bedrock client
│   │   └── prompts.go               # DB-specific prompts
│   ├── database/
│   │   ├── interface.go             # Database interface
│   │   ├── postgres.go              # PostgreSQL implementation
│   │   ├── mysql.go                 # MySQL implementation
│   │   ├── dynamodb.go              # DynamoDB implementation (future)
│   │   ├── mongodb.go               # MongoDB implementation (future)
│   │   ├── factory.go               # Database factory
│   │   └── schema.go                # Schema introspection
│   ├── grpc/client.go               # gRPC to parent
│   └── query/processor.go           # Query pipeline
└── Dockerfile
```

### Parent Service (central hub)
- gRPC server for sidecars
- REST API for dashboard
- Registry of connected sidecars (DynamoDB)
- Cache query results (Redis, 1h TTL)
- Store dashboard configs (DynamoDB)
- Health monitoring of sidecars

**Key Files to Create**:
```
parent-service/
├── cmd/server/main.go
├── internal/
│   ├── grpc/server.go               # gRPC for sidecars
│   ├── api/rest.go                  # REST for dashboard
│   ├── registry/sidecars.go         # Track connected sidecars
│   └── storage/cache.go             # Redis operations
└── Dockerfile
```

### Dashboard (React SPA)
- Team selector
- Natural language query input
- Display results (tables, charts)
- Save/load dashboards
- Query history

---

## gRPC Protocol

**File**: `proto/aiquery/v1/service.proto`

**Key RPCs**:
```protobuf
service ParentService {
  rpc RegisterSidecar(RegisterRequest) returns (RegisterResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc ExecuteQuery(QueryRequest) returns (QueryResponse);
  rpc StreamQueryResults(QueryRequest) returns (stream QueryResultChunk);
  rpc GetSchema(SchemaRequest) returns (SchemaResponse);
  rpc SaveDashboard(SaveDashboardRequest) returns (SaveDashboardResponse);
}
```

**Flow**: Dashboard → Parent (REST) → Parent → Sidecar (gRPC) → RDS + Bedrock → Results → Parent → Dashboard

---

## AWS Services Configuration

### Bedrock
- **Model**: `anthropic.claude-3-5-sonnet-20241022-v2:0`
- **Fallback**: `anthropic.claude-3-haiku-20240307-v1:0` (cheaper)
- **Region**: us-east-1 (verify model availability)
- **IAM**: `bedrock:InvokeModel` permission

### SSM Parameter Store
```
/platform/parent-service/config
/teams/{team-id}/sidecar/config
/teams/{team-id}/database/host
/teams/{team-id}/database/name
```

### Secrets Manager
```json
{
  "username": "app_user",
  "password": "...",
  "engine": "postgres",
  "host": "team-a-db.xxx.rds.amazonaws.com",
  "port": 5432,
  "dbname": "production"
}
```

### ECS Configuration
- **Parent Service**: 2 vCPU, 4 GB RAM, awsvpc mode
- **Sidecar**: 0.5 vCPU, 1 GB RAM, awsvpc mode
- **Service Discovery**: AWS Cloud Map (ai-query-platform.local)

**Note**: ECS infrastructure, VPC, security groups, and networking assumed to be already configured.

### DynamoDB Tables

**Dashboards**:
- PK: `team_id#{dashboard_id}`
- SK: `version#{timestamp}`
- Attrs: name, widgets[], created_at

**SidecarRegistry**:
- PK: `sidecar_id`
- Attrs: team_id, service_name, status, last_seen, db_info
- GSI: team_id

**QueryHistory**:
- PK: `team_id`
- SK: `timestamp`
- Attrs: query, sql, execution_time, result_count, query_id

---

## Implementation Order

### Phase 1: MVP (Start Here)
**Goal**: Single team can query via NL, see table results

1. **Proto definitions** (1-2 days)
   - Write `service.proto`
   - Generate Go code: `protoc --go_out=. --go-grpc_out=. proto/aiquery/v1/service.proto`

2. **Sidecar basics** (3-4 days)
   - Config loading (SSM + Secrets Manager)
   - RDS connection with lib/pq (Postgres)
   - Schema introspection
   - Bedrock client (Claude 3.5 Sonnet)

3. **NL to SQL pipeline** (2-3 days)
   - Prompt engineering for Bedrock
   - SQL validation (use `github.com/xwb1989/sqlparser`)
   - Safe execution (timeouts, row limits)

4. **gRPC client in sidecar** (1-2 days)
   - Connect to parent
   - RegisterSidecar on startup
   - Heartbeat loop
   - Send query results

5. **Parent service basics** (3-4 days)
   - gRPC server
   - Sidecar registry (in-memory first, DynamoDB later)
   - Basic REST API
   - Health checks

6. **Dashboard MVP** (4-5 days)
   - React setup (Vite + TypeScript)
   - Query input
   - API client
   - Table display (basic HTML table or AG Grid)

7. **Deployment** (1-2 days)
   - Create Dockerfiles
   - Build container images
   - Create ECS task definition JSONs
   - Deploy to existing ECS cluster

**Success Criteria**: Can type "Show me all users", get SQL, see table of results

### Phase 2: Production Features (After MVP)
- Redis caching
- DynamoDB for persistence
- Chart visualizations
- Dashboard builder
- Monitoring & alerts

---

## Key Go Dependencies

```go
// Sidecar + Parent
github.com/aws/aws-sdk-go-v2/config
github.com/aws/aws-sdk-go-v2/service/bedrockruntime
github.com/aws/aws-sdk-go-v2/service/ssm
github.com/aws/aws-sdk-go-v2/service/secretsmanager
google.golang.org/grpc
google.golang.org/protobuf

// Sidecar specific - SQL databases
github.com/lib/pq                    // PostgreSQL
github.com/go-sql-driver/mysql       // MySQL
github.com/jmoiron/sqlx              // SQL helpers
github.com/xwb1989/sqlparser         // SQL validation

// Sidecar specific - NoSQL databases (future)
github.com/aws/aws-sdk-go-v2/service/dynamodb  // DynamoDB
github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression  // DynamoDB expressions
go.mongodb.org/mongo-driver/mongo    // MongoDB official driver

// Parent specific
github.com/gin-gonic/gin             // REST API
github.com/go-redis/redis/v8         // Redis client
github.com/aws/aws-sdk-go-v2/service/dynamodb
go.uber.org/zap                      // Logging
```

---

## Security Checklist

- [ ] Read-only DB users by default
- [ ] SQL validation before execution (no DROP/DELETE/UPDATE)
- [ ] Query timeout enforcement (30s default)
- [ ] Row limit enforcement (10k default)
- [ ] ECS task roles configured with required permissions (IAM auth automatic via AWS SDK)
- [ ] Database credentials in Secrets Manager
- [ ] TLS for all communications
- [ ] Network configuration allows Bedrock access (VPC endpoint or NAT gateway)
- [ ] Audit logging for all queries
- [ ] Team-based isolation (can't query other team's data)

---

## Bedrock Prompt Engineering

### SQL Databases (PostgreSQL/MySQL)

**System Prompt Template**:
```
You are a SQL expert for {DATABASE_TYPE}. Generate valid SQL queries from natural language.

Database Schema:
{SCHEMA_HERE}

Rules:
1. Return ONLY the SQL query, no explanations or markdown
2. Use proper {DATABASE_TYPE} syntax
3. Always use explicit JOINs (never implicit)
4. Add LIMIT clause (max 10000 rows)
5. Use table aliases for readability
6. Only SELECT queries (no mutations)
7. Use proper date/time functions
8. Handle NULL values appropriately
```

### DynamoDB (PartiQL)

**System Prompt Template**:
```
You are a DynamoDB expert. Generate valid PartiQL queries from natural language.

Table Structure:
{TABLE_SCHEMA_HERE}

Rules:
1. Return ONLY the PartiQL query, no explanations
2. Always consider partition key and sort key
3. Use proper WHERE clauses for key conditions
4. Account for DynamoDB's eventually consistent reads
5. Use quotes around table and attribute names
6. Example: SELECT * FROM "TableName" WHERE "PK" = 'value'
```

### MongoDB (MQL)

**System Prompt Template**:
```
You are a MongoDB expert. Generate valid MongoDB Query Language from natural language.

Collection Structure:
{COLLECTION_SCHEMA_HERE}

Rules:
1. Return ONLY the MongoDB query, no explanations
2. Use aggregation pipeline or find() as appropriate
3. Consider existing indexes
4. Use proper operators: $match, $group, $project, $sort, $limit
5. Return as JSON object
6. Example: { "find": { "status": "active" }, "limit": 10 }
```

**Response Parsing**:
- Extract query from response
- Remove markdown code blocks if present
- Validate syntax based on database type
- Check for forbidden operations (mutations)

---

## NoSQL Database Integration (Future)

### DynamoDB Specific

**Schema Introspection**:
```go
// Use DescribeTable API to get table structure
table := dynamodb.DescribeTable(tableName)
// Extract: PK, SK, GSIs, LSIs, attributes
```

**Query Execution**:
- Use PartiQL via ExecuteStatement API
- Or use native SDK operations (Query, Scan, GetItem)
- Handle pagination for large result sets
- Consider read capacity units (RCUs)

**Challenges**:
- No traditional schema, need to infer from data
- Partition key required for efficient queries
- Limited JOIN capabilities (denormalized data model)

### MongoDB Specific

**Schema Introspection**:
```go
// Use collection.Stats() and sample documents
collections := db.ListCollections()
for each collection {
    sample := collection.Find().Limit(100)
    // Infer schema from sample documents
}
```

**Query Execution**:
- Use aggregation pipeline for complex queries
- find() for simple queries
- Cursor-based pagination
- Consider index usage

**Challenges**:
- Flexible schema, need to sample documents
- Aggregation pipeline syntax is complex
- AI needs good examples for accurate query generation

---

## Common Issues & Solutions

### Issue: Bedrock generates invalid SQL
**Solution**:
- Include more schema details (column types, relationships)
- Add example queries in prompt
- Validate with sqlparser before execution
- Log failures and improve prompt iteratively

### Issue: Sidecar can't connect to parent
**Solution**:
- Check Cloud Map service discovery is working
- Verify security groups allow traffic on port 9090
- Check ECS task role has servicediscovery:DiscoverInstances
- Use parent service DNS name, not IP

### Issue: RDS connection timeout
**Solution**:
- Verify sidecar security group in RDS security group rules
- Check RDS is in same VPC
- Verify Secrets Manager has correct credentials
- Check connection string format

### Issue: Bedrock throttling
**Solution**:
- Implement exponential backoff
- Cache schema descriptions
- Use Haiku model for simple queries
- Request quota increase in AWS console

---

## Testing Strategy

### Unit Tests
- SQL validation logic
- Bedrock response parsing
- Schema introspection
- Config loading

### Integration Tests
- Sidecar → RDS connection
- Sidecar → Bedrock API
- Sidecar → Parent gRPC
- Parent → DynamoDB/Redis

### E2E Tests
- Dashboard → Parent → Sidecar → RDS → Bedrock
- Full query flow
- Dashboard creation and loading
- Multi-team isolation

### Load Tests
- 100 concurrent queries
- Parent service auto-scaling
- Redis cache performance
- Bedrock rate limit handling

---

## Monitoring & Observability

**Note**: CloudWatch infrastructure, alarms, and dashboards assumed to be configured. This section covers application metrics and logs to emit.

### Application Metrics to Emit
- Bedrock: invocations/min, latency (p50/p95/p99), errors, tokens used
- Queries: execution time, rows returned, cache hit ratio
- Sidecars: connected count, healthy count, query rate
- Parent: request rate, latency, error rate
- Database: connection pool utilization, query duration

### Application Logs to Emit (Structured JSON)
```json
{
  "timestamp": "2025-11-20T10:30:00Z",
  "level": "info",
  "component": "sidecar",
  "team_id": "team-payments",
  "query_id": "q-12345",
  "nl_query": "Show revenue by region",
  "generated_sql": "SELECT region, SUM(amount) ...",
  "execution_time_ms": 234,
  "rows_returned": 5,
  "bedrock_model": "claude-3-5-sonnet",
  "cached": false
}
```

---

## Project File Structure

```
notsoMySQL/
├── ARCHITECTURE.md              # Full architecture (this exists)
├── PROJECT_CONTEXT.md           # This file
├── README.md                    # User-facing documentation (TODO)
├── proto/
│   └── aiquery/v1/
│       └── service.proto        # gRPC definitions (TODO)
├── sidecar/                     # Sidecar service (TODO)
│   ├── cmd/sidecar/main.go
│   ├── internal/...
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── parent-service/              # Parent service (TODO)
│   ├── cmd/server/main.go
│   ├── internal/...
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── dashboard/                   # React dashboard (TODO)
│   ├── src/
│   ├── package.json
│   └── vite.config.ts
├── deployment/                  # Deployment configs (TODO)
│   ├── ecs-task-definitions/
│   └── config-examples/
└── docs/                        # Additional documentation (TODO)
    ├── API.md
    └── USAGE.md
```

---

## Next Steps for New Chat

1. **If starting Phase 1**:
   - Create proto definition first
   - Generate Go code from proto
   - Set up Go modules for sidecar and parent
   - Start with sidecar AWS integrations (SSM, Secrets Manager, RDS)

2. **If continuing existing work**:
   - Check what files exist in the repo
   - Review any TODOs in code
   - Check terraform state if infrastructure deployed
   - Review any open issues or blockers

3. **If user says "continue" or "keep going"**:
   - Check last modified files to understand context
   - Ask if they want to continue where left off or pivot
   - Propose next logical step based on phase

4. **Always ask first**:
   - What phase are we in?
   - What's already been built?
   - Any blockers or issues?
   - Any architecture changes since ARCHITECTURE.md was written?

---

## Quick Reference Commands

**Generate proto code**:
```bash
protoc --go_out=. --go-grpc_out=. proto/aiquery/v1/service.proto
```

**Run sidecar locally**:
```bash
cd sidecar
AWS_PROFILE=dev go run cmd/sidecar/main.go
```

**Build Docker image**:
```bash
docker build -t ai-query-sidecar:latest -f sidecar/Dockerfile .
```

**Deploy to ECS** (using existing infrastructure):
```bash
# Update ECS task definition
aws ecs register-task-definition --cli-input-json file://deployment/ecs-task-definitions/sidecar.json

# Update service
aws ecs update-service --cluster my-cluster --service my-sidecar --task-definition sidecar:latest
```

**Test Bedrock locally**:
```bash
aws bedrock-runtime invoke-model \
  --model-id anthropic.claude-3-5-sonnet-20241022-v2:0 \
  --body '{"anthropic_version":"bedrock-2023-05-31","max_tokens":1000,"messages":[{"role":"user","content":"SELECT * FROM users LIMIT 10;"}]}' \
  --cli-binary-format raw-in-base64-out \
  output.json
```

---

**Last Updated**: 2025-11-20
**Current Phase**: Planning (no code written yet)
**Status**: Ready to start Phase 1 implementation
**Architecture Version**: 1.2 (NoSQL support + infrastructure assumptions clarified)

**Important Notes**:
- IAM authentication handled automatically by AWS SDK - no manual implementation needed
- Infrastructure (VPC, Terraform, ECS, CloudWatch) assumed to be already configured
- Focus is on application code only
