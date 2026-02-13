#!/bin/bash
# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT

# Script to delete all survey and survey_response documents from OpenSearch
# This is a temporary utility script for cleaning up test/migration data

set -e

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
SURVEY_COUNT=$(curl -s -X GET "${OPENSEARCH_URL}/${INDEX_NAME}/_count" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey"
            }
        }
    }' | jq -r '.count')

echo "Found $SURVEY_COUNT survey documents"

echo ""
echo "Step 2: Counting survey_response documents..."
RESPONSE_COUNT=$(curl -s -X GET "${OPENSEARCH_URL}/${INDEX_NAME}/_count" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey_response"
            }
        }
    }' | jq -r '.count')

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
SURVEY_RESULT=$(curl -s -X POST "${OPENSEARCH_URL}/${INDEX_NAME}/_delete_by_query?conflicts=proceed" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey"
            }
        }
    }')

SURVEY_DELETED=$(echo "$SURVEY_RESULT" | jq -r '.deleted')
echo "Deleted $SURVEY_DELETED survey documents from OpenSearch"

# Delete OpenSearch documents for survey responses
echo ""
echo "Step 4: Deleting OpenSearch survey_response documents..."
RESPONSE_RESULT=$(curl -s -X POST "${OPENSEARCH_URL}/${INDEX_NAME}/_delete_by_query?conflicts=proceed" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey_response"
            }
        }
    }')

RESPONSE_DELETED=$(echo "$RESPONSE_RESULT" | jq -r '.deleted')
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

REMAINING_SURVEY=$(curl -s -X GET "${OPENSEARCH_URL}/${INDEX_NAME}/_count" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey"
            }
        }
    }' | jq -r '.count')

REMAINING_RESPONSE=$(curl -s -X GET "${OPENSEARCH_URL}/${INDEX_NAME}/_count" \
    -H 'Content-Type: application/json' \
    -d '{
        "query": {
            "term": {
                "object_type": "survey_response"
            }
        }
    }' | jq -r '.count')

TOTAL_REMAINING=$((REMAINING_SURVEY + REMAINING_RESPONSE))

echo "Remaining survey documents: $REMAINING_SURVEY"
echo "Remaining survey_response documents: $REMAINING_RESPONSE"
echo "Total remaining: $TOTAL_REMAINING"

echo ""
if [ "$TOTAL_REMAINING" -eq 0 ]; then
    echo "✓ All OpenSearch documents successfully removed!"
else
    echo "⚠ Warning: $TOTAL_REMAINING documents still remain."
fi
