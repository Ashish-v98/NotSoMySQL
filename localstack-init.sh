#!/bin/bash

# Wait for LocalStack to be ready
echo "Waiting for LocalStack to be ready..."
until awslocal s3 ls 2>/dev/null; do
  sleep 1
done

echo "Creating S3 bucket: ai-query-results"
awslocal s3 mb s3://ai-query-results

echo "LocalStack initialization complete!"
