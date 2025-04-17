#!/bin/bash
set -e

echo "⏳ Waiting for DynamoDB to be ready..."

until awslocal dynamodb list-tables &> /dev/null; do
  sleep 1
done

echo "✅ LocalStack is ready. Creating resources..."

# Create DynamoDB table
awslocal dynamodb create-table \
  --table-name VotingCodes \
  --attribute-definitions AttributeName=PK,AttributeType=S \
  --key-schema AttributeName=PK,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

# Optional: create 'health' bucket to silence dashboard error
awslocal s3 mb s3://health

echo "✅ Resources created"