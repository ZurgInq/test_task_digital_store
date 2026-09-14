#!/bin/bash

url="localhost:3000/api/users/1/balance"

curl -s $url | jq
