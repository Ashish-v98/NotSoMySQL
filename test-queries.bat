@echo off
REM Test script for AI Query Sidecar (Windows)
REM Make sure services are running: docker-compose up -d

set BASE_URL=http://localhost:8080

echo ======================================
echo AI Query Sidecar - Test Script
echo ======================================
echo.

echo Test 1: Health Check
echo --------------------------------------
curl -s %BASE_URL%/health
echo.
echo.

echo Test 2: Show all customers
echo --------------------------------------
curl -s -X POST %BASE_URL%/query -H "Content-Type: application/json" -d "{\"query\": \"Show me all customers\"}"
echo.
echo.

echo Test 3: Calculate total revenue
echo --------------------------------------
curl -s -X POST %BASE_URL%/query -H "Content-Type: application/json" -d "{\"query\": \"What is the total revenue from all completed orders?\"}"
echo.
echo.

echo Test 4: Top 3 customers by order value
echo --------------------------------------
curl -s -X POST %BASE_URL%/query -H "Content-Type: application/json" -d "{\"query\": \"Show me the top 3 customers by total order value\"}"
echo.
echo.

echo Test 5: Products with low stock
echo --------------------------------------
curl -s -X POST %BASE_URL%/query -H "Content-Type: application/json" -d "{\"query\": \"Which products have less than 50 items in stock?\"}"
echo.
echo.

echo Test 6: Count orders by status
echo --------------------------------------
curl -s -X POST %BASE_URL%/query -H "Content-Type: application/json" -d "{\"query\": \"How many orders are there for each status?\"}"
echo.
echo.

echo ======================================
echo Tests completed!
echo ======================================
pause
