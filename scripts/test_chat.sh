#!/bin/bash

# 聊天服務測試腳本
# 測試 1對1 和群組聊天功能

BASE_URL="http://localhost:8080"
API_BASE="$BASE_URL/api/v1"

# 顏色定義
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印函數
print_header() {
    echo -e "${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ $1${NC}"
}

# 檢查服務是否運行
check_service() {
    print_header "檢查服務狀態"
    
    response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health")
    if [ "$response" = "200" ]; then
        print_success "服務正在運行"
        return 0
    else
        print_error "服務未運行，請先啟動服務"
        print_info "使用命令: air 或 go run cmd/api/main.go"
        return 1
    fi
}

# 測試 1對1 聊天
test_one_on_one_chat() {
    print_header "測試 1對1 聊天"
    
    # 創建 1對1 聊天室
    print_info "創建 1對1 聊天室..."
    room_response=$(curl -s -X POST "$API_BASE/rooms" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "1對1聊天測試",
            "type": "direct",
            "owner_id": "user1",
            "members": [
                {"user_id": "user1", "role": "admin"},
                {"user_id": "user2", "role": "member"}
            ]
        }')
    
    room_id=$(echo $room_response | jq -r '.data.id // empty')
    if [ -n "$room_id" ] && [ "$room_id" != "null" ]; then
        print_success "1對1 聊天室創建成功，ID: $room_id"
    else
        print_error "1對1 聊天室創建失敗"
        echo "響應: $room_response"
        return 1
    fi
    
    # 發送消息
    print_info "發送消息..."
    message_response=$(curl -s -X POST "$API_BASE/messages" \
        -H "Content-Type: application/json" \
        -d "{
            \"room_id\": \"$room_id\",
            \"sender_id\": \"user1\",
            \"content\": \"你好，這是 1對1 聊天測試消息\",
            \"type\": \"text\"
        }")
    
    message_id=$(echo $message_response | jq -r '.data.id // empty')
    if [ -n "$message_id" ] && [ "$message_id" != "null" ]; then
        print_success "消息發送成功，ID: $message_id"
    else
        print_error "消息發送失敗"
        echo "響應: $message_response"
    fi
    
    # 獲取消息列表
    print_info "獲取消息列表..."
    messages_response=$(curl -s "$API_BASE/messages?room_id=$room_id&user_id=user1&limit=10")
    message_count=$(echo $messages_response | jq -r '.data.data | length')
    if [ "$message_count" -gt 0 ]; then
        print_success "成功獲取 $message_count 條消息"
    else
        print_error "獲取消息失敗"
        echo "響應: $messages_response"
    fi
    
    echo "$room_id"
}

# 測試群組聊天
test_group_chat() {
    print_header "測試群組聊天"
    
    # 創建群組聊天室
    print_info "創建群組聊天室..."
    room_response=$(curl -s -X POST "$API_BASE/rooms" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "群組聊天測試",
            "type": "group",
            "owner_id": "user1",
            "members": [
                {"user_id": "user1", "role": "admin"},
                {"user_id": "user2", "role": "member"},
                {"user_id": "user3", "role": "member"},
                {"user_id": "user4", "role": "member"}
            ]
        }')
    
    room_id=$(echo $room_response | jq -r '.data.id // empty')
    if [ -n "$room_id" ] && [ "$room_id" != "null" ]; then
        print_success "群組聊天室創建成功，ID: $room_id"
    else
        print_error "群組聊天室創建失敗"
        echo "響應: $room_response"
        return 1
    fi
    
    # 發送多條消息
    print_info "發送多條消息..."
    for i in {1..5}; do
        message_response=$(curl -s -X POST "$API_BASE/messages" \
            -H "Content-Type: application/json" \
            -d "{
                \"room_id\": \"$room_id\",
                \"sender_id\": \"user$((i % 4 + 1))\",
                \"content\": \"這是群組消息 #$i\",
                \"type\": \"text\"
            }")
        
        message_id=$(echo $message_response | jq -r '.data.id // empty')
        if [ -n "$message_id" ] && [ "$message_id" != "null" ]; then
            print_success "消息 #$i 發送成功"
        else
            print_error "消息 #$i 發送失敗"
        fi
        
        sleep 0.1  # 避免太快發送
    done
    
    # 獲取消息列表
    print_info "獲取消息列表..."
    messages_response=$(curl -s "$API_BASE/messages?room_id=$room_id&user_id=user1&limit=10")
    message_count=$(echo $messages_response | jq -r '.data.data | length')
    if [ "$message_count" -gt 0 ]; then
        print_success "成功獲取 $message_count 條消息"
    else
        print_error "獲取消息失敗"
        echo "響應: $messages_response"
    fi
    
    echo "$room_id"
}

# 測試歷史消息分頁
test_history_pagination() {
    print_header "測試歷史消息分頁"
    
    room_id=$1
    if [ -z "$room_id" ]; then
        print_error "需要聊天室 ID"
        return 1
    fi
    
    # 測試分頁查詢
    print_info "測試分頁查詢..."
    page1_response=$(curl -s "$API_BASE/messages/history?room_id=$room_id&user_id=user1&limit=3")
    page1_count=$(echo $page1_response | jq -r '.data.data | length')
    next_cursor=$(echo $page1_response | jq -r '.data.pagination.next_cursor // empty')
    
    if [ "$page1_count" -gt 0 ]; then
        print_success "第一頁獲取 $page1_count 條消息"
        if [ -n "$next_cursor" ] && [ "$next_cursor" != "null" ]; then
            print_info "有下一頁，游標: $next_cursor"
            
            # 獲取第二頁
            page2_response=$(curl -s "$API_BASE/messages/history?room_id=$room_id&user_id=user1&limit=3&cursor=$next_cursor")
            page2_count=$(echo $page2_response | jq -r '.data.data | length')
            if [ "$page2_count" -gt 0 ]; then
                print_success "第二頁獲取 $page2_count 條消息"
            else
                print_error "第二頁獲取失敗"
            fi
        else
            print_info "沒有更多消息"
        fi
    else
        print_error "分頁查詢失敗"
        echo "響應: $page1_response"
    fi
}

# 測試 WebSocket 連接
test_websocket() {
    print_header "測試 WebSocket 連接"
    
    print_info "WebSocket 端點: ws://localhost:8080/ws?room_id=ROOM_ID&user_id=USER_ID"
    print_info "可以使用 wscat 工具測試:"
    print_info "npm install -g wscat"
    print_info "wscat -c 'ws://localhost:8080/ws?room_id=ROOM_ID&user_id=USER_ID'"
}

# 主測試流程
main() {
    print_header "聊天服務功能測試"
    
    # 檢查服務
    if ! check_service; then
        exit 1
    fi
    
    # 測試 1對1 聊天
    one_on_one_room=$(test_one_on_one_chat)
    if [ -n "$one_on_one_room" ]; then
        print_success "1對1 聊天測試完成"
    else
        print_error "1對1 聊天測試失敗"
    fi
    
    echo ""
    
    # 測試群組聊天
    group_room=$(test_group_chat)
    if [ -n "$group_room" ]; then
        print_success "群組聊天測試完成"
        
        # 測試歷史消息分頁
        test_history_pagination "$group_room"
    else
        print_error "群組聊天測試失敗"
    fi
    
    echo ""
    
    # WebSocket 測試說明
    test_websocket
    
    print_header "測試完成"
    print_info "如需測試 WebSocket，請使用 wscat 工具"
    print_info "服務日誌請查看終端輸出"
}

# 執行主函數
main "$@"
