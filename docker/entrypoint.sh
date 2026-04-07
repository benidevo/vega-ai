#!/bin/sh
set -e

# Set COOKIE_SECURE based on mode
if [ "$CLOUD_MODE" = "true" ]; then
    # Cloud mode MUST use secure cookies for security
    export COOKIE_SECURE="true"
    echo "Cloud mode: Enforcing COOKIE_SECURE=true"
else
    # Regular mode: respect user settings, default to false if not set
    if [ -z "$COOKIE_SECURE" ]; then
        export COOKIE_SECURE="false"
    fi
fi

echo "Starting Vega AI..."
echo "Mode: $([ "$CLOUD_MODE" = "true" ] && echo "Cloud" || echo "Self-hosted")"

exec "$@"
