#!/bin/bash

# Test script for Traefik Quota Plugin with Multiple Identifiers
# This script tests rate limiting and quota functionality with custom response messages

echo "🚀 Testing Traefik Quota Plugin with Multiple Identifiers"
echo "========================================================"

BASE_URL="http://chat.localhost:8000"
USER_ID="test-user-123"

echo "📊 Testing Rate Limiting (5 requests per minute, burst 10)"
echo "Sending 15 rapid requests with User ID: $USER_ID"
echo ""

# Test rate limiting with rapid requests
for i in {1..15}; do
    echo -n "Request $i: "
    response=$(curl -s -w "HTTP %{http_code}" \
        -H "X-User-ID: $USER_ID" \
        "$BASE_URL/test" 2>/dev/null)
    
    echo "$response"
    
    # Check if we got a rate limit error (429) and show the response body
    if [[ "$response" == *"429"* ]]; then
        echo "  → Rate limit response body:"
        curl -s -H "X-User-ID: $USER_ID" "$BASE_URL/test" | jq . 2>/dev/null || echo "  (Not JSON format)"
        break
    fi
    
    # Small delay to see rate limiting in action
    sleep 0.1
done

echo ""
echo "⏱️  Waiting 65 seconds to test quota recovery..."
sleep 65

echo ""
echo "📈 Testing Different Users with Custom Headers"
echo "User 1 (first-user):"
response1=$(curl -s -w "\nHTTP %{http_code} | Identifier: %{header_x-quota-identifier} | Type: %{header_x-quota-identifier-type}" \
    -H "X-User-ID: first-user" \
    "$BASE_URL/test" 2>/dev/null)
echo "$response1"

echo ""
echo "User 2 (second-user):"
response2=$(curl -s -w "\nHTTP %{http_code} | Identifier: %{header_x-quota-identifier} | Type: %{header_x-quota-identifier-type}" \
    -H "X-User-ID: second-user" \
    "$BASE_URL/test" 2>/dev/null)
echo "$response2"

echo ""
echo "User 3 (no header - default user):"
response3=$(curl -s -w "\nHTTP %{http_code} | Identifier: %{header_x-quota-identifier} | Type: %{header_x-quota-identifier-type}" \
    "$BASE_URL/test" 2>/dev/null)
echo "$response3"

echo ""
echo "🔍 Detailed Response with All Headers:"
curl -v -H "X-User-ID: detailed-test" "$BASE_URL/test" 2>&1 | grep -E "(HTTP|X-Rate|X-Quota|Retry-After|Content-Type)"

echo ""
echo "📋 Redis Keys Check:"
echo "Rate limit keys:"
docker exec redis-quota redis-cli KEYS "ratelimit:*" 2>/dev/null | head -5

echo "Quota keys:"
docker exec redis-quota redis-cli KEYS "quota:*" 2>/dev/null | head -5

echo ""
echo "🎯 Testing Quota Usage for specific user:"
USER_TEST="quota-test-user"
echo "Making 5 requests for user: $USER_TEST"

for i in {1..5}; do
    response=$(curl -s -H "X-User-ID: $USER_TEST" "$BASE_URL/test" \
        -w "\nUsed: %{header_x-quota-used}, Remaining: %{header_x-quota-remaining}, Period: %{header_x-quota-period}")
    echo "Request $i - $response"
done

echo ""
echo "💥 Testing Quota Exhaustion (sending 100+ requests rapidly):"
USER_QUOTA_TEST="quota-exhaust-user"
echo "Sending 105 requests for user: $USER_QUOTA_TEST"

for i in {1..105}; do
    if [ $((i % 20)) -eq 0 ]; then
        echo -n "[$i] "
    fi
    
    response_code=$(curl -s -w "%{http_code}" -o /dev/null \
        -H "X-User-ID: $USER_QUOTA_TEST" \
        "$BASE_URL/test" 2>/dev/null)
    
    if [ "$response_code" -eq 429 ] || [ "$response_code" -eq 403 ]; then
        echo ""
        echo "  → Quota exceeded at request $i (HTTP $response_code)"
        echo "  → Custom response body:"
        curl -s -H "X-User-ID: $USER_QUOTA_TEST" "$BASE_URL/test" | jq . 2>/dev/null || echo "  (Not JSON format)"
        break
    fi
done

echo ""
echo "🌐 Testing Multiple Identifier Types:"

# Test with different header (if you add more identifiers)
echo "Testing IP-based identification:"
curl -s -w "HTTP %{http_code} | IP: %{header_x-quota-identifier}" \
    -H "X-Forwarded-For: 192.168.1.100" \
    "$BASE_URL/test" 2>/dev/null
echo ""

echo ""
echo "✅ Test completed!"
echo "Dashboard: http://traefik.localhost:8080"
echo "Monitor Redis: docker exec -it redis-quota redis-cli monitor"
echo "Check Redis keys: docker exec redis-quota redis-cli KEYS '*'"