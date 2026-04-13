# Development Guide

This guide covers development setup, testing, and contributing to Vega AI.

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- [Docker Compose](https://docs.docker.com/compose/)
- [Go 1.25+](https://golang.org/) (for local development)
- AI provider key: [Gemini](https://aistudio.google.com/app/apikey) (free), [OpenAI](https://platform.openai.com/api-keys), or [Ollama](https://ollama.com) for fully local

## Development Setup

1. **Clone the repository:**

   ```bash
   git clone https://github.com/benidevo/vega-ai.git
   cd vega-ai
   ```

2. **Set up environment:**

   Create a `.env` file with your configuration:

   ```bash
   # Create .env file
   cat > .env << EOF
   # Required — works with Gemini, OpenAI, or any OpenAI-compatible provider
   # GEMINI_API_KEY is deprecated but still works for backward compatibility
   AI_KEY=your-api-key
   TOKEN_SECRET=your-jwt-secret

   # Optional - for cloud mode
   CLOUD_MODE=false
   GOOGLE_CLIENT_ID=your-client-id
   GOOGLE_CLIENT_SECRET=your-client-secret
   EOF
   ```

3. **Start development environment:**

   ```bash
   make run
   ```

4. **Access the application:**
   - Web interface: <http://localhost:8765>
   - The application will auto-reload on code changes

## Development Commands

| Command | Description |
|---------|-------------|
| `make run` | Start development environment |
| `make test` | Run test suite |
| `make restart` | Rebuild and restart containers |
| `make logs` | View container logs |
| `make stop` | Stop containers |

## Frontend Development

### CSS Build Process

The project uses Tailwind CSS with a build process to optimize CSS delivery. Instead of loading the entire Tailwind compiler (340KB) from CDN in production, we compile and purge unused styles to create a minimal CSS file (~46KB).

#### Prerequisites

- Node.js and npm installed

#### Setup

1. Install dependencies:

   ```bash
   npm install
   ```

2. Build CSS for production:

   ```bash
   npm run build
   ```

3. For development with auto-rebuild on changes:

   ```bash
   npm run dev
   ```

#### Deployment

The CSS build should be run as part of your deployment process:

```bash
# Install dependencies and build CSS
npm install
npm run build

# Then run your Go application
go run main.go
```

#### Docker

If using Docker, add these steps to your Dockerfile:

```dockerfile
# Install Node.js
RUN apk add --no-cache nodejs npm

# Copy package files
COPY package*.json ./
COPY tailwind.config.js ./
COPY postcss.config.js ./

# Install dependencies and build CSS
RUN npm ci --only=production
RUN npm run build
```

The generated `static/css/output.css` file is gitignored and should be built during deployment.

### Template Structure

- `templates/layouts/base.html` - Base layout with CSS/JS includes
- `templates/partials/` - Reusable template components
- `static/` - Static assets (CSS, images, etc.)

## Testing

### Run Tests

```bash
# All tests
make test

# Specific package
go test ./internal/auth/...

# With coverage
go test -cover ./...
```

### Test Structure

- Unit tests: `*_test.go` files alongside source code
- Integration tests: `internal/*/setup_test.go`
- Test coverage reports: `coverage.out`

## Production Build

### Local Build

```bash
# Build Docker image locally
./scripts/docker-build.sh

# Build with specific tag and push
./scripts/docker-build.sh --tag v1.0.0 --push
```

### CI/CD

- Automated builds on GitHub Actions
- Images published to GitHub Container Registry
- Builds triggered on version tags and manual dispatch

## Project Structure

```plaintext
├── cmd/vega/           # Application entry point
├── internal/           # Private application code
│   ├── auth/          # Authentication & authorization
│   ├── job/           # Job management
│   ├── ai/            # AI integration (OpenAI-compatible)
│   ├── settings/      # User settings & profiles
│   └── common/        # Shared utilities
├── templates/         # HTML templates
├── static/           # Static assets
├── migrations/       # Database migrations
├── docker/           # Docker configurations
├── scripts/          # Build and utility scripts
└── docs/             # Documentation
```

## API Documentation

### Environment Variables

#### Core Application Settings

| Variable | Required | Description |
|----------|----------|-------------|
| `TOKEN_SECRET` | Yes | JWT secret for user sessions |
| `AI_KEY` | Yes | API key for your AI provider (Gemini, OpenAI, etc). `GEMINI_API_KEY` is deprecated but still works. |
| `GOOGLE_CLIENT_ID` | No | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | No | Google OAuth client secret |

#### Admin User Management

**Self-Hosted Mode Only**: Admin user creation is disabled in cloud mode. In cloud mode, users authenticate via Google OAuth and admin privileges must be granted manually in the database.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ADMIN_USERNAME` | No | `admin` | Admin username (3-50 characters) |
| `ADMIN_PASSWORD` | No | `VegaAdmin` | Admin password (8-64 characters) |
| `RESET_ADMIN_PASSWORD` | No | `false` | Reset admin password on startup if user exists |

**Note**: In self-hosted mode, if no admin user exists, one is created automatically on startup using the configured credentials.

**Admin User Setup Examples:**

```bash
# Self-hosted with custom admin credentials
ADMIN_USERNAME=myadmin
ADMIN_PASSWORD=SecurePassword123

# Reset existing admin password
ADMIN_USERNAME=admin
ADMIN_PASSWORD=NewSecurePassword456
RESET_ADMIN_PASSWORD=true
```

#### CORS Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CORS_ALLOWED_ORIGINS` | No | `*` | Comma-separated list of allowed origins |
| `CORS_ALLOW_CREDENTIALS` | No | `false` | Allow credentials in CORS requests |

**CORS Setup Examples:**

```bash
# Development - allow all origins
CORS_ALLOWED_ORIGINS=*
CORS_ALLOW_CREDENTIALS=false

# Production - specific origins only
CORS_ALLOWED_ORIGINS=https://yourdomain.com,https://api.yourdomain.com
CORS_ALLOW_CREDENTIALS=true
```

### Database

- SQLite for development and small deployments
- Automatic migrations on startup
- Schema in `migrations/sqlite/`

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes
4. Add tests for new functionality
5. Ensure tests pass: `make test`
6. Commit your changes: `git commit -m 'Add amazing feature'`
7. Push to the branch: `git push origin feature/amazing-feature`
8. Open a Pull Request

### Code Style

- Follow Go conventions and `gofmt`
- Use meaningful variable and function names
- Add comments for exported functions
- Keep functions small and focused
- Write tests for new functionality

### Commit Messages

- Use present tense: "Add feature" not "Added feature"
- Use imperative mood: "Move cursor to..." not "Moves cursor to..."
- Limit first line to 72 characters
- Reference issues and pull requests when applicable

## Troubleshooting

### Common Issues

1. **Port already in use:**

   ```bash
   # Stop conflicting services
   docker ps
   docker stop <container-id>
   ```

2. **Database locked:**

   ```bash
   # Restart the application
   make restart
   ```

3. **Permission denied:**

   ```bash
   # Check file permissions
   ls -la data/
   # Fix if needed
   sudo chown -R $USER:$USER data/
   ```

### Logs

```bash
# View application logs
make logs

# Follow logs in real-time
docker compose logs -f vega-ai
```

## Cloud Mode Deployment

### Overview

Cloud mode enables multi-user deployments with:

- OAuth-only authentication (no password login)
- Multi-tenant data isolation
- Shared reference data (companies)

### Configuration

Enable cloud mode with environment variables:

```bash
# Required for cloud mode
CLOUD_MODE=true
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_CLIENT_REDIRECT_URL=https://yourdomain.com/auth/google/callback
```

### Building Cloud Images

#### Manual Build

```bash
# Via GitHub Actions
# Go to Actions → "Build and Push Cloud Docker Image"
# Click "Run workflow" and choose tag

# Via git tags
git tag v1.0.0-cloud
git push origin v1.0.0-cloud
```

#### Running Cloud Mode

```bash
docker run -d \
  -p 8765:8765 \
  -v vega-data:/app/data \
  -e CLOUD_MODE=true \
  -e GOOGLE_CLIENT_ID=your-client-id \
  -e GOOGLE_CLIENT_SECRET=your-client-secret \
  -e AI_KEY=your-api-key \
  -e TOKEN_SECRET=your-jwt-secret \
  ghcr.io/benidevo/vega-ai:latest
```

### Development Testing

```bash
# Using docker-compose
CLOUD_MODE=true \
GOOGLE_CLIENT_ID=your-dev-client-id \
GOOGLE_CLIENT_SECRET=your-dev-secret \
AI_KEY=your-api-key \
TOKEN_SECRET=your-jwt-secret \
docker compose up

# Run tests with cloud mode
CLOUD_MODE=true go test ./...
```

### Quota System

#### Overview

Vega AI implements a quota system to manage resource usage in cloud deployments:

- **AI Analysis**: 10 analyses per month (per user)
- **Job Search**: Unlimited (tracking only, no limits enforced)

#### Configuration

Quotas are stored in the `quota_configs` table and can be modified via direct database access:

```sql
-- View current quota limits
SELECT * FROM quota_configs;

-- Update quota limits
UPDATE quota_configs SET free_limit = 10 WHERE quota_type = 'ai_analysis_monthly';
```

#### Admin Users

Admin users have unlimited quotas in cloud mode. To grant admin access:

```sql
-- Make a user admin
UPDATE users SET role = 'admin' WHERE username = 'user@example.com';
```

#### Quota Types

| Quota Type | Period | Default Limit | Description |
|------------|--------|---------------|-------------|
| `ai_analysis_monthly` | Monthly | 10 | AI job analysis quota |

#### Self-Hosted Mode

In self-hosted mode (`CLOUD_MODE=false`), all quotas are unlimited by default.

### Data Architecture

#### Storage Model

- **Database**: SQLite with multi-tenant support
- **User Isolation**: Row-level security with user_id
- **Shared Data**: Companies table (reference data)
- **User Data**: Jobs, profiles, match results (isolated)

#### Authentication Flow

1. User redirected to Google OAuth
2. Google returns with user info
3. JWT token created with user claims
4. All requests filtered by user_id

### Multi-Tenant Considerations

1. **Data Isolation**
   - All queries automatically filtered by user_id
   - Repository pattern enforces boundaries
   - See `internal/job/repository/` for implementation

2. **Shared Resources**
   - Companies are reference data (shared)
   - Prevents duplicate "Google Inc" entries
   - Similar to how job boards handle companies

3. **Security**
   - No passwords stored (OAuth only)
   - JWT tokens for session management
   - Automatic user_id injection in middleware
