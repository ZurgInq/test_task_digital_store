#!/bin/bash

url="localhost:3000/api/users/1/transactions"

curl -s $url | jq
