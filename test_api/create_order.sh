#!/bin/bash

result=$(curl -s -X POST \
    -H "Content-Type: application/json" \
    --data @test_api/createOrder.json \
    localhost:3000/api/orders)

echo "$result"
