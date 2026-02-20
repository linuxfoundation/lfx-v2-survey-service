#!/bin/bash
# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT

# Script to delete all survey and survey_response documents from OpenSearch
# This is a temporary utility script for cleaning up test/migration data

set -e

# Check for required dependencies
if ! command -v jq &> /dev/null; then
    echo "Error: jq is not installed. Please install jq to run this script."
    echo "  - macOS: brew install jq"
    echo "  - Ubuntu/Debian: apt-get install jq"
    echo "  - RHEL/CentOS: yum install jq"
    exit 1
fi

if ! command -v curl &> /dev/null; then
    echo "Error: curl is not installed. Please install curl to run this script."
    exit 1
fi

# Configuration
OPENSEARCH_URL="${OPENSEARCH_URL:-http://opensearch-cluster-master.lfx.svc.cluster.local:9200}"
INDEX_NAME="${INDEX_NAME:-resources}"

echo "================================================"
echo "Survey & Survey Response Cleanup Script"
echo "================================================"
echo "OpenSearch URL: $OPENSEARCH_URL"
echo "OpenSearch Index: $INDEX_NAME"
echo ""
echo "This will delete ALL OpenSearch documents with type:"
echo "  - survey"
echo "  - survey_response"
echo ""
read -p "Are you sure you want to proceed? (yes/no): " CONFIRM

if [ "$CONFIRM" != "yes" ]; then
    echo "Aborted."
    exit 0
fi

echo ""
echo "Step 1: Counting survey documents..."
SURVEY_RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "${OPENSEARCH_URL}/${INDEX_NAME}/_count" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey"
            }
        }
    }')

HTTP_CODE=$(echo "$SURVEY_RESPONSE" | tail -n1)
SURVEY_BODY=$(echo "$SURVEY_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" != "200" ]; then
    echo "Error: Failed to query OpenSearch (HTTP $HTTP_CODE)"
    echo "Response: $SURVEY_BODY"
    exit 1
fi

SURVEY_COUNT=$(echo "$SURVEY_BODY" | jq -r '.count')
if [ "$SURVEY_COUNT" = "null" ] || [ -z "$SURVEY_COUNT" ]; then
    echo "Error: Invalid response from OpenSearch"
    echo "Response: $SURVEY_BODY"
    exit 1
fi

echo "Found $SURVEY_COUNT survey documents"

echo ""
echo "Step 2: Counting survey_response documents..."
RESPONSE_RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "${OPENSEARCH_URL}/${INDEX_NAME}/_count" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey_response"
            }
        }
    }')

HTTP_CODE=$(echo "$RESPONSE_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$RESPONSE_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" != "200" ]; then
    echo "Error: Failed to query OpenSearch (HTTP $HTTP_CODE)"
    echo "Response: $RESPONSE_BODY"
    exit 1
fi

RESPONSE_COUNT=$(echo "$RESPONSE_BODY" | jq -r '.count')
if [ "$RESPONSE_COUNT" = "null" ] || [ -z "$RESPONSE_COUNT" ]; then
    echo "Error: Invalid response from OpenSearch"
    echo "Response: $RESPONSE_BODY"
    exit 1
fi

echo "Found $RESPONSE_COUNT survey_response documents"

TOTAL_COUNT=$((SURVEY_COUNT + RESPONSE_COUNT))
echo ""
echo "Total documents to delete: $TOTAL_COUNT"

if [ "$TOTAL_COUNT" -eq 0 ]; then
    echo "No documents to delete. Exiting."
    exit 0
fi

echo ""
read -p "Proceed with deletion? (yes/no): " CONFIRM_DELETE

if [ "$CONFIRM_DELETE" != "yes" ]; then
    echo "Aborted."
    exit 0
fi

# Delete OpenSearch documents for surveys
echo ""
echo "Step 3: Deleting OpenSearch survey documents..."
SURVEY_DELETE_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${OPENSEARCH_URL}/${INDEX_NAME}/_delete_by_query?conflicts=proceed" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey"
            }
        }
    }')

HTTP_CODE=$(echo "$SURVEY_DELETE_RESPONSE" | tail -n1)
SURVEY_RESULT=$(echo "$SURVEY_DELETE_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" != "200" ]; then
    echo "Error: Failed to delete survey documents (HTTP $HTTP_CODE)"
    echo "Response: $SURVEY_RESULT"
    exit 1
