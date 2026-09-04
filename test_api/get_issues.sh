#!/bin/bash

orderID=$1
url="localhost:3000/api/orders/$orderID/issues"

curl -s $url | jq
