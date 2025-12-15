#!/bin/bash

# Test script for AI Query Sidecar
# Make sure services are running: docker-compose up -d

BASE_URL="http://localhost:8080"

echo "======================================"
echo "AI Query Sidecar - Test Script"
echo "======================================"
echo ""

# Test 1: Health check
echo "Test 1: Health Check"
echo "------------------------------------"
curl -s ${BASE_URL}/health | jq .
echo -e "\n"

# Test 2: Show all customers
echo "Test 2: Show all customers"
echo "------------------------------------"
curl -s -X POST ${BASE_URL}/query \
  -H "Content-Type: application/json" \
  -d '{"query": "Show me all customers"}' | jq .
echo -e "\n"

# Test 3: Total revenue
echo "Test 3: Calculate total revenue"
echo "------------------------------------"
curl -s -X POST ${BASE_URL}/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What is the total revenue from all completed orders?"}' | jq .
echo -e "\n"

# Test 4: Top customers
echo "Test 4: Top 3 customers by order value"
echo "------------------------------------"
curl -s -X POST ${BASE_URL}/query \
  -H "Content-Type: application/json" \
  -d '{"query": "Show me the top 3 customers by total order value"}' | jq .
echo -e "\n"

# Test 5: Low stock products
echo "Test 5: Products with low stock"
echo "------------------------------------"
curl -s -X POST ${BASE_URL}/query \
  -H "Content-Type: application/json" \
  -d '{"query": "Which products have less than 50 items in stock?"}' | jq .
echo -e "\n"

# Test 6: Orders by status
echo "Test 6: Count orders by status"
echo "------------------------------------"
curl -s -X POST ${BASE_URL}/query \
  -H "Content-Type: application/json" \
  -d '{"query": "How many orders are there for each status?"}' | jq .
echo -e "\n"

echo "======================================"
echo "Tests completed!"
echo "======================================"
