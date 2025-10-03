#!/bin/bash

# 測試聊天訊息滾動加載功能
# 用法: ./scripts/test_scroll_messages.sh <room_id> <user_id> <message_count>

ROOM_ID=${1:-"test_room"}
USER_ID=${2:-"user_1"}
MESSAGE_COUNT=${3:-30}
API_BASE=${4:-"http://localhost:8080/api"}

echo "準備發送 $MESSAGE_COUNT 條測試訊息到聊天室 $ROOM_ID"
echo "使用用戶: $USER_ID"
echo "API 地址: $API_BASE"
echo "========================================"

for i in $(seq 1 $MESSAGE_COUNT); do
    MESSAGE="測試訊息 #$i - 這是用來測試滾動加載功能的訊息"
    
    RESPONSE=$(curl -s -X POST "$API_BASE/messages" \
        -H "Content-Type: application/json" \
        -d "{
            \"room_id\": \"$ROOM_ID\",
            \"user_id\": \"$USER_ID\",
            \"content\": \"$MESSAGE\",
            \"content_type\": \"text\"
        }")
    
    if echo "$RESPONSE" | grep -q '"success":true'; then
        echo "✓ 已發送訊息 $i/$MESSAGE_COUNT"
    else
        echo "✗ 發送訊息 $i 失敗: $RESPONSE"
    fi
    
    # 稍微延遲，避免請求過快
    sleep 0.1
done

echo "========================================"
echo "✅ 完成！已發送 $MESSAGE_COUNT 條訊息"
echo ""
echo "測試步驟："
echo "1. 在網頁中選擇聊天室: $ROOM_ID"
echo "2. 等待訊息加載完成（只會顯示最新的 20 條）"
echo "3. 將滾輪往上滾動到頂部"
echo "4. 系統會自動加載更早的訊息"

