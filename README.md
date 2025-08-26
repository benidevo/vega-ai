# 🌟 Vega AI

[![CI](https://github.com/benidevo/vega-ai/workflows/CI/badge.svg)](https://github.com/benidevo/vega-ai/actions/workflows/ci.yaml)
[![Docker](https://github.com/benidevo/vega-ai/workflows/Build%20and%20Push%20Docker%20Image/badge.svg)](https://github.com/benidevo/vega-ai/actions/workflows/docker-build.yml)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![GitHub Container Registry](https://img.shields.io/badge/ghcr.io-vega--ai-blue)](https://github.com/benidevo/vega-ai/pkgs/container/vega-ai)

> Navigate your career journey with AI-powered precision

Just as ancient navigators used the star Vega to find their way, Vega AI helps you navigate your career journey with intelligent job search tools. Track applications, generate tailored documents using AI, get smart job matching based on your profile, and capture opportunities from LinkedIn with the browser extension.

**🚀 Try it now:** Visit [vega.benidevo.com](https://vega.benidevo.com) for the cloud mode, or self-host for complete data privacy.

## 🚀 Self-Hosted Quick Start

Self-hosting Vega AI gives you complete control over your data. You only need a Gemini API key to get started.

### 1. Get Your API Key

Get your [free Gemini API key](https://aistudio.google.com/app/apikey) from Google AI Studio.

### 2. Create Configuration

```bash
# Create a directory for Vega AI
mkdir vega-ai && cd vega-ai

# Create a config file with your Gemini API key
echo "GEMINI_API_KEY=your-gemini-api-key" > config
```

That's it! No complex setup required.

### 3. Run with Docker

Start Vega AI with a single command:

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

## ✨ Features

- **🤖 AI Document Generation**: Generate tailored cover letters and CVs based on your profile
- **📊 Smart Job Matching**: Get AI-powered match scores and detailed analysis for job compatibility
- **📝 CV Parsing**: Upload your existing CV to automatically populate your profile
- **💼 Job Management**: Track job applications with customizable statuses
- **🔗 Browser Extension**: One-click job capture from LinkedIn and other job boards
- **👤 Profile Management**: Comprehensive professional profile with experience, education, and skills
- **🔒 Privacy-First**: Self-hosted option with complete data control
- **📊 Usage Quotas**: Fair usage limits for AI features (cloud mode)

## 🔗 Browser Extension

Download the **Vega AI Job Capture** extension from [GitHub Releases](https://github.com/benidevo/vega-ai-extension/releases/latest) for one-click job capture from LinkedIn.

### Installation Steps:
1. Download the latest `.zip` file from the [releases page](https://github.com/benidevo/vega-ai-extension/releases/latest)
2. Extract the ZIP file to a folder on your computer
3. Open Chrome and navigate to `chrome://extensions/`
4. Enable "Developer mode" in the top right
5. Click "Load unpacked" and select the extracted folder

For development or to build from source, visit the [extension repository](https://github.com/benidevo/vega-ai-extension).

## 🐳 Docker Options

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

## 🛠️ Development

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for development setup, testing, and contributing guidelines.

## 📝 License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0).

What this means:

- ✅ You can use, study, modify, and distribute the code
- ✅ If you run this software on a server, you must make your source code available to users
- ✅ Any modifications must also be released under AGPL-3.0

**Commercial licensing:** For commercial use without AGPL restrictions, contact <vega@benidevo.com> for licensing options.
