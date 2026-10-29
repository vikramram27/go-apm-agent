#!/usr/bin/env bash
set -e

PORT=${1:-9933}
BASE_URL="http://localhost:${PORT}/api/v1"
HEALTH_URL="http://localhost:${PORT}/health"
APP_NAME="notes-api (Native net/http on port ${PORT})"

# Color formatting
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

pass_count=0
fail_count=0

assert_status() {
    local test_name="$1"
    local expected_status="$2"
    local actual_status="$3"
    local body="$4"

    if [ "$actual_status" -eq "$expected_status" ]; then
        echo -e "  [${GREEN}PASS${NC}] $test_name (HTTP $actual_status as expected)"
        pass_count=$((pass_count + 1))
    else
        echo -e "  [${RED}FAIL${NC}] $test_name (Expected HTTP $expected_status, got $actual_status)"
        echo -e "         Response: $body"
        fail_count=$((fail_count + 1))
        exit 1
    fi
}

assert_status_one_of() {
    local test_name="$1"
    local allowed="$2"
    local actual_status="$3"
    local body="$4"

    local matched=0
    for s in $allowed; do
        if [ "$actual_status" -eq "$s" ]; then
            matched=1
            break
        fi
    done

    if [ "$matched" -eq 1 ]; then
        echo -e "  [${GREEN}PASS${NC}] $test_name (HTTP $actual_status as expected in [$allowed])"
        pass_count=$((pass_count + 1))
    else
        echo -e "  [${RED}FAIL${NC}] $test_name (Expected one of [$allowed], got $actual_status)"
        echo -e "         Response: $body"
        fail_count=$((fail_count + 1))
        exit 1
    fi
}

echo -e "${BLUE}========================================================================${NC}"
echo -e "${BLUE} Starting Full Verification Suite for: ${APP_NAME}${NC}"
echo -e "${BLUE} Base URL: ${BASE_URL}${NC}"
echo -e "${BLUE}========================================================================${NC}"

SUFFIX=$(date +%s)
ALICE_USER="alice_${SUFFIX}"
ALICE_EMAIL="alice_${SUFFIX}@example.com"
BOB_USER="bob_${SUFFIX}"
BOB_EMAIL="bob_${SUFFIX}@example.com"
CHARLIE_USER="charlie_${SUFFIX}"
CHARLIE_EMAIL="charlie_${SUFFIX}@example.com"
PASSWORD="securePassword123!"

echo -e "\n${BLUE}--- Section 1: Health & System Endpoints ---${NC}"
RESP=$(curl -s -w "\n%{http_code}" -X GET "$HEALTH_URL")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "1.1 Health check (200 OK)" 200 "$STATUS" "$BODY"

echo -e "\n${BLUE}--- Section 2: Registration Validation & 4xx Errors ---${NC}"
# 400 Bad Request: missing password or too short
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"baduser","email":"bad@example.com","password":"123"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "2.1 Register with short password (400 Bad Request)" 400 "$STATUS"

# 400 Bad Request: empty username
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"","email":"bad2@example.com","password":"password123"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "2.2 Register with empty username (400 Bad Request)" 400 "$STATUS"

echo -e "\n${BLUE}--- Section 3: User Registration (201 Created) ---${NC}"
# Register Alice
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"'"$ALICE_USER"'","email":"'"$ALICE_EMAIL"'","password":"'"$PASSWORD"'"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "3.1 Register Alice (201 Created)" 201 "$STATUS"

# Register Bob
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"'"$BOB_USER"'","email":"'"$BOB_EMAIL"'","password":"'"$PASSWORD"'"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "3.2 Register Bob (201 Created)" 201 "$STATUS"

# Register Charlie
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"'"$CHARLIE_USER"'","email":"'"$CHARLIE_EMAIL"'","password":"'"$PASSWORD"'"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "3.3 Register Charlie (201 Created)" 201 "$STATUS"

echo -e "\n${BLUE}--- Section 4: Login Validation, 4xx Errors & 200 OK ---${NC}"
# Login with empty password (400 Bad Request or 401 Unauthorized)
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"'"$ALICE_USER"'","password":""}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status_one_of "4.1 Login with empty password (4xx Client Error)" "400 401" "$STATUS"

# 401 Unauthorized: wrong password
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"'"$ALICE_USER"'","password":"wrongPassword!"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "4.2 Login with wrong password (401 Unauthorized)" 401 "$STATUS"

