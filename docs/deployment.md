# Deployment Guide

This guide covers deploying Alyx to production environments, including Docker, bare metal, and cloud platforms.

## Deployment Options

| Option         | Best For                       | Complexity |
| -------------- | ------------------------------ | ---------- |
| Single Binary  | Simple deployments, VMs        | Low        |
| Docker         | Container orchestration, CI/CD | Medium     |
| Docker Compose | Multi-container setups         | Medium     |
| Kubernetes     | Large-scale, high availability | High       |

## Quick Production Checklist

Before deploying to production, ensure:

- [ ] Set a strong `JWT_SECRET` (min 32 characters)
- [ ] Configure CORS for your domain(s)
- [ ] Set up database backups
- [ ] Enable HTTPS (via reverse proxy)
- [ ] Review access control rules
- [ ] Set appropriate resource limits
- [ ] Configure logging and monitoring

## Single Binary Deployment

### Download and Setup

```bash
# Download latest release
curl -L https://github.com/watzon/alyx/releases/latest/download/alyx-linux-amd64 -o alyx
chmod +x alyx
sudo mv alyx /usr/local/bin/

# Create directories
sudo mkdir -p /var/lib/alyx /etc/alyx

# Create config file
sudo cat > /etc/alyx/alyx.yaml << 'EOF'
server:
  host: 0.0.0.0
  port: 8090
  cors:
    enabled: true
    origins:
      - "https://your-domain.com"

database:
  path: /var/lib/alyx/data.db

auth:
  jwt:
    secret: "${JWT_SECRET}"
    access_ttl: 15m
    refresh_ttl: 7d

functions:
  enabled: true
  pool:
    min_warm: 2
    max_instances: 20
EOF

# Copy your schema
sudo cp schema.yaml /etc/alyx/
```

### Systemd Service

```bash
# Create service file
sudo cat > /etc/systemd/system/alyx.service << 'EOF'
[Unit]
Description=Alyx Backend-as-a-Service
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=alyx
Group=alyx
WorkingDirectory=/etc/alyx
ExecStart=/usr/local/bin/alyx serve --config /etc/alyx/alyx.yaml
Restart=always
RestartSec=5

# Environment
Environment=JWT_SECRET=your-secure-secret-here

# Resource limits
MemoryLimit=1G
CPUQuota=200%

# Security
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/alyx

[Install]
WantedBy=multi-user.target
EOF

# Create user
sudo useradd -r -s /bin/false alyx
sudo chown -R alyx:alyx /var/lib/alyx /etc/alyx

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable alyx
sudo systemctl start alyx

# Check status
sudo systemctl status alyx
```

### Nginx Reverse Proxy

```nginx
# /etc/nginx/sites-available/alyx
upstream alyx {
    server 127.0.0.1:8090;
    keepalive 32;
}

server {
    listen 80;
    server_name api.your-domain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.your-domain.com;

    ssl_certificate /etc/letsencrypt/live/api.your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.your-domain.com/privkey.pem;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    location / {
        proxy_pass http://alyx;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

## Docker Deployment

### Docker Run

```bash
docker run -d \
  --name alyx \
  -p 8090:8090 \
  -v $(pwd)/schema.yaml:/app/schema.yaml:ro \
  -v $(pwd)/alyx.yaml:/app/alyx.yaml:ro \
  -v $(pwd)/functions:/app/functions:ro \
  -v alyx-data:/app/data \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e JWT_SECRET="your-secure-secret-here" \
  --restart unless-stopped \
  ghcr.io/watzon/alyx:latest
```

### Docker Compose (Recommended)

```yaml
# docker-compose.yml
version: "3.8"

