#!/bin/bash

orderID=$1
status=$2

result=$(curl -s -X POST localhost:3001/api/payments/$orderID/webhook)

echo "$result"
