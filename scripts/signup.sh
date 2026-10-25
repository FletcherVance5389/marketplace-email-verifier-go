#!/bin/sh
set -eu

curl --request POST "${MARKETPLACE_URL:-http://localhost:8080}/signup" \
  --header 'Content-Type: application/json' \
  --data '{
    "seller_email": "seller@example.com",
    "seller_name": "Mina",
    "asset_ids": ["asset-photo-set"],
    "buyer_email": "buyer@example.com",
    "order_id": "order-42"
  }'
