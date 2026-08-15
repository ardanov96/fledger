#!/usr/bin/env bash
# =============================================================================
# Fly.io One-Command Deploy — FMCG Wallet Demo
# =============================================================================
#
# Purpose: First-time + subsequent deploys to Fly.io for interview demo.
# Cost target: $0/month (free tier — DO NOT modify vm.size in fly.toml).
#
# Prerequisites:
#   - Fly.io account (free): https://fly.io/app/sign-up
#   - flyctl installed: https://fly.io/docs/hands-on/install-flyctl/
#   - Logged in: fly auth login
#
# Usage:
#   ./scripts/fly-deploy.sh              # full first-time deploy
#   ./scripts/fly-deploy.sh --quick      # subsequent deploys (no re-prompt)
#   ./scripts/fly-deploy.sh --only-secrets  # just update secrets
#
# What this does:
#   1. Verify prerequisites
#   2. Create Fly app (if not exists)
#   3. Create persistent volume (if not exists)
#   4. Set secrets (JWT_SECRET, DB_PASSWORD)
#   5. Deploy (build + push + release)
#   6. Wait for health check
#   7. Print demo URL + credentials
# =============================================================================

set -euo pipefail

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------
APP_NAME="${FLY_APP_NAME:-fmcg-wallet-demo}"
REGION="${FLY_REGION:-sin}"
VOLUME_NAME="pg_data"
VOLUME_SIZE="${FLY_VOLUME_SIZE:-1}"   # GB — within free tier (max 3GB)
QUICK_MODE=false
SECRETS_ONLY=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# -----------------------------------------------------------------------------
# Parse arguments
# -----------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
    case $1 in
        --quick) QUICK_MODE=true; shift ;;
        --only-secrets) SECRETS_ONLY=true; shift ;;
        -h|--help)
            echo "Usage: $0 [--quick] [--only-secrets]"
            echo ""
            echo "Options:"
            echo "  --quick          Skip confirmation prompts (for CI/CD)"
            echo "  --only-secrets   Only update secrets, don't redeploy"
            exit 0
            ;;
        *) echo -e "${RED}Unknown option: $1${NC}"; exit 1 ;;
    esac
done

# -----------------------------------------------------------------------------
# Helper functions
# -----------------------------------------------------------------------------
log_info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $*"; }

# -----------------------------------------------------------------------------
# Step 1: Verify prerequisites
# -----------------------------------------------------------------------------
log_info "Checking prerequisites..."

if ! command -v fly >/dev/null 2>&1; then
    log_error "flyctl not installed. Install: https://fly.io/docs/hands-on/install-flyctl/"
    exit 1
fi

if ! fly auth whoami >/dev/null 2>&1; then
    log_error "Not logged in to Fly.io. Run: fly auth login"
    exit 1
fi

FLY_USER=$(fly auth whoami)
log_success "Logged in as: $FLY_USER"

# -----------------------------------------------------------------------------
# Step 2: Create Fly app (if not exists)
# -----------------------------------------------------------------------------
if fly apps list | grep -q "^${APP_NAME}"; then
    log_success "App '${APP_NAME}' already exists"
else
    if [ "$QUICK_MODE" = true ]; then
        log_info "Creating app '${APP_NAME}'..."
        fly apps create "${APP_NAME}" --org personal
    else
        echo ""
        log_warn "About to create Fly app '${APP_NAME}' in region '${REGION}'"
        read -p "Continue? (y/N) " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_error "Aborted"
            exit 1
        fi
        fly apps create "${APP_NAME}" --org personal
    fi
    log_success "App created"
fi

# -----------------------------------------------------------------------------
# Secrets-only mode: update secrets and exit
# -----------------------------------------------------------------------------
if [ "$SECRETS_ONLY" = true ]; then
    log_info "Updating secrets only..."
    JWT_SECRET=$(openssl rand -hex 32)
    fly secrets set \
        JWT_SECRET="${JWT_SECRET}" \
        DB_PASSWORD="fmcg_demo_password" \
        --app "${APP_NAME}"
    log_success "Secrets updated"
    exit 0
fi

# -----------------------------------------------------------------------------
# Step 3: Create persistent volume (if not exists)
# -----------------------------------------------------------------------------
if fly volumes list --app "${APP_NAME}" 2>/dev/null | grep -q "${VOLUME_NAME}"; then
    log_success "Volume '${VOLUME_NAME}' already exists"
else
    log_info "Creating volume '${VOLUME_NAME}' (${VOLUME_SIZE}GB in ${REGION})..."
    fly volumes create "${VOLUME_NAME}" --size "${VOLUME_SIZE}" --region "${REGION}" --app "${APP_NAME}"
    log_success "Volume created"
fi

# -----------------------------------------------------------------------------
# Step 4: Set secrets (JWT_SECRET generated, DB_PASSWORD hardcoded for demo)
# -----------------------------------------------------------------------------
log_info "Setting secrets..."
JWT_SECRET=$(openssl rand -hex 32)
fly secrets set \
    JWT_SECRET="${JWT_SECRET}" \
    DB_PASSWORD="fmcg_demo_password" \
    --app "${APP_NAME}"
log_success "Secrets set (JWT_SECRET regenerated, DB_PASSWORD=fmcg_demo_password)"

# -----------------------------------------------------------------------------
# Step 5: Deploy
# -----------------------------------------------------------------------------
log_info "Deploying to Fly.io..."
log_info "(First deploy may take 3-5 minutes due to image build + Postgres init)"

if [ "$QUICK_MODE" = true ]; then
    fly deploy --app "${APP_NAME}" --strategy rolling
else
    fly deploy --app "${APP_NAME}" --strategy rolling
fi

log_success "Deploy command completed"

# -----------------------------------------------------------------------------
# Step 6: Wait for health check
# -----------------------------------------------------------------------------
APP_URL="https://${APP_NAME}.fly.dev"
log_info "Waiting for health check at ${APP_URL}/healthz..."

RETRY_COUNT=0
MAX_RETRIES=30
until curl -sf "${APP_URL}/healthz" >/dev/null 2>&1; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        log_warn "Health check failed after ${MAX_RETRIES} retries"
        log_warn "Check status: fly status --app ${APP_NAME}"
        log_warn "Check logs:   fly logs --app ${APP_NAME}"
        exit 1
    fi
    echo "  Waiting... ($RETRY_COUNT/$MAX_RETRIES)"
    sleep 5
done

log_success "Health check passed!"

# -----------------------------------------------------------------------------
# Step 7: Print demo info
# -----------------------------------------------------------------------------
echo ""
echo "============================================="
echo "🚀 FMCG Wallet Demo — Deployed!"
echo "============================================="
echo ""
echo "URL:       ${APP_URL}"
echo "Health:    ${APP_URL}/healthz"
echo "Ready:     ${APP_URL}/readyz"
echo "Metrics:   ${APP_URL}/metrics"
echo ""
echo "Demo login credentials:"
echo "  admin@demo.fmcg-wallet  /  demo123  (role: hq_admin)"
echo "  sales@demo.fmcg-wallet  /  demo123  (role: sales_rep)"
echo ""
echo "Useful commands:"
echo "  fly logs --app ${APP_NAME}              # tail logs"
echo "  fly status --app ${APP_NAME}            # check status"
echo "  fly ssh console --app ${APP_NAME}       # SSH into machine"
echo "  fly volumes snapshots create ${VOLUME_NAME}  # backup DB"
echo ""
echo "Runbook: docs/runbooks/deployment-fly.md"
echo "============================================="