services:
  alyx:
    image: ghcr.io/watzon/alyx:latest
    container_name: alyx
    ports:
      - "8090:8090"
    volumes:
      - ./schema.yaml:/app/schema.yaml:ro
      - ./alyx.yaml:/app/alyx.yaml:ro
      - ./functions:/app/functions:ro
      - alyx-data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - JWT_SECRET=${JWT_SECRET}
      - ALYX_SERVER_HOST=0.0.0.0
    restart: unless-stopped
    healthcheck:
      test:
        ["CMD", "wget", "-q", "--spider", "http://localhost:8090/health/live"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: "2"

volumes:
  alyx-data:
    driver: local
```

### Docker Compose with Nginx

```yaml
# docker-compose.yml
version: "3.8"

services:
  alyx:
    image: ghcr.io/watzon/alyx:latest
    volumes:
      - ./schema.yaml:/app/schema.yaml:ro
      - ./alyx.yaml:/app/alyx.yaml:ro
      - ./functions:/app/functions:ro
      - alyx-data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - JWT_SECRET=${JWT_SECRET}
    restart: unless-stopped
    networks:
      - internal

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
    depends_on:
      - alyx
    restart: unless-stopped
    networks:
      - internal
      - external

networks:
  internal:
  external:

volumes:
  alyx-data:
```

## Production Configuration

### Complete alyx.yaml

```yaml
# Production alyx.yaml
server:
  host: 0.0.0.0
  port: 8090

  # CORS configuration
  cors:
    enabled: true
    origins:
      - "https://your-app.com"
      - "https://admin.your-app.com"
    methods:
      - GET
      - POST
      - PATCH
      - DELETE
      - OPTIONS
    headers:
      - Authorization
      - Content-Type
    credentials: true
    max_age: 86400

database:
  path: /app/data/alyx.db

  # Optional: Use Turso for distributed deployments
  # turso:
  #   url: libsql://your-database.turso.io
  #   token: ${TURSO_TOKEN}

auth:
  jwt:
    secret: ${JWT_SECRET}
    access_ttl: 15m
    refresh_ttl: 7d
    issuer: "your-app"

  password:
    min_length: 8
    require_uppercase: true
    require_number: true

  oauth:
    github:
      client_id: ${GITHUB_CLIENT_ID}
      client_secret: ${GITHUB_CLIENT_SECRET}
    google:
      client_id: ${GOOGLE_CLIENT_ID}
      client_secret: ${GOOGLE_CLIENT_SECRET}

  rate_limit:
    login: 5/minute
    register: 3/minute

functions:
  enabled: true
  timeout: 30s

  pool:
    min_warm: 2
    max_instances: 20
    idle_timeout: 120s
    memory_limit: 512mb
    cpu_limit: 1.0

  env:
    APP_URL: https://your-app.com

logging:
  level: info
  format: json
```

### Environment Variables

Create a `.env` file (not committed to git):

```bash
# .env
JWT_SECRET=your-very-long-secure-secret-at-least-32-characters
GITHUB_CLIENT_ID=your-github-client-id
GITHUB_CLIENT_SECRET=your-github-client-secret
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
TURSO_TOKEN=your-turso-token
```

## Health Checks and Monitoring

### Health Endpoints

Alyx provides built-in health endpoints:

| Endpoint        | Purpose            | Response                        |
| --------------- | ------------------ | ------------------------------- |
| `/health`       | Full health status | JSON with component status      |
| `/health/live`  | Liveness probe     | `200 OK` if running             |
| `/health/ready` | Readiness probe    | `200 OK` if ready to serve      |
| `/health/stats` | Runtime statistics | Memory, goroutines, connections |
| `/metrics`      | Prometheus metrics | Prometheus format               |

### Prometheus Metrics

Configure Prometheus to scrape `/metrics`:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: "alyx"
    static_configs:
      - targets: ["alyx:8090"]
    scrape_interval: 15s
```

Available metrics:

- `alyx_http_requests_total` - Request count by method, path, status
- `alyx_http_request_duration_seconds` - Request latency histogram
- `alyx_http_requests_in_flight` - Currently processing requests
- `alyx_http_response_size_bytes` - Response size histogram
- `alyx_db_connections_*` - Database connection pool stats
- `alyx_realtime_connections` - Active WebSocket connections
- `alyx_function_invocations_total` - Function call count
- `alyx_function_duration_seconds` - Function execution time

### Grafana Dashboard

Import the Alyx dashboard from the repository:

```bash
curl -O https://raw.githubusercontent.com/watzon/alyx/main/contrib/grafana-dashboard.json
```

## Database Backups

### SQLite Backup

```bash
# Manual backup
sqlite3 /var/lib/alyx/data.db ".backup '/backups/alyx-$(date +%Y%m%d).db'"

# Automated backup script
cat > /etc/cron.daily/alyx-backup << 'EOF'
#!/bin/bash
BACKUP_DIR=/backups/alyx
DATE=$(date +%Y%m%d)
mkdir -p $BACKUP_DIR

# Create backup
sqlite3 /var/lib/alyx/data.db ".backup '$BACKUP_DIR/alyx-$DATE.db'"

# Compress
gzip $BACKUP_DIR/alyx-$DATE.db

# Keep only last 7 days
find $BACKUP_DIR -name "*.gz" -mtime +7 -delete
EOF

chmod +x /etc/cron.daily/alyx-backup
```

### Turso Backup

When using Turso, backups are handled automatically. You can also create manual snapshots:

```bash
turso db shell your-database .dump > backup.sql
```

## Scaling

### Vertical Scaling

Increase resources for a single instance:

```yaml
# docker-compose.yml
services:
  alyx:
    deploy:
      resources:
        limits:
          memory: 4G
          cpus: "4"
```

### Horizontal Scaling (with Turso)

For multiple Alyx instances, use Turso as the database:

```yaml
# alyx.yaml
database:
  turso:
    url: libsql://your-database.turso.io
    token: ${TURSO_TOKEN}
```

Then run multiple instances behind a load balancer:

```yaml
# docker-compose.yml
services:
  alyx:
    image: ghcr.io/watzon/alyx:latest
    deploy:
      replicas: 3
    environment:
      - TURSO_URL=libsql://your-database.turso.io
      - TURSO_TOKEN=${TURSO_TOKEN}
```

## Security Hardening

### 1. Use Strong Secrets

```bash
# Generate a strong JWT secret
openssl rand -base64 48
```

### 2. Enable HTTPS

Always use HTTPS in production via a reverse proxy (Nginx, Caddy, Traefik).

### 3. Restrict CORS Origins

```yaml
server:
  cors:
    origins:
      - "https://your-app.com" # Specific domains only
```

### 4. Rate Limiting

```yaml
auth:
  rate_limit:
    login: 5/minute
    register: 3/minute
```

### 5. Network Isolation

```yaml
# docker-compose.yml
services:
  alyx:
    networks:
      - internal # Not exposed externally

  nginx:
    networks:
      - internal
      - external # Exposed

networks:
  internal:
    internal: true # No external access
  external:
```

### 6. Container Security

```yaml
services:
  alyx:
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp
```

## Troubleshooting

### Common Issues

**Container can't connect to Docker socket:**

```bash
# Check socket permissions
ls -la /var/run/docker.sock

# Add user to docker group
sudo usermod -aG docker alyx
```

**Database locked errors:**

```yaml
# Increase busy timeout
database:
  busy_timeout: 10000 # 10 seconds
```

**WebSocket connections dropping:**

```nginx
# Increase proxy timeouts
proxy_read_timeout 3600s;
proxy_send_timeout 3600s;
```

**Function cold start too slow:**

```yaml
# Increase warm pool
functions:
  pool:
    min_warm: 3 # More warm containers
```

### Viewing Logs

```bash
# Systemd
journalctl -u alyx -f

# Docker
docker logs -f alyx

# Docker Compose
docker-compose logs -f alyx
```

### Debug Mode

For debugging, enable verbose logging:

```yaml
logging:
  level: debug
```

Or via environment variable:

```bash
ALYX_LOGGING_LEVEL=debug
```

## Cloud Deployments

### Fly.io

```toml
# fly.toml
app = "your-alyx-app"
primary_region = "ord"

[build]
  image = "ghcr.io/watzon/alyx:latest"

[env]
  ALYX_SERVER_HOST = "0.0.0.0"
  ALYX_SERVER_PORT = "8080"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = false
  auto_start_machines = true
  min_machines_running = 1

[[vm]]
  memory = "1gb"
  cpu_kind = "shared"
  cpus = 1

[mounts]
  source = "alyx_data"
  destination = "/app/data"
```

### Railway

```json
{
  "build": {
    "builder": "NIXPACKS"
  },
  "deploy": {
    "startCommand": "alyx serve",
    "healthcheckPath": "/health/live",
    "healthcheckTimeout": 30
  }
}
```

### Render

```yaml
# render.yaml
services:
  - type: web
    name: alyx
    env: docker
    dockerfilePath: ./Dockerfile
    healthCheckPath: /health/live
    envVars:
      - key: JWT_SECRET
        generateValue: true
    disk:
      name: alyx-data
      mountPath: /app/data
      sizeGB: 10
```
