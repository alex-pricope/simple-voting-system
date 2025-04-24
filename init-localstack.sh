#!/bin/bash
set -e

echo "Waiting for DynamoDB to be ready..."

until awslocal dynamodb list-tables &> /dev/null; do
  sleep 1
done

echo "DynamoDB is ready. Creating resources..."

# Create VotingCodes table (PK = code)
awslocal dynamodb create-table \
  --table-name VotingCodes \
  --attribute-definitions AttributeName=PK,AttributeType=S \
  --key-schema AttributeName=PK,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

# Create VotingCategories table (PK = ID as string)
awslocal dynamodb create-table \
  --table-name VotingCategories \
  --attribute-definitions AttributeName=PK,AttributeType=N \
  --key-schema AttributeName=PK,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

# Create Teams table (PK = ID as string)
awslocal dynamodb create-table \
  --table-name VotingTeams \
  --attribute-definitions AttributeName=PK,AttributeType=N \
  --key-schema AttributeName=PK,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

# Create Votes table (PK = code, SK = category#team)
awslocal dynamodb create-table \
  --table-name Votes \
  --attribute-definitions AttributeName=PK,AttributeType=S AttributeName=SK,AttributeType=S \
  --key-schema AttributeName=PK,KeyType=HASH AttributeName=SK,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST

# Optional: create 'health' bucket to silence dashboard error
awslocal s3 mb s3://health

echo "Resources created - all is ready"
