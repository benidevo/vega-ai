# Security Policy

## Reporting a Vulnerability

**Please do not open public GitHub issues for security vulnerabilities.**

### How to Report

- **Email:** [vega@benidevo.com](mailto:vega@benidevo.com)
- **GitHub:** [Private Security Advisory](https://github.com/benidevo/vega-ai/security/advisories/new)

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

| Severity | Acknowledgment | Resolution Target |
| -------- | -------------- | ----------------- |
| Critical | 24 hours       | 7 days            |
| High     | 48 hours       | 14 days           |
| Medium/Low | 48 hours     | 30 days           |

## Supported Versions

Only the latest release receives security updates. We recommend always running the most recent version.

## Scope

### In Scope

- Authentication/authorization bypasses
- Cross-tenant data exposure
- Injection vulnerabilities (SQL, XSS, command)
- Sensitive data leaks in logs or responses
- AI prompt injection affecting other users

### Out of Scope

- Self-hosted misconfiguration issues
- Third-party dependency vulnerabilities (report upstream)
- Denial of service (unless causing data loss)

## Further Reading

- [Architecture & Security Design](docs/ARCHITECTURE.md)
- [Deployment Security](docs/DEPLOYMENT_GUIDE.md)