fi

SURVEY_DELETED=$(echo "$SURVEY_RESULT" | jq -r '.deleted')
if [ "$SURVEY_DELETED" = "null" ]; then
    SURVEY_DELETED=0
fi

echo "Deleted $SURVEY_DELETED survey documents from OpenSearch"

# Delete OpenSearch documents for survey responses
echo ""
echo "Step 4: Deleting OpenSearch survey_response documents..."
RESPONSE_DELETE_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${OPENSEARCH_URL}/${INDEX_NAME}/_delete_by_query?conflicts=proceed" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey_response"
            }
        }
    }')

HTTP_CODE=$(echo "$RESPONSE_DELETE_RESPONSE" | tail -n1)
RESPONSE_RESULT=$(echo "$RESPONSE_DELETE_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" != "200" ]; then
    echo "Error: Failed to delete survey_response documents (HTTP $HTTP_CODE)"
    echo "Response: $RESPONSE_RESULT"
    exit 1
fi

RESPONSE_DELETED=$(echo "$RESPONSE_RESULT" | jq -r '.deleted')
if [ "$RESPONSE_DELETED" = "null" ]; then
    RESPONSE_DELETED=0
fi

echo "Deleted $RESPONSE_DELETED survey_response documents from OpenSearch"

TOTAL_DELETED=$((SURVEY_DELETED + RESPONSE_DELETED))

echo ""
echo "================================================"
echo "Cleanup Complete"
echo "================================================"
echo "OpenSearch documents deleted: $TOTAL_DELETED"
echo ""
echo "Waiting 5 seconds for OpenSearch to process deletions..."
sleep 5

# Verify OpenSearch cleanup
echo ""
echo "Step 5: Verifying OpenSearch cleanup..."

VERIFY_SURVEY_RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "${OPENSEARCH_URL}/${INDEX_NAME}/_count" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey"
            }
        }
    }')

HTTP_CODE=$(echo "$VERIFY_SURVEY_RESPONSE" | tail -n1)
VERIFY_SURVEY_BODY=$(echo "$VERIFY_SURVEY_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    REMAINING_SURVEY=$(echo "$VERIFY_SURVEY_BODY" | jq -r '.count')
    if [ "$REMAINING_SURVEY" = "null" ]; then
        REMAINING_SURVEY=0
    fi
else
    echo "Warning: Failed to verify survey document count (HTTP $HTTP_CODE)"
    REMAINING_SURVEY="unknown"
fi

VERIFY_RESPONSE_RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "${OPENSEARCH_URL}/${INDEX_NAME}/_count" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey_response"
            }
        }
    }')

HTTP_CODE=$(echo "$VERIFY_RESPONSE_RESPONSE" | tail -n1)
VERIFY_RESPONSE_BODY=$(echo "$VERIFY_RESPONSE_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    REMAINING_RESPONSE=$(echo "$VERIFY_RESPONSE_BODY" | jq -r '.count')
    if [ "$REMAINING_RESPONSE" = "null" ]; then
        REMAINING_RESPONSE=0
    fi
else
    echo "Warning: Failed to verify survey_response document count (HTTP $HTTP_CODE)"
    REMAINING_RESPONSE="unknown"
fi

echo "Remaining survey documents: $REMAINING_SURVEY"
echo "Remaining survey_response documents: $REMAINING_RESPONSE"

if [ "$REMAINING_SURVEY" = "unknown" ] || [ "$REMAINING_RESPONSE" = "unknown" ]; then
    echo "Total remaining: unknown (verification failed)"
    echo ""
    echo "⚠ Warning: Could not verify cleanup completion"
else
    TOTAL_REMAINING=$((REMAINING_SURVEY + REMAINING_RESPONSE))
    echo "Total remaining: $TOTAL_REMAINING"

    echo ""
    if [ "$TOTAL_REMAINING" -eq 0 ]; then
        echo "✓ All OpenSearch documents successfully removed!"
    else
        echo "⚠ Warning: $TOTAL_REMAINING documents still remain."
    fi
fi
