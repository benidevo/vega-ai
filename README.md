# Vega AI

[![CI](https://github.com/benidevo/vega-ai/workflows/CI/badge.svg)](https://github.com/benidevo/vega-ai/actions/workflows/ci.yaml)
[![Docker](https://github.com/benidevo/vega-ai/workflows/Build%20and%20Push%20Docker%20Image/badge.svg)](https://github.com/benidevo/vega-ai/actions/workflows/docker-build.yml)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![GitHub Container Registry](https://img.shields.io/badge/ghcr.io-vega--ai-blue)](https://github.com/benidevo/vega-ai/pkgs/container/vega-ai)

Vega AI is a self-hostable job search assistant. Track applications, generate tailored CVs and cover letters, get AI-powered job match scores, and capture jobs from LinkedIn via a browser extension.

Cloud instance: [vega.benidevo.com](https://vega.benidevo.com)

## Self-Hosted Quick Start

Requires Docker and an API key for any OpenAI-compatible provider (or a local Ollama instance).

### 1. Choose Your AI Provider

Vega AI works with any OpenAI-compatible provider. Pick one:

| Provider | Cost | Privacy | Setup |
|---|---|---|---|
| **Gemini** (default) | Free tier available | Cloud | [Get API key](https://aistudio.google.com/app/apikey) |
| **OpenAI** | Paid | Cloud | [Get API key](https://platform.openai.com/api-keys) |
| **Ollama** | Free | 100% local | [Install Ollama](https://ollama.com) |
| **LM Studio** | Free | 100% local | [Install LM Studio](https://lmstudio.ai) |

### 2. Create Configuration

**Gemini (quickest cloud start):**

```bash
mkdir vega-ai && cd vega-ai
echo "AI_KEY=your-gemini-api-key" > config
```

> `GEMINI_API_KEY` still works but is deprecated. Use `AI_KEY` instead.

**OpenAI:**

```bash
mkdir vega-ai && cd vega-ai
cat > config <<EOF
AI_PROVIDER=openai
AI_KEY=sk-your-openai-key
AI_MODEL=gpt-4o-mini
EOF
```

**Ollama (fully local, no API key):**

```bash
# First install and pull a model
ollama pull llama3.2

mkdir vega-ai && cd vega-ai
cat > config <<EOF
AI_PROVIDER=openai
AI_KEY=ollama
AI_BASE_URL=http://host.docker.internal:11434/v1
AI_MODEL=llama3.2
EOF
```

### Supported Models

Models must support **JSON mode** (`response_format: json_object`). Instruct-tuned models from 2025 onwards reliably do. For example `gemini-2.5-flash`, `gpt-4o-mini`, or `llama3.2` via Ollama.

> **Note:** Models under ~3B parameters may produce inconsistent JSON output. 7B+ instruct-tuned models are recommended for reliable results. Older or custom GGUF models that don't support JSON mode will fail with a parse error.

### 3. Run with Docker

Run the container:

```bash
docker run --pull always -d \
  --name vega-ai \
  -p 8765:8765 \
  -v vega-data:/app/data \
  --env-file config \
  ghcr.io/benidevo/vega-ai:latest
```

### 4. Access Vega AI

1. Visit <http://localhost:8765>
2. Log in with default credentials:
   - Username: `admin`
   - Password: `VegaAdmin`
3. **Important:** Change your password after first login via Settings → Account

## Features

- **CV and cover letter generation**: AI-generated documents tailored to a specific job description
- **Job match scoring**: AI analysis of how well your profile matches a job posting
- **CV parsing**: Upload a CV to auto-populate your profile
- **Job tracking**: Manage applications with customizable statuses
- **Browser extension**: Capture jobs from LinkedIn and other boards in one click
- **Self-hosted**: All data stays on your machine; no third-party storage
- **Cloud mode**: Hosted instance with per-user AI usage quotas

## Browser Extension

Download the **Vega AI Job Capture** extension from [GitHub Releases](https://github.com/benidevo/vega-ai-extension/releases/latest) for one-click job capture from LinkedIn.

### Installation Steps

1. Download the latest `.zip` file from the [releases page](https://github.com/benidevo/vega-ai-extension/releases/latest)
2. Extract the ZIP file to a folder on your computer
3. Open Chrome and navigate to `chrome://extensions/`
4. Enable "Developer mode" in the top right
5. Click "Load unpacked" and select the extracted folder

For development or to build from source, visit the [extension repository](https://github.com/benidevo/vega-ai-extension).

## Docker Options

### ARM64 Support (Apple Silicon)

The Docker images support both AMD64 and ARM64 architectures:

```bash
# Works on both Intel/AMD and ARM processors
docker pull ghcr.io/benidevo/vega-ai:latest
```

### Docker Compose

For easier management, use Docker Compose:

```yaml
# docker-compose.yml
services:
  vega-ai:
    image: ghcr.io/benidevo/vega-ai:latest
    ports:
      - "8765:8765"
    volumes:
      - vega-data:/app/data
    env_file:
      - config
    restart: unless-stopped

volumes:
  vega-data:
```

Then run: `docker compose up -d`

### Docker Swarm

Docker Stack Deploy doesn't read `.env` files ([known limitation](https://github.com/moby/moby/issues/29133)). Use one of these approaches:

```bash
# Option 1: Process env file first
docker-compose config | docker stack deploy -c - vega-stack

# Option 2: Export variables manually
export $(cat config | xargs)
docker stack deploy -c docker-compose.yml vega-stack
```

See [docs/DOCKER_SWARM.md](docs/DOCKER_SWARM.md) for detailed instructions.

### Advanced Configuration

- **Docker Secrets**: Use `_FILE` environment variables for secure configuration. See [Docker Swarm deployment](docs/DOCKER_SWARM.md#method-3-using-docker-secrets-recommended-for-production).
- **Development Setup**: Custom ports, SSL, external databases. See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Development

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for development setup, testing, and contributing guidelines.

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0).

What this means:

- You can use, study, modify, and distribute the code
- If you run this software on a server, you must make your source code available to users
- Any modifications must also be released under AGPL-3.0

**Commercial licensing:** For commercial use without AGPL restrictions, contact <vega@benidevo.com> for licensing options.
