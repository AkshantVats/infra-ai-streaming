#!/bin/bash
# =============================================================================
# gmail_send.sh — Send email via Gmail API using OAuth2 refresh token
# =============================================================================
#
# REQUIRED ENVIRONMENT VARIABLES:
#   GMAIL_REFRESH_TOKEN   — OAuth2 refresh token obtained during initial
#                           authorization flow
#   GMAIL_CLIENT_ID       — OAuth2 client ID from Google Cloud Console
#   GMAIL_CLIENT_SECRET   — OAuth2 client secret from Google Cloud Console
#
# HOW TO SET UP GMAIL OAUTH2 CREDENTIALS:
#   1. Go to https://console.cloud.google.com/
#   2. Create or select a project.
#   3. Enable the Gmail API under "APIs & Services > Library".
#   4. Go to "APIs & Services > Credentials" and create an OAuth 2.0 Client ID
#      (type: Desktop application).
#   5. Note the client_id and client_secret.
#   6. Run the OAuth2 authorization flow to obtain a refresh token. You need
#      the scope: https://www.googleapis.com/auth/gmail.send
#      Example using oauth2l or a manual browser-based flow:
#        https://accounts.google.com/o/oauth2/auth?client_id=CLIENT_ID
#          &redirect_uri=urn:ietf:wg:oauth:2.0:oob
#          &response_type=code
#          &scope=https://www.googleapis.com/auth/gmail.send
#      Exchange the returned code for tokens via:
#        curl -X POST https://oauth2.googleapis.com/token \
#          -d code=AUTH_CODE \
#          -d client_id=CLIENT_ID \
#          -d client_secret=CLIENT_SECRET \
#          -d redirect_uri=urn:ietf:wg:oauth:2.0:oob \
#          -d grant_type=authorization_code
#      Save the "refresh_token" from the response.
#   7. Export the three variables in your shell or secret manager before
#      running this script.
#
# CALLER:
#   This script is intended to be called by the daily routine agent as part
#   of automated reporting and notification workflows.
#
# USAGE:
#   gmail_send.sh --to recipient@example.com \
#                 --subject "Subject line" \
#                 --body "Plain text body (markdown accepted)"
#
# EXIT CODES:
#   0 — Email sent successfully
#   1 — Error (missing args, missing env vars, network failure, API error)
# =============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
TO=""
SUBJECT=""
BODY=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --to)
      TO="${2:-}"
      shift 2
      ;;
    --subject)
      SUBJECT="${2:-}"
      shift 2
      ;;
    --body)
      BODY="${2:-}"
      shift 2
      ;;
    *)
      echo "ERROR: Unknown argument: $1" >&2
      echo "Usage: $0 --to <email> --subject <subject> --body <body>" >&2
      exit 1
      ;;
  esac
done

# ---------------------------------------------------------------------------
# Validate required arguments
# ---------------------------------------------------------------------------
if [[ -z "$TO" ]]; then
  echo "ERROR: --to is required" >&2
  exit 1
fi
if [[ -z "$SUBJECT" ]]; then
  echo "ERROR: --subject is required" >&2
  exit 1
fi
if [[ -z "$BODY" ]]; then
  echo "ERROR: --body is required" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Validate required environment variables
# ---------------------------------------------------------------------------
if [[ -z "${GMAIL_REFRESH_TOKEN:-}" ]]; then
  echo "ERROR: GMAIL_REFRESH_TOKEN environment variable is not set." >&2
  echo "  Set it to the OAuth2 refresh token for the Gmail API." >&2
  exit 1
fi
if [[ -z "${GMAIL_CLIENT_ID:-}" ]]; then
  echo "ERROR: GMAIL_CLIENT_ID environment variable is not set." >&2
  echo "  Set it to your Google OAuth2 client ID." >&2
  exit 1
fi
if [[ -z "${GMAIL_CLIENT_SECRET:-}" ]]; then
  echo "ERROR: GMAIL_CLIENT_SECRET environment variable is not set." >&2
  echo "  Set it to your Google OAuth2 client secret." >&2
  exit 1
fi

FROM="akshant3@gmail.com"
TOKEN_ENDPOINT="https://oauth2.googleapis.com/token"
GMAIL_SEND_ENDPOINT="https://gmail.googleapis.com/gmail/v1/users/me/messages/send"

# ---------------------------------------------------------------------------
# Exchange refresh token for access token
# ---------------------------------------------------------------------------
echo "Obtaining access token..." >&2

TOKEN_RESPONSE=$(curl --silent --show-error --fail-with-body \
  --write-out "\n%{http_code}" \
  -X POST "${TOKEN_ENDPOINT}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=refresh_token" \
  --data-urlencode "client_id=${GMAIL_CLIENT_ID}" \
  --data-urlencode "client_secret=${GMAIL_CLIENT_SECRET}" \
  --data-urlencode "refresh_token=${GMAIL_REFRESH_TOKEN}" \
  2>&1) || {
  echo "ERROR: curl failed when contacting token endpoint." >&2
  exit 1
}

TOKEN_HTTP_CODE=$(echo "${TOKEN_RESPONSE}" | tail -n1)
TOKEN_BODY=$(echo "${TOKEN_RESPONSE}" | head -n -1)

if [[ "${TOKEN_HTTP_CODE}" != "200" ]]; then
  echo "ERROR: Token exchange failed with HTTP ${TOKEN_HTTP_CODE}." >&2
  echo "Response body: ${TOKEN_BODY}" >&2
  exit 1
fi

# Extract access_token from JSON using only bash + sed (no jq dependency)
ACCESS_TOKEN=$(echo "${TOKEN_BODY}" | sed -n 's/.*"access_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')

if [[ -z "${ACCESS_TOKEN}" ]]; then
  echo "ERROR: Could not parse access_token from token response." >&2
  echo "Response body: ${TOKEN_BODY}" >&2
  exit 1
fi

echo "Access token obtained successfully." >&2

# ---------------------------------------------------------------------------
# Construct the RFC 2822 raw email message
# ---------------------------------------------------------------------------
RAW_EMAIL="From: ${FROM}
To: ${TO}
Subject: ${SUBJECT}
Content-Type: text/plain; charset=utf-8
MIME-Version: 1.0

${BODY}"

# ---------------------------------------------------------------------------
# Base64url-encode the message
# (base64 -w0 produces standard base64; then swap +/ -> -_ for URL safety)
# ---------------------------------------------------------------------------
ENCODED_MESSAGE=$(printf '%s' "${RAW_EMAIL}" | base64 -w0 | tr '+/' '-_')

# ---------------------------------------------------------------------------
# Send the message via Gmail API
# ---------------------------------------------------------------------------
echo "Sending email to ${TO}..." >&2

SEND_RESPONSE=$(curl --silent --show-error \
  --write-out "\n%{http_code}" \
  -X POST "${GMAIL_SEND_ENDPOINT}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  --data "{\"raw\": \"${ENCODED_MESSAGE}\"}" \
  2>&1) || {
  echo "ERROR: curl failed when contacting Gmail send endpoint." >&2
  exit 1
}

SEND_HTTP_CODE=$(echo "${SEND_RESPONSE}" | tail -n1)
SEND_BODY=$(echo "${SEND_RESPONSE}" | head -n -1)

if [[ "${SEND_HTTP_CODE}" != "200" ]]; then
  echo "ERROR: Gmail API send failed with HTTP ${SEND_HTTP_CODE}." >&2
  echo "Response body: ${SEND_BODY}" >&2
  exit 1
fi

echo "Email sent successfully to ${TO}." >&2
exit 0
