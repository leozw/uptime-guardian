# Uptime Guardian

<div align="center">
  <h3>🛡️ Enterprise-Grade Uptime Monitoring System</h3>
  <p>Multi-tenant monitoring solution with advanced SLA tracking, intelligent alerting, and comprehensive metrics</p>
</div>

## 📋 Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Installation](#installation)
- [Configuration](#configuration)
- [API Reference](#api-reference)
  - [Authentication](#authentication)
  - [Monitors](#monitors)
  - [Monitor Groups](#monitor-groups)
  - [Incidents](#incidents)
  - [Metrics](#metrics)
- [Monitor Types](#monitor-types)
- [Metrics & Observability](#metrics--observability)
- [Examples](#examples)
- [Development](#development)

## 🌟 Overview

Uptime Guardian is a comprehensive monitoring solution designed for multi-tenant environments. It provides real-time monitoring of HTTP endpoints, SSL certificates, DNS records, and domain expiration, with advanced features like monitor grouping, SLA tracking, and intelligent incident management.

### Key Capabilities

- **Multi-tenant Architecture**: Complete isolation between tenants with Keycloak integration
- **Multiple Monitor Types**: HTTP, SSL, DNS, and Domain monitoring
- **Monitor Groups**: Logical grouping of related monitors with composite health scores
- **SLA/SLO Management**: Track and report on service level objectives
- **Intelligent Alerting**: Reduce alert fatigue with smart correlation
- **Comprehensive Metrics**: Prometheus/Mimir integration with detailed observability
- **Incident Management**: Automatic incident creation, tracking, and acknowledgment

## ✨ Features

### Monitor Management
- ✅ Create, update, delete monitors
- ✅ Enable/disable monitoring
- ✅ Multi-region monitoring support
- ✅ Custom check intervals and timeouts
- ✅ Tag-based organization

### Monitor Groups
- ✅ Logical service grouping
- ✅ Weighted health score calculation
- ✅ Critical monitor designation
- ✅ Group-level SLA tracking
- ✅ Unified incident management

### Incident Management
- ✅ Automatic incident detection
- ✅ Incident acknowledgment workflow
- ✅ MTTR/MTTA tracking
- ✅ Incident timeline and comments
- ✅ Root cause analysis

### Notifications
- ✅ Multiple notification channels
- ✅ Customizable alert rules
- ✅ Alert suppression and correlation
- ✅ Cooldown periods

### Metrics & Reporting
- ✅ Real-time Prometheus metrics
- ✅ SLA/SLO reporting
- ✅ Historical data analysis
- ✅ Grafana-ready dashboards

## 🏗️ Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Frontend      │────▶│   API Gateway   │────▶│   Keycloak      │
│   (React)       │     │    (Kong)       │     │   (Auth)        │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                │
                                ▼
                        ┌───────────────┐
                        │   API Server  │
                        │   (Golang)    │
                        └───────┬───────┘
                                │
                ┌───────────────┼───────────────┐
                ▼               ▼               ▼
        ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
        │  PostgreSQL  │ │    Mimir     │ │   Worker     │
        │  (Database)  │ │  (Metrics)   │ │  (Checks)    │
        └──────────────┘ └──────────────┘ └──────────────┘
```

## 🚀 Installation

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Docker & Docker Compose
- Keycloak instance
- Mimir/Prometheus instance

### Quick Start

1. **Clone the repository**
```bash
git clone https://github.com/leozw/uptime-guardian.git
cd uptime-guardian
```

2. **Set up environment variables**
```bash
cp .env.example .env
# Edit .env with your configuration
```

3. **Run database migrations**
```bash
go run cmd/migrate/main.go up
```

4. **Start the services**
```bash
# Start API server
go run cmd/api/main.go

# Start worker (in another terminal)
go run cmd/worker/main.go
```

### Docker Deployment

```bash
docker-compose up -d
```

## ⚙️ Configuration

### Environment Variables

```env
# Database
DATABASE_URL=postgres://user:pass@localhost:5432/uptime_guardian?sslmode=disable

# Keycloak
KEYCLOAK_URL=https://auth.example.com
KEYCLOAK_REALM=your-realm
KEYCLOAK_CLIENT_ID=uptime-guardian
KEYCLOAK_CLIENT_SECRET=your-secret

# Mimir
MIMIR_URL=https://mimir.example.com
MIMIR_AUTH_TOKEN=your-token

# Server
SERVER_PORT=8080
```

### Configuration File (config.yaml)

```yaml
server:
  port: 8080
  mode: release

database:
  url: ${DATABASE_URL}
  maxconnections: 25
  maxidleconns: 5

scheduler:
  worker_count: 10
  check_timeout: 30s
  max_retries: 3

regions:
  us-east:
    name: US East
    location: Virginia
    provider: aws
  eu-west:
    name: EU West
    location: Ireland
    provider: aws
```

## 📚 API Reference

### Base URL
```
https://api.uptime-guardian.com/api/v1
```

### Authentication

All API requests require a Bearer token from Keycloak:

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
     https://api.uptime-guardian.com/api/v1/monitors
```

### Response Format

```json
{
  "data": { ... },
  "error": null,
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 100
  }
}
```

## 📊 Monitors

### Create Monitor

```http
POST /api/v1/monitors
```

#### HTTP Monitor Example
```json
{
  "name": "Production API",
  "type": "http",
  "target": "https://api.example.com/health",
  "enabled": true,
  "interval": 60,
  "timeout": 30,
  "regions": ["us-east", "eu-west"],
  "config": {
    "method": "GET",
    "expected_status_codes": [200, 204],
    "headers": {
      "User-Agent": "Uptime-Guardian/1.0"
    },
    "search_string": "\"status\":\"healthy\"",
    "follow_redirects": true
  },
  "notification_config": {
    "channels": [
      {
        "type": "webhook",
        "enabled": true,
        "config": {
          "url": "https://slack.com/webhook",
          "method": "POST"
        }
      }
    ],
    "on_failure_count": 3,
    "on_recovery": true,
    "reminder_interval": 60
  },
  "tags": {
    "environment": "production",
    "team": "backend"
  }
}
```

#### SSL Monitor Example
```json
{
  "name": "Production SSL Certificate",
  "type": "ssl",
  "target": "https://example.com",
  "enabled": true,
  "interval": 3600,
  "timeout": 10,
  "regions": ["us-east"],
  "config": {
    "check_expiry": true,
    "min_days_before_expiry": 30
  }
}
```

#### DNS Monitor Example
```json
{
  "name": "DNS Resolution Check",
  "type": "dns",
  "target": "example.com",
  "enabled": true,
  "interval": 300,
  "timeout": 5,
  "regions": ["us-east", "eu-west", "asia-pac"],
  "config": {
    "record_type": "A",
    "expected_values": ["93.184.216.34"]
  }
}
```

#### Domain Monitor Example
```json
{
  "name": "Domain Expiration Check",
  "type": "domain",
  "target": "example.com",
  "enabled": true,
  "interval": 86400,
  "timeout": 30,
  "regions": ["us-east"],
  "config": {
    "domain_min_days_before_expiry": 60
  }
}
```

### List Monitors

```http
GET /api/v1/monitors?page=1&limit=20
```

Response:
```json
{
  "monitors": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Production API",
      "type": "http",
      "target": "https://api.example.com/health",
      "enabled": true,
      "interval": 60,
      "timeout": 30,
      "regions": ["us-east", "eu-west"],
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 45
  }
}
```

### Get Monitor Status

```http
GET /api/v1/monitors/:id/status
```

Response:
```json
{
  "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "up",
  "message": "",
  "last_check": "2024-01-15T10:30:00Z",
  "response_time_ms": 125,
  "ssl_expiry_days": 45
}
```

### Get Monitor History

```http
GET /api/v1/monitors/:id/history?limit=100
```

Response:
```json
{
  "history": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440000",
      "status": "up",
      "response_time_ms": 125,
      "status_code": 200,
      "region": "us-east",
      "checked_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

### Set Monitor SLO

```http
POST /api/v1/monitors/:id/slo
```

Request:
```json
{
  "target_uptime_percentage": 99.9,
  "measurement_period_days": 30
}
```

## 👥 Monitor Groups

Monitor Groups allow you to logically group related monitors and track their collective health.

### Create Monitor Group

```http
POST /api/v1/monitor-groups
```

Request:
```json
{
  "name": "E-commerce Platform",
  "description": "All services for the e-commerce platform",
  "enabled": true,
  "members": [
    {
      "monitor_id": "550e8400-e29b-41d4-a716-446655440001",
      "weight": 0.4,
      "is_critical": true
    },
    {
      "monitor_id": "550e8400-e29b-41d4-a716-446655440002",
      "weight": 0.3,
      "is_critical": true
    },
    {
      "monitor_id": "550e8400-e29b-41d4-a716-446655440003",
      "weight": 0.2,
      "is_critical": false
    },
    {
      "monitor_id": "550e8400-e29b-41d4-a716-446655440004",
      "weight": 0.1,
      "is_critical": false
    }
  ],
  "notification_config": {
    "channels": [
      {
        "type": "email",
        "enabled": true,
        "config": {
          "to": ["ops-team@example.com"]
        }
      }
    ]
  },
  "tags": {
    "business_unit": "e-commerce",
    "priority": "p1"
  }
}
```

### Get Group Status

```http
GET /api/v1/monitor-groups/:id/status
```

Response:
```json
{
  "group_id": "770e8400-e29b-41d4-a716-446655440000",
  "overall_status": "degraded",
  "health_score": 85.5,
  "monitors_up": 3,
  "monitors_down": 0,
  "monitors_degraded": 1,
  "critical_monitors_down": 0,
  "last_check": "2024-01-15T10:35:00Z",
  "message": "1 monitor(s) degraded"
}
```

### Set Group SLO

```http
POST /api/v1/monitor-groups/:id/slo
```

Request:
```json
{
  "target_uptime_percentage": 99.95,
  "measurement_period_days": 30,
  "calculation_method": "weighted_average"
}
```

Calculation methods:
- `weighted_average`: Uses monitor weights
- `worst_case`: Takes the worst performing monitor
- `critical_only`: Only considers critical monitors

### Create Alert Rule

```http
POST /api/v1/monitor-groups/:id/alert-rules
```

Request:
```json
{
  "name": "Health Score Alert",
  "enabled": true,
  "trigger_condition": "health_score_below",
  "threshold_value": 90,
  "notification_channels": [
    {
      "type": "slack",
      "enabled": true,
      "config": {
        "webhook_url": "https://hooks.slack.com/...",
        "channel": "#alerts"
      }
    }
  ],
  "cooldown_minutes": 10
}
```

Trigger conditions:
- `health_score_below`: Triggers when health score < threshold
- `any_critical_down`: Triggers when any critical monitor is down
- `percentage_down`: Triggers when X% of monitors are down
- `all_down`: Triggers when all monitors are down

## 🚨 Incidents

### Get Monitor Incidents

```http
GET /api/v1/monitors/:id/incidents?limit=50
```

Response:
```json
{
  "incidents": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440000",
      "monitor_id": "550e8400-e29b-41d4-a716-446655440000",
      "started_at": "2024-01-15T09:00:00Z",
      "resolved_at": "2024-01-15T09:15:00Z",
      "severity": "critical",
      "downtime_minutes": 15,
      "affected_checks": 8,
      "acknowledged_at": "2024-01-15T09:05:00Z",
      "acknowledged_by": "john@example.com"
    }
  ]
}
```

### Acknowledge Incident

```http
POST /api/v1/incidents/:incident_id/acknowledge
```

### Add Incident Comment

```http
POST /api/v1/incidents/:incident_id/comment
```

Request:
```json
{
  "comment": "Investigating database connection issues"
}
```

## 📈 Metrics

### Get Metrics Summary

```http
GET /api/v1/metrics/summary?range=24h
```

Response:
```json
{
  "overview": {
    "total_monitors": 45,
    "enabled_monitors": 42,
    "monitors_by_type": {
      "http": 30,
      "ssl": 10,
      "dns": 3,
      "domain": 2
    },
    "monitors_by_status": {
      "up": 40,
      "down": 1,
      "degraded": 1,
      "unknown": 0
    }
  },
  "sla": {
    "average_uptime_percentage": 99.85,
    "sla_violations": 2,
    "monitors_with_sla": 35
  },
  "incidents": {
    "active_incidents": 1,
    "total_incidents": 15,
    "average_mttr_minutes": 12.5
  },
  "time_range": {
    "start": "2024-01-14T10:35:00Z",
    "end": "2024-01-15T10:35:00Z"
  }
}
```

### Get Monitor Metrics

```http
GET /api/v1/monitors/:id/metrics
```

Response:
```json
{
  "monitor": { ... },
  "current_status": { ... },
  "sla": {
    "current": {
      "uptime_percentage": 99.92,
      "downtime_minutes": 35,
      "slo_met": true
    },
    "history": [ ... ],
    "slo_configuration": {
      "target_uptime_percentage": 99.9,
      "measurement_period_days": 30
    }
  },
  "performance": {
    "average_response_time_ms": 145,
    "p95_response_time_ms": 320,
    "p99_response_time_ms": 580,
    "total_checks": 43200,
    "successful_checks": 43165
  },
  "incidents": {
    "recent": [ ... ],
    "total_count": 3,
    "active_count": 0
  }
}
```

## 📊 Prometheus Metrics

The system exports comprehensive Prometheus metrics:

### Check Metrics
- `uptime_check_duration_seconds` - Check duration histogram
- `uptime_check_up` - Whether check is up (1) or down (0)
- `uptime_checks_total` - Total checks counter
- `uptime_http_response_code` - HTTP response codes

### SSL Metrics
- `ssl_cert_days_until_expiry` - Days until certificate expires
- `ssl_cert_valid` - Certificate validity status

### DNS Metrics
- `dns_lookup_duration_seconds` - DNS lookup duration
- `dns_record_count` - Number of DNS records found
- `dns_resolution_success` - DNS resolution success status

### Domain Metrics
- `domain_days_until_expiry` - Days until domain expires
- `domain_valid` - Domain validity status

### SLA/SLO Metrics
- `uptime_sla_percentage` - Current SLA percentage
- `uptime_sla_target_percentage` - Target SLA percentage
- `uptime_slo_error_budget_remaining_minutes` - Error budget remaining
- `uptime_slo_violation` - SLO violation status

### Incident Metrics
- `uptime_incidents_total` - Total incidents
- `uptime_incident_duration_minutes` - Incident duration histogram
- `uptime_incidents_active` - Active incidents gauge
- `uptime_incident_mttr_minutes` - Mean Time To Recovery
- `uptime_incident_mtta_minutes` - Mean Time To Acknowledge

### Group Metrics
- `uptime_group_health_score` - Group health score (0-100)
- `uptime_group_status` - Group status (1=up, 0.5=degraded, 0=down)
- `uptime_group_monitors_up/down/degraded` - Monitor counts
- `uptime_group_critical_monitors_down` - Critical monitors down

## 🔍 Examples

### Complete Service Monitoring Setup

```bash
# 1. Create HTTP monitor for API
curl -X POST https://api.uptime-guardian.com/api/v1/monitors \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "API Health Check",
    "type": "http",
    "target": "https://api.example.com/health",
    "enabled": true,
    "interval": 60,
    "timeout": 30,
    "regions": ["us-east", "eu-west"],
    "config": {
      "expected_status_codes": [200]
    }
  }'

# 2. Create SSL monitor
curl -X POST https://api.uptime-guardian.com/api/v1/monitors \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "SSL Certificate",
    "type": "ssl",
    "target": "https://api.example.com",
    "enabled": true,
    "interval": 3600,
    "timeout": 10,
    "regions": ["us-east"],
    "config": {
      "check_expiry": true,
      "min_days_before_expiry": 30
    }
  }'

# 3. Create DNS monitor
curl -X POST https://api.uptime-guardian.com/api/v1/monitors \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "DNS Resolution",
    "type": "dns",
    "target": "api.example.com",
    "enabled": true,
    "interval": 300,
    "timeout": 5,
    "regions": ["us-east", "eu-west"],
    "config": {
      "record_type": "A"
    }
  }'

# 4. Create monitor group
curl -X POST https://api.uptime-guardian.com/api/v1/monitor-groups \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "API Service",
    "description": "Complete API service monitoring",
    "enabled": true,
    "members": [
      {
        "monitor_id": "MONITOR_ID_1",
        "weight": 0.6,
        "is_critical": true
      },
      {
        "monitor_id": "MONITOR_ID_2",
        "weight": 0.3,
        "is_critical": false
      },
      {
        "monitor_id": "MONITOR_ID_3",
        "weight": 0.1,
        "is_critical": false
      }
    ]
  }'

# 5. Set group SLO
curl -X POST https://api.uptime-guardian.com/api/v1/monitor-groups/GROUP_ID/slo \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "target_uptime_percentage": 99.9,
    "measurement_period_days": 30,
    "calculation_method": "weighted_average"
  }'

# 6. Create alert rule
curl -X POST https://api.uptime-guardian.com/api/v1/monitor-groups/GROUP_ID/alert-rules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Service Degradation Alert",
    "enabled": true,
    "trigger_condition": "health_score_below",
    "threshold_value": 90,
    "cooldown_minutes": 5
  }'
```

### Grafana Dashboard Queries

```promql
# Group Health Score
uptime_group_health_score{group_name="API Service"}

# Group Status Over Time
uptime_group_status{group_name="API Service"}

# SLA Compliance
uptime_group_sla_percentage{group_name="API Service"} >= 99.9

# Active Incidents
sum(uptime_group_incidents_active) by (group_name)

# Average Response Time by Monitor
avg(rate(uptime_check_duration_seconds_sum[5m])) by (monitor_name) / 
avg(rate(uptime_check_duration_seconds_count[5m])) by (monitor_name)

# Error Budget Burn Rate
(1 - (uptime_group_sla_percentage / 100)) * 43200  # minutes in 30 days
```

## 🛠️ Development

### Project Structure

```
uptime-guardian/
├── cmd/
│   ├── api/          # API server entry point
│   └── worker/       # Worker process entry point
├── internal/
│   ├── api/          # HTTP handlers and routing
│   ├── checks/       # Monitor check implementations
│   ├── config/       # Configuration management
│   ├── db/           # Database models and repositories
│   ├── groups/       # Monitor groups logic
│   ├── incidents/    # Incident management
│   ├── metrics/      # Prometheus metrics
│   ├── scheduler/    # Check scheduling
│   └── sla/          # SLA calculations
├── pkg/
│   └── keycloak/     # Keycloak client
├── deployments/      # Deployment configurations
└── docs/             # Documentation
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/checks
```

### Building

```bash
# Build API server
go build -o bin/api cmd/api/main.go

# Build worker
go build -o bin/worker cmd/worker/main.go

# Build with optimizations
go build -ldflags="-s -w" -o bin/api cmd/api/main.go
```

### Database Migrations

```bash
# Create new migration
migrate create -ext sql -dir internal/db/migrations -seq add_new_feature

# Run migrations
migrate -path internal/db/migrations -database $DATABASE_URL up

# Rollback
migrate -path internal/db/migrations -database $DATABASE_URL down 1
```

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📞 Support

- Documentation: https://docs.uptime-guardian.com
- Issues: https://github.com/leozw/uptime-guardian/issues
- Email: support@uptime-guardian.com

---

Built with ❤️ by the Uptime Guardian Team
