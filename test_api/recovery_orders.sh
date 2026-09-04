#!/bin/bash

result=$(curl -s -X POST \
    -H "Content-Type: application/json" \
    localhost:3000/api/recovery/orders)

echo "$result"