# 200 OK: Alice Login
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"'"$ALICE_USER"'","password":"'"$PASSWORD"'"}')
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "4.3 Login Alice (200 OK)" 200 "$STATUS"
ALICE_TOKEN=$(echo "$BODY" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
ALICE_REFRESH_TOKEN=$(echo "$BODY" | grep -o '"refresh_token":"[^"]*' | cut -d'"' -f4)

# 200 OK: Bob Login
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"'"$BOB_USER"'","password":"'"$PASSWORD"'"}')
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "4.4 Login Bob (200 OK)" 200 "$STATUS"
BOB_TOKEN=$(echo "$BODY" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
BOB_ID=$(echo "$BODY" | grep -o '"id":[0-9]*' | head -n1 | cut -d':' -f2)

# 200 OK: Charlie Login
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"'"$CHARLIE_USER"'","password":"'"$PASSWORD"'"}')
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "4.5 Login Charlie (200 OK)" 200 "$STATUS"
CHARLIE_TOKEN=$(echo "$BODY" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

echo -e "\n${BLUE}--- Section 5: Authorization & Token Verification ---${NC}"
# 401 Unauthorized: missing Authorization header
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/auth/me")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "5.1 Access protected route without token (401 Unauthorized)" 401 "$STATUS"

# 401 Unauthorized: invalid token
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/auth/me" \
  -H "Authorization: Bearer invalid.token.value")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "5.2 Access protected route with invalid token (401 Unauthorized)" 401 "$STATUS"

# 200 OK: Alice GET /auth/me
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/auth/me" \
  -H "Authorization: Bearer $ALICE_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "5.3 Access protected route with valid token (200 OK)" 200 "$STATUS"

echo -e "\n${BLUE}--- Section 6: Notes Creation & Listing ---${NC}"
# 400 Bad Request: Create note with empty title
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/notes" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -d '{"title":"","content":"Empty title should fail"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "6.1 Create note with empty title (400 Bad Request)" 400 "$STATUS"

# 201 Created: Alice creates Note
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/notes" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -d '{"title":"Quarterly Strategy","content":"Secret company roadmap"}')
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "6.2 Alice creates note (201 Created)" 201 "$STATUS"
NOTE_ID=$(echo "$BODY" | grep -o '"id":[0-9]*' | head -n1 | cut -d':' -f2)

# 200 OK: List notes
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/notes" \
  -H "Authorization: Bearer $ALICE_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "6.3 Alice lists notes (200 OK)" 200 "$STATUS"

echo -e "\n${BLUE}--- Section 7: Note Access Control & 4xx Verification ---${NC}"
# 404 Not Found: Get non-existent note
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/notes/999999" \
  -H "Authorization: Bearer $ALICE_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "7.1 Get non-existent note (404 Not Found)" 404 "$STATUS"

# 403 Forbidden: Bob tries to access Alice's unshared note
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/notes/$NOTE_ID" \
  -H "Authorization: Bearer $BOB_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "7.2 Bob tries to read unshared note (403 Forbidden)" 403 "$STATUS"

# 200 OK: Check access endpoint (Alice checks Bob's access)
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/notes/$NOTE_ID/access?target_username=$BOB_USER" \
  -H "Authorization: Bearer $ALICE_TOKEN")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "7.3 Alice checks Bob authorization status (200 OK)" 200 "$STATUS"

# 403 Forbidden: Bob tries to share Alice's note
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/notes/$NOTE_ID/share" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $BOB_TOKEN" \
  -d '{"username":"'"$CHARLIE_USER"'","permission":"read"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "7.4 Non-owner tries to share note (403 Forbidden)" 403 "$STATUS"

echo -e "\n${BLUE}--- Section 8: Sharing with Read Permission ---${NC}"
# 200 OK: Alice shares note with Bob (permission: read)
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/notes/$NOTE_ID/share" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -d '{"username":"'"$BOB_USER"'","permission":"read"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "8.1 Alice shares note with Bob as 'read' (200 OK)" 200 "$STATUS"

# 200 OK: Bob checks his own access status
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/notes/$NOTE_ID/access" \
  -H "Authorization: Bearer $BOB_TOKEN")
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "8.2 Bob verifies note is shared to him (200 OK)" 200 "$STATUS"

# 200 OK: Bob reads note
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/notes/$NOTE_ID" \
  -H "Authorization: Bearer $BOB_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "8.3 Bob reads shared note (200 OK)" 200 "$STATUS"

# 403 Forbidden: Bob tries to update note with 'read' permission
RESP=$(curl -s -w "\n%{http_code}" -X PUT "$BASE_URL/notes/$NOTE_ID" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $BOB_TOKEN" \
  -d '{"title":"Tampered Title","content":"Tampered content"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "8.4 Read-only user attempts update (403 Forbidden)" 403 "$STATUS"

echo -e "\n${BLUE}--- Section 9: Upgrading to Write Permission ---${NC}"
# 200 OK: Alice upgrades Bob to 'write'
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/notes/$NOTE_ID/share" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -d '{"username":"'"$BOB_USER"'","permission":"write"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "9.1 Alice upgrades Bob to 'write' permission (200 OK)" 200 "$STATUS"

# 200 OK: Bob updates note
RESP=$(curl -s -w "\n%{http_code}" -X PUT "$BASE_URL/notes/$NOTE_ID" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $BOB_TOKEN" \
  -d '{"title":"Quarterly Strategy (Updated by Bob)","content":"Updated roadmap notes"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "9.2 Bob updates note with write permission (200 OK)" 200 "$STATUS"

# 403 Forbidden: Charlie still cannot read note
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/notes/$NOTE_ID" \
  -H "Authorization: Bearer $CHARLIE_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "9.3 Unshared user Charlie cannot read note (403 Forbidden)" 403 "$STATUS"

echo -e "\n${BLUE}--- Section 10: Shares List, Revocation & Deletion ---${NC}"
# 200 OK: Alice views shares list
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/notes/$NOTE_ID/shares" \
  -H "Authorization: Bearer $ALICE_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "10.1 Alice lists note shares (200 OK)" 200 "$STATUS"

# 403 Forbidden: Bob cannot delete Alice's note
RESP=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE_URL/notes/$NOTE_ID" \
  -H "Authorization: Bearer $BOB_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "10.2 Non-owner Bob tries to delete note (403 Forbidden)" 403 "$STATUS"

# 200 OK: Alice revokes Bob's share
RESP=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE_URL/notes/$NOTE_ID/share/$BOB_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "10.3 Alice revokes Bob's share (200 OK)" 200 "$STATUS"

# 403 Forbidden: Bob tries to read note after revocation
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/notes/$NOTE_ID" \
  -H "Authorization: Bearer $BOB_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "10.4 Bob tries to read note after revocation (403 Forbidden)" 403 "$STATUS"

# 200 OK: Alice deletes note
RESP=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE_URL/notes/$NOTE_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "10.5 Alice deletes note (200 OK)" 200 "$STATUS"

# 404 Not Found: Alice tries to read deleted note
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/notes/$NOTE_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "10.6 Read deleted note (404 Not Found)" 404 "$STATUS"

echo -e "\n${BLUE}--- Section 11: Redis Auth Cache, Refresh Token & Token Revocation ---${NC}"
# 401 Unauthorized: Refresh with fake token
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/refresh" \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"fake-invalid-refresh-token"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "11.1 Refresh with invalid token (401 Unauthorized)" 401 "$STATUS"

# 200 OK: Refresh with valid refresh token stored in Redis
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/refresh" \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"'"$ALICE_REFRESH_TOKEN"'"}')
BODY=$(echo "$RESP" | head -n -1)
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "11.2 Refresh token via Redis session (200 OK)" 200 "$STATUS"
NEW_ALICE_TOKEN=$(echo "$BODY" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
NEW_ALICE_REFRESH_TOKEN=$(echo "$BODY" | grep -o '"refresh_token":"[^"]*' | cut -d'"' -f4)

# 200 OK: Verify newly issued access token works
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/auth/me" \
  -H "Authorization: Bearer $NEW_ALICE_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "11.3 Access API with refreshed access token (200 OK)" 200 "$STATUS"

# 401 Unauthorized: Old refresh token is rotated and no longer valid
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/refresh" \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"'"$ALICE_REFRESH_TOKEN"'"}')
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "11.4 Reusing old rotated refresh token fails (401 Unauthorized)" 401 "$STATUS"

# 200 OK: Alice logs out
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/logout" \
  -H "Authorization: Bearer $NEW_ALICE_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "11.5 Alice logs out (200 OK)" 200 "$STATUS"

# 401 Unauthorized: Blacklisted token rejected
RESP=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/auth/me" \
  -H "Authorization: Bearer $NEW_ALICE_TOKEN")
STATUS=$(echo "$RESP" | tail -n 1)
assert_status "11.6 Call API with blacklisted token (401 Unauthorized)" 401 "$STATUS"

echo -e "\n${BLUE}========================================================================${NC}"
echo -e "${GREEN} Verification completed successfully: ${pass_count} passed, ${fail_count} failed.${NC}"
echo -e "${BLUE}========================================================================${NC}"
