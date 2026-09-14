#!/bin/bash

orderID=$1
result=$(curl -s -X POST localhost:3000/api/orders/$orderID/status/cancel)

echo "$result"