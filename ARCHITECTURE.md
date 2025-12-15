# AI Query Platform - Architecture & Planning Document

## Executive Summary

An AI-powered database query and dashboard platform built as a SaaS solution using AWS services. The platform uses a hub-and-spoke architecture with sidecars deployed alongside microservices in ECS, communicating with a central parent service. Teams access a unified dashboard to query their databases using natural language and visualize data.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Components](#components)
3. [Technology Stack](#technology-stack)
4. [Data Flow](#data-flow)
5. [AWS Services Integration](#aws-services-integration)
6. [Security & Compliance](#security--compliance)
7. [Deployment Strategy](#deployment-strategy)
8. [Scalability & Performance](#scalability--performance)
9. [Cost Estimation](#cost-estimation)
10. [Implementation Phases](#implementation-phases)
11. [Risk Mitigation](#risk-mitigation)

---

## Architecture Overview

### High-Level Architecture

```
┌──────────────────────────────────────────────────┐
│          Single Dashboard (React)                │
│     All teams access their dashboards here       │
│                                                  │
│  Features:                                       │
│  - Team selector                                 │
│  - Natural language query interface              │
│  - Dynamic chart/graph generation                │
│  - Saved dashboards                              │
│  - Query history                                 │
└───────────────────┬──────────────────────────────┘
                    │
                    │ HTTPS / REST API
                    │
                    │
┌───────────────────▼──────────────────────────────┐
│          Parent Service (ECS Fargate)            │
│                                                  │
│  Responsibilities:                               │
│  - REST API for dashboard                        │
│  - gRPC server for sidecars                      │
│  - Team authentication & authorization           │
│  - Sidecar registry & health monitoring          │
│  - Query result caching (Redis)                  │
│  - Dashboard configuration storage (DynamoDB)    │
│  - Metrics & logging aggregation                 │
└──────┬────────────────┬────────────────┬─────────┘
       │                │                │
       │ gRPC           │ gRPC           │ gRPC
       │ (Service       │ (Service       │ (Service
       │  Discovery)    │  Discovery)    │  Discovery)
       │                │                │
┌──────▼──────┐  ┌──────▼──────┐  ┌─────▼───────┐
│ ECS Task A  │  │ ECS Task B  │  │ ECS Task C  │
│ (Team A)    │  │ (Team B)    │  │ (Team C)    │
├─────────────┤  ├─────────────┤  ├─────────────┤
│ Main App    │  │ Main App    │  │ Main App    │
│ Container   │  │ Container   │  │ Container   │
├─────────────┤  ├─────────────┤  ├─────────────┤
│  Sidecar    │  │  Sidecar    │  │  Sidecar    │
│  Container  │  │  Container  │  │  Container  │
│             │  │             │  │             │
│  - Bedrock  │  │  - Bedrock  │  │  - Bedrock  │
│  - gRPC     │  │  - gRPC     │  │  - gRPC     │
│  - RDS      │  │  - RDS      │  │  - RDS      │
│    Client   │  │    Client   │  │    Client   │
└──────┬──────┘  └──────┬──────┘  └──────┬──────┘
       │                │                │
       │ SQL            │ SQL            │ SQL
       │                │                │
┌──────▼──────┐  ┌──────▼──────┐  ┌─────▼───────┐
│   RDS A     │  │   RDS B     │  │   RDS C     │
│  (Team A)   │  │  (Team B)   │  │  (Team C)   │
│  Postgres   │  │   MySQL     │  │  Postgres   │
└─────────────┘  └─────────────┘  └─────────────┘
```

### Design Principles

1. **Isolation**: Each team's sidecar only accesses their own database
2. **Zero-trust**: Sidecars authenticate with parent service, IAM roles for AWS services
3. **Scalability**: Sidecars scale with their services, parent service scales independently
4. **Observability**: Comprehensive logging and metrics at all layers
5. **Fault tolerance**: Graceful degradation when sidecars are unavailable
6. **Cost efficiency**: Pay-per-use for AI (Bedrock), efficient caching
7. **Database Agnostic**: Abstraction layer supports SQL and NoSQL databases

---

## Database Abstraction Architecture

To support both SQL and NoSQL databases, the sidecar implements a database abstraction layer:

### Database Interface

```go
type Database interface {
    Connect(ctx context.Context, config DatabaseConfig) error
    Disconnect() error
    GetSchema(ctx context.Context) (*Schema, error)
    ExecuteQuery(ctx context.Context, query string) (*QueryResult, error)
    GetDialect() DatabaseDialect
    ValidateQuery(query string) error
}
```

### Supported Database Types

| Database Type | Query Language | Implementation Status | Driver/SDK |
|---------------|----------------|----------------------|------------|
| PostgreSQL    | SQL            | ✅ Phase 1           | lib/pq or pgx |
| MySQL         | SQL            | ✅ Phase 1           | go-sql-driver/mysql |
| DynamoDB      | PartiQL        | 🔄 Future            | AWS SDK DynamoDB |
| MongoDB       | MQL            | 🔄 Future            | mongo-driver |
| Aurora        | SQL            | ✅ Phase 1           | PostgreSQL/MySQL compatible |

### Database Detection & Configuration

The sidecar automatically detects database type from configuration:

```yaml
database:
  type: "postgres"  # postgres, mysql, dynamodb, mongodb
  # For SQL databases (RDS)
  connection_string_ssm: "/team-a/database/connection"

  # For DynamoDB
  table_prefix: "team-a-"
  region: "us-east-1"

  # For MongoDB
  connection_uri_secret: "team-a/mongodb/uri"
  database_name: "production"
```

### AI Prompt Selection

Based on database type, sidecar selects appropriate AI prompt template:
- **SQL databases**: SQL-focused prompt with schema tables/columns
- **DynamoDB**: PartiQL prompt with table structure and keys
- **MongoDB**: MQL prompt with collection structure and indexes

### Implementation Strategy for NoSQL

**Phase 1 (Current)**:
- Build with database interface from the start
- Implement PostgreSQL and MySQL

**Phase 2 (Future)**:
- Implement DynamoDB adapter
  - Use DescribeTable for schema introspection
  - Generate PartiQL queries
  - Handle DynamoDB-specific limits and pagination
- Implement MongoDB adapter
  - Sample documents for schema inference
  - Generate MQL/aggregation pipeline
  - Handle MongoDB-specific operators

**Benefits of This Approach**:
- Adding new databases requires only implementing the Database interface
- No changes to parent service or dashboard
- Each database type can have optimized query generation
- Team-specific sidecar configs specify database type

---

## Components

### 1. Sidecar Service

**Purpose**: Deployed alongside each microservice to provide AI-powered query capabilities for that service's database.

**Responsibilities**:
- Connect to team's RDS database
- Introspect database schema
- Communicate with AWS Bedrock/Ollama for natural language to SQL conversion
- Execute SQL queries safely with validation
- Upload large query results to S3 (>1MB)
- Return results inline or S3 path via gRPC
- Expose gRPC service for parent to call
- Maintain heartbeat with parent service
- Load configuration from config.yaml (Viper)
- Support LocalStack for local S3 development

**Key Features**:
- Read-only query execution by default (configurable)
- Query timeout protection
- Row limit enforcement
- SQL injection prevention
- Connection pooling
- Automatic retry logic
- Health check endpoints

**Configuration Sources**:
- SSM Parameter Store: `/team-{id}/sidecar/config`
- Secrets Manager: Database credentials (for RDS/MongoDB)
- Environment variables: Team ID, service name, parent service address
- config.yaml: Default settings (includes database type detection)

**Supported Database Types**:
- **Phase 1**: PostgreSQL, MySQL (RDS)
- **Future**: DynamoDB, MongoDB, Aurora Serverless

**Database Type Configuration**:
The sidecar uses a database abstraction layer allowing seamless support for:
- **SQL databases**: Traditional relational databases via standard SQL
- **NoSQL databases**: Document stores (MongoDB) and key-value (DynamoDB)
- **Hybrid queries**: Future support for cross-database queries with proper isolation

### 2. Parent Service

**Purpose**: Central hub that aggregates data from all sidecars and provides unified API for dashboard.

**Responsibilities**:
- Provide REST API for dashboard consumption
- Run gRPC server to receive data from sidecars
- Maintain registry of active sidecars
- Monitor sidecar health via heartbeats
- Cache query results (Redis)
- Store dashboard configurations (DynamoDB)
- Rate limiting per team
- Audit logging
- Metrics collection and aggregation

**API Endpoints**:
```
# Dashboard API (REST)
GET    /api/v1/teams                           # List all teams
GET    /api/v1/teams/{teamId}/services         # Get team's services
POST   /api/v1/teams/{teamId}/query            # Execute query
GET    /api/v1/teams/{teamId}/schema           # Get DB schema
GET    /api/v1/teams/{teamId}/dashboards       # List dashboards
POST   /api/v1/teams/{teamId}/dashboards       # Save dashboard
GET    /api/v1/teams/{teamId}/dashboards/{id}  # Get dashboard
PUT    /api/v1/teams/{teamId}/dashboards/{id}  # Update dashboard
DELETE /api/v1/teams/{teamId}/dashboards/{id}  # Delete dashboard
GET    /api/v1/teams/{teamId}/history          # Query history
GET    /api/v1/query/{queryId}/status          # Query status
GET    /api/v1/query/{queryId}/result          # Get result
GET    /api/v1/health                          # Health check
GET    /api/v1/metrics                         # Platform metrics
```

**gRPC Server Interface**:
```
RegisterSidecar()        # Sidecar registration
Heartbeat()              # Keep-alive
ExecuteQuery()           # Execute and return results
StreamQueryResults()     # Stream large result sets
GetSchema()              # Get database schema
SaveDashboard()          # Save dashboard config
UnregisterSidecar()      # Graceful shutdown
```

### 3. Dashboard UI

**Purpose**: Single-page application for all teams to access their data.

**Features**:
- Team selector dropdown
- Service selector (for teams with multiple services)
- Natural language query input with autocomplete
- Query history with favorites
- Automatic chart type suggestion
- Manual visualization selection (bar, line, pie, table, metric cards)
- Dashboard builder (drag-and-drop widgets)
- Dashboard templates library
- Export capabilities (CSV, JSON, PDF)
- Share dashboards (read-only links)
- Real-time query status
- Schema browser/explorer
- Query explanation (show generated SQL)

**User Experience Flow**:
1. Select team from dropdown
2. Type natural language query: "Show top 10 customers by revenue this month"
3. System generates SQL and shows preview
4. User confirms or modifies
5. Results displayed with auto-suggested chart type
6. User can adjust visualization, save to dashboard

**Technology**:
- React 18+ with TypeScript
- Recharts / Apache ECharts for visualizations
- TanStack Query for data fetching
- Zustand for state management
- TailwindCSS for styling
- Vite for build tooling

---

## Technology Stack

### Backend Services

#### Sidecar
- **Language**: Go 1.23+
- **Framework**: None (lightweight, standard library)
- **gRPC**: google.golang.org/grpc
- **AWS SDK**: aws-sdk-go-v2
  - bedrock-runtime
  - ssm
  - secretsmanager
  - rds
  - dynamodb (for NoSQL support)
- **Database Drivers**:
  - **SQL Databases**:
    - lib/pq (PostgreSQL)
    - go-sql-driver/mysql (MySQL)
    - jackc/pgx (PostgreSQL alternative, better performance)
  - **NoSQL Databases** (Future):
    - AWS SDK DynamoDB client
    - mongo-driver/mongo (MongoDB official driver)
- **Query Parsing/Validation**:
  - xwb1989/sqlparser (SQL)
  - Custom validators for PartiQL/NoSQL
- **Configuration**: viper (for config management)
- **Logging**: zap or zerolog

#### Parent Service
- **Language**: Go 1.23+
- **Web Framework**: Gin or Fiber
- **gRPC**: google.golang.org/grpc
- **Cache**: go-redis/redis
- **Database**: AWS SDK (DynamoDB)
- **Metrics**: Prometheus client
- **Logging**: zap with structured logging

### Frontend
- **Framework**: React 18+ with TypeScript
- **Build Tool**: Vite
- **Charts**: Recharts, Apache ECharts, or Plotly.js
- **UI Components**: shadcn/ui or Ant Design
- **HTTP Client**: Axios or native fetch
- **State**: Zustand or Redux Toolkit
- **Routing**: React Router v6
- **Forms**: React Hook Form
- **Styling**: TailwindCSS

### Infrastructure
- **Container Orchestration**: AWS ECS Fargate
- **Service Discovery**: AWS Cloud Map
- **Load Balancer**: Application Load Balancer
- **CDN**: CloudFront (for dashboard)
- **DNS**: Route 53
- **IaC**: Terraform or AWS CDK
- **CI/CD**: GitHub Actions or AWS CodePipeline

### Data Storage
- **Application Database**: DynamoDB (dashboard configs, metadata)
- **Cache**: ElastiCache for Redis
- **Client Databases**: RDS (PostgreSQL, MySQL, Aurora)

### AI/ML
- **Primary**: AWS Bedrock
  - Model: Claude 3.5 Sonnet (best for SQL)
  - Fallback: Claude 3 Haiku (cheaper, faster)
  - Alternative: Amazon Titan Text Premier

### Monitoring & Observability
- **Metrics**: CloudWatch + Prometheus
- **Logging**: CloudWatch Logs
- **Tracing**: AWS X-Ray
- **Alerting**: CloudWatch Alarms + SNS
- **Dashboards**: CloudWatch Dashboards + Grafana

---

## Data Flow

### Query Execution Flow

1. **User Input** (Dashboard)
   - User types: "Show revenue by region for last quarter"
   - Dashboard sends POST to `/api/v1/teams/team-sales/query`

2. **Parent Service** (Receives Request)
   - Validates request
   - Checks rate limits
   - Generates unique query ID
   - Looks up registered sidecar for team-sales
   - Forwards via gRPC to sidecar

3. **Sidecar** (Processes Query)
   - Receives gRPC request from parent
   - Fetches database schema (cached)
   - Constructs AI prompt with schema context (Bedrock/Ollama)
   - Calls AI service with retry logic (max 3 attempts)
   - Receives generated SQL
   - Validates SQL (syntax, SELECT-only, no mutations)
   - Executes SQL on database with timeout and row limit
   - Checks result size:
     - **Small (<1MB)**: Returns inline via gRPC
     - **Large (>1MB)**: Uploads to S3, returns S3 path via gRPC
   - Returns SidecarQueryResponse to parent

4. **Parent Service** (Returns Results)
   - Receives SidecarQueryResponse from sidecar
   - Checks result location:
     - **Inline**: Use results directly
     - **S3 Path**: Download from S3 using S3 client
   - Caches results in Redis (1 hour TTL)
   - Converts to REST API format
   - Returns to dashboard with metadata

5. **Dashboard** (Displays Results)
   - Renders data in suggested chart type
   - User can change visualization
   - User can save to dashboard
   - User can export data

### Sidecar Registration Flow

1. **Sidecar Starts**
   - Loads config from config.yaml (Viper)
   - Connects to database (MySQL/PostgreSQL)
   - Initializes AI client (Ollama or Bedrock)
   - Initializes S3 client (AWS S3 or LocalStack)
   - Starts gRPC server on configured port (default 8080)
   - Resolves parent service address from config

2. **Register with Parent**
   - Opens gRPC connection to parent service
   - Calls RegisterSidecar RPC
   - Sends: team_id, service_name, version, db_info, hostname:port
   - Receives: sidecar_id, success status
   - Stores sidecar_id for heartbeat

3. **Heartbeat Loop**
   - Every 30 seconds, send Heartbeat RPC
   - Includes: sidecar_id, status, metrics, timestamp
   - Parent updates last_seen timestamp
   - If heartbeat fails 3 times, mark as unhealthy

4. **Parent Monitoring**
   - Tracks all registered sidecars with GRPCAddress
   - If last_seen > 2 minutes, mark as disconnected
   - Can call sidecar's Health RPC directly
   - Routes queries to sidecar via ExecuteQuery RPC

---

## AWS Services Integration

### 1. Amazon Bedrock
**Purpose**: AI model for natural language to SQL conversion

**Configuration**:
- Region: us-east-1 (check model availability)
- Model: anthropic.claude-3-5-sonnet-20241022-v2:0
- VPC Endpoint: Recommended for security

**IAM Permissions**:
```
bedrock:InvokeModel
bedrock:InvokeModelWithResponseStream
```

**Prompt Engineering Strategy**:
- Include full database schema in system prompt
- Provide example queries for better accuracy
- Specify database dialect (PostgreSQL, MySQL, PartiQL, MQL)
- Enforce read-only constraint
- Request only query output (no explanations)

**Database-Specific Prompts**:

**SQL Databases (PostgreSQL/MySQL)**:
- Generate SELECT queries only
- Use proper JOIN syntax
- Include LIMIT clauses

**DynamoDB (PartiQL)**:
- Generate PartiQL SELECT statements
- Consider partition key and sort key in queries
- Account for eventually consistent reads
- Example: `SELECT * FROM "TableName" WHERE "PK" = 'value'`

**MongoDB (MQL)**:
- Generate aggregation pipeline or find() queries
- Use proper MongoDB operators ($match, $group, $project)
- Consider collection indexes
- Return as JSON query object
- Example: `db.collection.find({ status: "active" }).limit(10)`

### 2. AWS Systems Manager (SSM) Parameter Store
**Purpose**: Store configuration for sidecars

**Parameter Structure**:
```
/platform/parent-service/config
/teams/team-{id}/sidecar/config
/teams/team-{id}/sidecar/parent-address
/teams/team-{id}/database/host
/teams/team-{id}/database/name
```

**Benefits**:
- Centralized configuration
- Version tracking
- Encryption at rest
- No application redeployment for config changes

### 3. AWS Secrets Manager
**Purpose**: Store database credentials securely

**Secret Structure**:
```json
{
  "username": "app_user",
  "password": "...",
  "engine": "postgres",
  "host": "...",
  "port": 5432,
  "dbname": "production"
}
```

**Rotation**: Enable automatic rotation for RDS credentials

**Note**: For DynamoDB, credentials are not needed - IAM roles provide access directly.

### 4. Database Services
**Purpose**: Host client databases (team-specific)

**Amazon RDS** (Current Support):
- PostgreSQL 14+
- MySQL 8+
- Amazon Aurora (PostgreSQL/MySQL compatible)

**Security** (infrastructure assumed configured):
- Encryption at rest (KMS)
- Encryption in transit (SSL/TLS)
- Connection via IAM authentication (optional) or credentials from Secrets Manager

**NoSQL Databases** (Future Support):

**Amazon DynamoDB**:
- Query Language: PartiQL (SQL-like for DynamoDB)
- Access: IAM role-based (no credentials needed)
- Schema: Schemaless, introspect via DescribeTable API
- Considerations: Different query patterns, cost per request

**MongoDB** (Self-hosted or Atlas):
- Query Language: MongoDB Query Language (MQL) / Aggregation Pipeline
- Access: Username/password via Secrets Manager
- Schema: Schemaless, introspect via collection stats
- Deployment: EC2, ECS, or MongoDB Atlas

### 5. AWS Cloud Map
**Purpose**: Service discovery for parent service

**Configuration**:
- Namespace: ai-query-platform.local
- Service: parent-service
- Health checks: TCP on gRPC port (9090)

**Benefits**:
- Sidecars discover parent dynamically
- No hardcoded IPs
- Automatic health checking

### 6. Amazon ECS Fargate
**Purpose**: Run containerized services (infrastructure assumed configured)

**Task Resource Requirements**:
- Parent service: 2 vCPU, 4 GB RAM
- Sidecar: 0.5 vCPU, 1 GB RAM
- Network mode: awsvpc

### 7. Amazon ElastiCache (Redis)
**Purpose**: Cache query results

**Configuration**:
- Engine: Redis 7.x
- Instance: cache.t4g.small (start), scale up as needed
- Replication: Multi-AZ for HA

**Cache Strategy**:
- Key: query_result:{query_hash}
- TTL: 1 hour (configurable per query)
- Store: JSON serialized results

### 8. Amazon DynamoDB
**Purpose**: Store dashboard configurations and metadata

**Tables**:

**Dashboards**:
```
PK: team_id#{dashboard_id}
SK: version#{timestamp}
Attributes: name, widgets[], created_at
```

**QueryHistory**:
```
PK: team_id
SK: timestamp
Attributes: query, sql, execution_time, result_count, query_id
```

**SidecarRegistry**:
```
PK: sidecar_id
Attributes: team_id, service_name, status, last_seen, db_info
GSI: team_id for lookups
```

### 9. Amazon CloudWatch
**Purpose**: Monitoring, logging, alerting (infrastructure assumed to be configured)

**Application Metrics to Emit**:
- Bedrock invocation count, latency, errors, tokens used
- Query execution time (P50, P95, P99)
- Cache hit ratio
- Database connection pool utilization
- Query validation failures

**Application Logs** (structured JSON format):
- Query execution logs
- Bedrock API calls
- Database operations
- Error traces

### 10. AWS IAM
**Purpose**: Access control via task roles (handled automatically by AWS SDK)

**Important**: IAM authentication is handled automatically by the AWS SDK when running in ECS with task roles. No manual implementation of IAM auth is required in your code.

**Required Permissions**:

**ECS Task Role (Sidecar)**:
- bedrock:InvokeModel
- ssm:GetParameter
- secretsmanager:GetSecretValue
- rds-db:connect (if using IAM database authentication)
- dynamodb:DescribeTable, dynamodb:ExecuteStatement (for DynamoDB)

**ECS Task Role (Parent)**:
- dynamodb:GetItem, PutItem, Query, Scan
- elasticache:Connect
- cloudwatch:PutMetricData

**Note**: The AWS SDK automatically uses the ECS task role credentials. You don't need to implement credential management or authentication logic - just ensure the task roles have the correct permissions.

---

## Security & Compliance

**Note**: VPC, security groups, and network infrastructure are assumed to be already configured. This section covers application-level security only.

### Application Security

**SQL Injection Prevention**:
1. Parameterized queries only
2. SQL parser validates queries before execution
3. Allowlist of permitted tables (configurable)
4. Deny list for dangerous keywords (DROP, DELETE, UPDATE, etc.)
5. Read-only database user by default

**Authorization**:
- IAM roles for service-to-service (sidecars to parent)
- mTLS for gRPC (optional, future enhancement)
- Team-based access control (configurable per deployment)

**Data Encryption**:
- At rest: KMS encryption for RDS, DynamoDB, Redis
- In transit: TLS 1.3 for all HTTP/gRPC traffic
- Secrets: Secrets Manager encryption

**Audit Logging**:
- All queries logged with: team, timestamp, query, SQL, execution time
- API access logged with: endpoint, IP, timestamp
- Configuration changes logged

### Compliance Considerations

**GDPR**:
- Data minimization: Only query necessary data
- Right to erasure: Delete saved dashboards and history on request
- Data portability: Export capabilities
- Consent: Clear terms for data processing

**SOC 2**:
- Access controls via IAM
- Encryption at rest and in transit
- Audit logging of all activities
- Regular security assessments

**HIPAA** (if applicable):
- BAA with AWS
- Encrypted PHI data
- Audit logs for PHI access
- Access controls

---

## Deployment Considerations

**Note**: Terraform, VPC setup, ECS configuration, and CI/CD pipelines are assumed to be already handled by your existing infrastructure.

### Application-Specific Deployment Notes

**Container Images**:
- Build separate images for sidecar and parent service
- Tag with version/commit SHA for traceability
- Sidecar image should be lightweight (<100MB if possible)

**ECS Task Definition Requirements**:
- Sidecar: 0.5 vCPU, 1 GB RAM minimum
- Parent: 2 vCPU, 4 GB RAM minimum
- Both need task roles with appropriate IAM permissions
- Use awsvpc network mode for both

**Configuration Management**:
- Store team-specific configs in SSM Parameter Store
- Database credentials in Secrets Manager
- Use environment variables for team_id, service_name

---

## Scalability & Performance

**Note**: Auto-scaling policies assumed to be configured in infrastructure. This section covers application-level optimizations.

### Performance Optimization

**Caching Strategy**:
- Schema cache: 24 hours (invalidate on schema change)
- Query results cache: 1 hour (configurable)
- Dashboard configs cache: 5 minutes
- Team metadata cache: 15 minutes

**Database Optimization**:
- Connection pooling (max 10 connections per sidecar)
- Prepared statements
- Query timeout: 30 seconds default
- Row limit: 10,000 rows default

**Bedrock Optimization**:
- Streaming responses for large contexts
- Token limit enforcement
- Temperature: 0.2 (deterministic SQL)
- Caching of schema descriptions

**gRPC Optimization**:
- Connection pooling
- Keep-alive: 30 seconds
- Compression: gzip enabled
- Load balancing: Round-robin

### Expected Performance

**Query Execution**:
- Natural language to SQL: 1-3 seconds (Bedrock)
- SQL execution: 0.5-5 seconds (depends on query)
- Total end-to-end: 2-8 seconds

**Concurrent Users**:
- 100 concurrent users: Handled by 2 parent service tasks
- 500 concurrent users: Auto-scale to ~5 tasks
- 1000+ concurrent users: Scale to 10+ tasks

**Throughput**:
- Parent service: 100-200 queries/second per task
- Sidecar: 50-100 queries/second
- Bedrock: Rate limits vary by region (typically 10-100 requests/second)

---

## Cost Estimation

**Note**: Infrastructure costs (ECS, RDS, VPC, ALB, CloudWatch) are assumed to be part of existing infrastructure. This section covers application-specific costs only.

### Application-Level Costs (Monthly)

**AWS Bedrock** (Claude 3.5 Sonnet):
- Assumptions: 10,000 queries/month, avg 2K input + 500 output tokens
- Input: 10K × 2K tokens = 20M tokens × $3/1M = $60
- Output: 10K × 500 tokens = 5M tokens × $15/1M = $75
- **Total: ~$135/month for 10K queries**

**Per-Query Cost**:
- Claude 3.5 Sonnet: ~$0.01-0.03 per query
- Claude 3 Haiku (fallback): ~$0.001-0.005 per query

**Cost Optimization Strategies**:
1. Use Bedrock's Claude Haiku for simple queries (90% cheaper)
2. Implement aggressive caching to reduce duplicate queries
3. Cache schema descriptions to reduce token usage
4. Set up query result caching (Redis) - most queries hit cache after first execution

---

## Implementation Phases

### Phase 1: MVP (4-6 weeks)

**Scope**:
- Single team support
- Basic sidecar with Bedrock integration
- Simple parent service (no caching)
- Basic dashboard (query input, table output)
- PostgreSQL support only

**Deliverables**:
1. gRPC protocol definitions
2. Sidecar service (Go)
   - Connect to RDS
   - Bedrock integration
   - gRPC client
3. Parent service (Go)
   - gRPC server
   - Basic REST API
4. Dashboard (React)
   - Team selection
   - Query input
   - Table display
5. Deployment artifacts
   - Dockerfiles for sidecar and parent
   - ECS task definition JSONs
   - Configuration examples

**Success Criteria**:
- Single team can query their database via natural language
- Results displayed in table format
- <5 second query latency

### Phase 2: Multi-Team & Caching (3-4 weeks)

**Scope**:
- Support multiple teams
- Redis caching layer
- Dashboard configurations storage
- Query history

**Deliverables**:
1. Redis integration
2. DynamoDB tables
3. Sidecar registry
4. Enhanced dashboard
   - Query history
   - Saved queries

**Success Criteria**:
- 10+ teams supported
- Cache hit ratio >50%
- Users can save and reuse queries

### Phase 3: Visualization & Dashboards (3-4 weeks)

**Scope**:
- Auto-suggest chart types
- Dashboard builder
- Multiple chart types (bar, line, pie)
- Export capabilities

**Deliverables**:
1. Chart suggestion algorithm
2. Dashboard builder UI
3. Chart components (Recharts)
4. Export to CSV/JSON
5. Dashboard sharing

**Success Criteria**:
- Automatic chart suggestions >80% accuracy
- Users can create custom dashboards
- Dashboards load in <2 seconds

### Phase 4: Production Hardening (2-3 weeks)

**Scope**:
- Comprehensive monitoring
- Alerting
- Performance optimization
- Security hardening
- Documentation

**Deliverables**:
1. Load testing results
2. Security audit
3. User documentation
4. API documentation
5. Application metrics and logging
6. Error handling and retry logic

**Success Criteria**:
- >99.5% uptime
- P95 latency <5 seconds
- All security findings remediated
- Complete documentation

### Phase 5: NoSQL Support (3-4 weeks)

**Scope**:
- DynamoDB integration
- MongoDB integration
- NoSQL-specific query generation
- Schema inference for schemaless databases

**Deliverables**:
1. DynamoDB database adapter
   - PartiQL query generation
   - Table schema introspection
   - Pagination handling
2. MongoDB database adapter
   - MQL/aggregation pipeline generation
   - Collection schema inference
   - Index-aware query optimization
3. Database-specific AI prompts
4. Updated dashboard (no schema browser for NoSQL)
5. Testing with sample NoSQL datasets

**Success Criteria**:
- Can query DynamoDB tables via natural language
- Can query MongoDB collections via natural language
- Proper handling of schemaless data
- Performance comparable to SQL databases

### Phase 6: Advanced Features (Ongoing)

**Potential Features**:
- Real-time query streaming
- Collaborative dashboards
- Scheduled queries/reports
- Slack/email notifications
- AI-suggested insights ("Revenue dropped 15% - investigate?")
- Natural language follow-up questions
- Query optimization suggestions
- Cost analysis per query

---

## Risk Mitigation

### Technical Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Bedrock rate limiting | High | Medium | Implement queue, caching, fallback to Haiku |
| SQL injection vulnerability | Critical | Low | Multiple validation layers, read-only users |
| Sidecar crashes affect main app | Medium | Low | Health checks, automatic restarts, isolation |
| Database connection exhaustion | High | Medium | Connection pooling, limits, monitoring |
| gRPC connection failures | Medium | Medium | Retry logic, circuit breakers, timeouts |
| Inaccurate SQL generation | Medium | Medium | Prompt engineering, validation, user feedback loop |

### Operational Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Team misconfiguration | Medium | Medium | Validation on config, templates, documentation |
| Cost overrun (Bedrock) | High | Medium | Budget alerts, rate limiting, usage dashboards |
| Parent service outage | Critical | Low | Multi-AZ, auto-scaling, health checks |
| Cache poisoning | Medium | Low | Cache key hashing, TTL, invalidation API |
| Unauthorized access | Critical | Low | IAM, audit logs, least privilege, external auth integration |

### Business Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Low adoption by teams | High | Medium | Pilot program, training, clear value proposition |
| Compliance issues | Critical | Low | Security review, compliance checklist, audits |
| Vendor lock-in (AWS) | Medium | High | Modular design, abstract cloud services |
| Schema changes break queries | Medium | High | Schema versioning, auto-detection, notifications |

---

## Success Metrics

### Technical KPIs

- **Availability**: >99.5% uptime
- **Latency**: P95 <5 seconds end-to-end
- **Error Rate**: <1% of queries
- **Cache Hit Ratio**: >60%
- **SQL Accuracy**: >85% (queries executable without modification)

### Business KPIs

- **Adoption**: Number of teams using platform
- **Engagement**: Queries per team per week
- **Satisfaction**: NPS score >50
- **Time Saved**: Estimated hours saved vs manual SQL writing
- **Dashboard Creation**: Dashboards created per team

### Cost KPIs

- **Cost per Query**: <$0.05 (including all AWS services)
- **Cost per Team**: <$50/month
- **ROI**: Time saved vs platform cost

---

## Future Enhancements

### Short-term (3-6 months)
- Multi-database join queries (across teams, with permission)
- Query performance analysis and optimization suggestions
- AI-generated insights ("Sales dropped 20% in region X")
- Mobile-responsive dashboard
- Slack integration for query notifications

### Medium-term (6-12 months)
- **NoSQL database support** (DynamoDB, MongoDB)
  - PartiQL query generation for DynamoDB
  - MQL generation for MongoDB
  - Schema introspection for schemaless databases
  - Database-specific optimizations
- Streaming dashboards (real-time updates)
- Predictive analytics (forecast future trends)
- Natural language alerts ("Notify me if revenue drops >10%")
- API for programmatic access

### Long-term (12+ months)
- Self-service sidecar deployment (teams deploy their own)
- Marketplace for dashboard templates
- AI-powered anomaly detection
- Multi-cloud support (Azure, GCP)
- Open-source community edition

---

## Appendix

### Glossary

- **Sidecar**: Container deployed alongside main application container
- **Parent Service**: Central hub service that aggregates sidecar data
- **Bedrock**: AWS managed service for foundation models
- **gRPC**: High-performance RPC framework
- **ECS**: Elastic Container Service
- **Fargate**: Serverless compute for containers
- **SSM**: AWS Systems Manager
- **Cloud Map**: AWS service discovery solution

### References

- AWS Bedrock Documentation: https://docs.aws.amazon.com/bedrock/
- gRPC Best Practices: https://grpc.io/docs/guides/
- AWS ECS Best Practices: https://docs.aws.amazon.com/AmazonECS/latest/bestpracticesguide/
- SQL Injection Prevention: https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html

### Contact & Support

- Architecture Owner: [TBD]
- On-call Rotation: [TBD]
- Slack Channel: #ai-query-platform
- Documentation: [Confluence/Wiki Link]

---

**Document Version**: 1.2
**Last Updated**: 2025-11-20
**Changes**:
- v1.1: Added NoSQL database support (DynamoDB/MongoDB), database abstraction layer, removed authentication
- v1.2: Simplified infrastructure sections (VPC, Terraform, deployment assumed handled), clarified IAM auth is automatic
**Next Review**: 2025-12-20
