#!/bin/bash

sku=$1

order=('{
    "userId": 1,
    "sku": ['$sku']
}')

result=$(curl -s -X POST \
    -H "Content-Type: application/json" \
    --data "$order" \
    localhost:3000/api/orders)

echo "$result"
