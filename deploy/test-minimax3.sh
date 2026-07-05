#!/bin/bash
# Test 3: with reasoning_effort (Cline's native parameter)
curl -s -X POST https://api.cline.bot/api/v1/chat/completions \
  -H "Authorization: Bearer sk_b1a9318b433fbbe02eef29711721201252b0d37d8075c20155bf69e2b74e5275" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "cline-pass/minimax-m3",
    "messages": [{"role": "user", "content": "1+1=? 先思考再回答"}],
    "stream": true,
    "reasoning_effort": "high"
  }' 2>&1 | head -c 5000
