# Vega AI Production Deployment Guide

This guide covers setting up a fresh Hetzner Cloud VM to run Vega AI in production.

## Prerequisites

- Hetzner Cloud account
- Domain name with Cloudflare DNS
- SSH key pair generated locally
- Google OAuth credentials
- AI provider key (`AI_KEY`) Gemini, OpenAI, or any OpenAI-compatible provider

## Step 1: Create Hetzner Cloud VM

1. **Server Type**: CPX21 (3 vCPU AMD, 4GB RAM, 80GB SSD)
2. **Location**: Choose based on your users (Falkenstein for EU, Ashburn for US)
3. **OS**: Ubuntu 24.04
4. **SSH Key**: Add your public SSH key during creation
5. **Firewall**: Create firewall with these rules:
   - SSH (port 22) - from any IP
   - HTTP (port 80) - from any IP
   - HTTPS (port 443) - from any IP

## Step 2: Initial Server Setup

SSH into your server as root:

```bash
ssh -i ~/.ssh/your_key root@<YOUR_SERVER_IP>
```

Create a non-root user:

```bash
# Create user
adduser vega --disabled-password --gecos ""

# Add to sudo group
usermod -aG sudo vega

# Install Docker
curl -fsSL https://get.docker.com | sh

# Add vega to docker group
usermod -aG docker vega

# Set up SSH for vega user
mkdir -p /home/vega/.ssh
cp /root/.ssh/authorized_keys /home/vega/.ssh/
chown -R vega:vega /home/vega/.ssh
chmod 700 /home/vega/.ssh
chmod 600 /home/vega/.ssh/authorized_keys

# Allow passwordless sudo for deployment
echo "vega ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Update system
apt update && apt upgrade -y

# Exit and reconnect as vega user
exit
```

## Step 3: Application Setup

SSH as vega user:

```bash
ssh -i ~/.ssh/your_key vega@<YOUR_SERVER_IP>
```

Create application structure:

```bash
# Create app directory
sudo mkdir -p /opt/vega-ai
sudo chown -R vega:vega /opt/vega-ai
cd /opt/vega-ai

# Create docker-compose.yml
cat > docker-compose.yml << 'EOF'
services:
  app:
    image: ghcr.io/benidevo/vega-ai:${IMAGE_TAG:-latest}
    container_name: vega-ai-app
    user: "0:0"  # Run as root to avoid permission issues
    restart: always
    env_file:
      - .env.production
    environment:
      - DB_CONNECTION_STRING=/app/data/vega.db?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON&_cache_size=10000&_synchronous=NORMAL
    volumes:
      - vega-prod-data:/app/data
    ports:
      - "80:8765"
    networks:
      - vega

  db-dashboard:
    image: coleifer/sqlite-web:latest
    container_name: vega-ai-db-dashboard
    restart: unless-stopped
    volumes:
      - vega-prod-data:/data
    command: sqlite_web -H 0.0.0.0 -p 8080 /data/vega.db
    depends_on:
      - app
    ports:
      - "127.0.0.1:8081:8080"  # Only accessible via SSH tunnel
    networks:
      - vega

networks:
  vega:

volumes:
  vega-prod-data:
EOF
```

## Step 4: Environment Configuration

Create `.env.production`:

```bash
cat > .env.production << 'EOF'
# JWT Token Secret - Generate with: openssl rand -base64 32
TOKEN_SECRET=<GENERATED_SECRET>

# AI Provider Key (required): works with Gemini, OpenAI, or any OpenAI-compatible provider
# GEMINI_API_KEY is deprecated but still works for backward compatibility
AI_KEY=<YOUR_API_KEY>

# Google OAuth Configuration (required)
GOOGLE_CLIENT_ID=<YOUR_GOOGLE_CLIENT_ID>
GOOGLE_CLIENT_SECRET=<YOUR_GOOGLE_CLIENT_SECRET>
GOOGLE_CLIENT_REDIRECT_URL=https://<YOUR_DOMAIN>/auth/google/callback

# Enable cloud mode
CLOUD_MODE=true

# Cookie domain
COOKIE_DOMAIN=<YOUR_DOMAIN>
EOF

# Secure the file
chmod 600 .env.production
```

## Step 5: Start Application

```bash
# Pull and start containers
docker compose up -d

# Check logs
docker compose logs -f app
```

## Step 6: Cloudflare Configuration

1. **Add DNS Record**:
   - Type: A
   - Name: @ (or subdomain)
   - Content: <YOUR_SERVER_IP>
   - Proxy: ON (orange cloud)

2. **SSL/TLS Settings**:
   - Go to SSL/TLS → Overview
   - Set to **"Flexible"**
   - This allows HTTPS to visitors while your server only needs HTTP

3. **Security Settings**:
   - Firewall → Security Level: Medium
   - SSL/TLS → Edge Certificates → Always Use HTTPS: ON

## Maintenance

### View Logs

```bash
docker compose logs -f app
```

### Access SQLite Web UI

```bash
# From your local machine
ssh -L 8081:localhost:8081 vega@<YOUR_SERVER_IP>
# Visit http://localhost:8081
```

### Manual Updates

```bash
cd /opt/vega-ai
docker compose pull
docker compose down
docker compose up -d
```

### Backup Database

```bash
docker compose exec app sqlite3 /app/data/vega.db ".backup /app/data/backup-$(date +%Y%m%d).db"
```

## Troubleshooting

### Permission Issues

If you see database permission errors, the container is already configured to run as root (user: "0:0") to avoid these issues.

### Domain Not Working

- Check DNS propagation: `nslookup <YOUR_DOMAIN>`
- Ensure Cloudflare SSL mode is "Flexible"
- Test with curl: `curl -H "Host: <YOUR_DOMAIN>" http://<YOUR_SERVER_IP>/`

### Port 80 Issues

- Check firewall: `sudo ufw status`
- Check what's listening: `sudo netstat -tlnp | grep :80`
- Ensure docker-proxy is running on port 80

## Security Notes

- The app runs as root in the container for simplicity (avoids permission issues)
- SQLite dashboard is only accessible via SSH tunnel
- All secrets are in `.env.production` with restricted permissions
- Cloudflare provides DDoS protection and SSL termination
