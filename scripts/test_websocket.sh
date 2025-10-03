#!/bin/bash

# WebSocket 聊天室測試腳本
# 需要安裝 wscat: npm install -g wscat

WS_HOST="ws://localhost:8080/ws"

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

# 檢查 wscat 是否安裝
check_wscat() {
    if ! command -v wscat &> /dev/null; then
        print_error "wscat 未安裝"
        print_info "請執行: npm install -g wscat"
        exit 1
    fi
    print_success "wscat 已安裝"
}

# 檢查 WebSocket 服務
check_websocket_service() {
    print_header "檢查 WebSocket 服務"
    
    # 嘗試連接 WebSocket
    timeout 5 wscat -c "${WS_HOST}?room_id=test&user_id=test" > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        print_success "WebSocket 服務正在運行"
        return 0
    else
        print_error "WebSocket 服務未運行或無法連接"
        print_info "請確保服務正在運行: go run cmd/api/main.go"
        return 1
    fi
}

# 測試 WebSocket 連接
test_websocket_connection() {
    local room_id=$1
    local user_id=$2
    
    if [ -z "$room_id" ] || [ -z "$user_id" ]; then
        print_error "請提供房間 ID 和用戶 ID"
        print_info "用法: $0 connect <room_id> <user_id>"
        return 1
    fi
    
    print_header "測試 WebSocket 連接"
    print_info "房間 ID: $room_id"
    print_info "用戶 ID: $user_id"
    print_info "WebSocket URL: ${WS_HOST}?room_id=${room_id}&user_id=${user_id}"
    print_info ""
    print_info "連接中... (按 Ctrl+C 停止)"
    print_info "你可以輸入消息來測試聊天功能"
    print_info ""
    
    wscat -c "${WS_HOST}?room_id=${room_id}&user_id=${user_id}"
}

# 創建測試聊天室
create_test_room() {
    local room_type=$1
    local user_id=${2:-"user_alice"}
    
    print_header "創建測試聊天室"
    
    if [ "$room_type" = "direct" ]; then
        print_info "創建一對一聊天室..."
        curl -s -X POST "http://localhost:8080/api/v1/rooms" \
            -H "Content-Type: application/json" \
            -d "{
                \"name\": \"WebSocket 測試 - 一對一\",
                \"type\": \"direct\",
                \"owner_id\": \"$user_id\",
                \"members\": [
                    {\"user_id\": \"$user_id\", \"role\": \"admin\"},
                    {\"user_id\": \"user_bob\", \"role\": \"member\"}
                ]
            }" | jq -r '.data.id // empty'
    elif [ "$room_type" = "group" ]; then
        print_info "創建群組聊天室..."
        curl -s -X POST "http://localhost:8080/api/v1/rooms" \
            -H "Content-Type: application/json" \
            -d "{
                \"name\": \"WebSocket 測試 - 群組\",
                \"type\": \"group\",
                \"owner_id\": \"$user_id\",
                \"members\": [
                    {\"user_id\": \"$user_id\", \"role\": \"admin\"},
                    {\"user_id\": \"user_bob\", \"role\": \"member\"},
                    {\"user_id\": \"user_charlie\", \"role\": \"member\"},
                    {\"user_id\": \"user_david\", \"role\": \"member\"}
                ]
            }" | jq -r '.data.id // empty'
    else
        print_error "無效的房間類型: $room_type"
        print_info "支援的類型: direct, group"
        return 1
    fi
}

# 列出用戶聊天室
list_user_rooms() {
    local user_id=${1:-"user_alice"}
    
    print_header "列出用戶聊天室"
    print_info "用戶 ID: $user_id"
    
    curl -s "http://localhost:8080/api/v1/rooms?user_id=${user_id}&limit=10" | jq '.'
}

# 發送測試消息
send_test_message() {
    local room_id=$1
    local user_id=$2
    local content=${3:-"WebSocket 測試消息"}
    
    if [ -z "$room_id" ] || [ -z "$user_id" ]; then
        print_error "請提供房間 ID 和用戶 ID"
        print_info "用法: $0 send <room_id> <user_id> [content]"
        return 1
    fi
    
    print_header "發送測試消息"
    print_info "房間 ID: $room_id"
    print_info "用戶 ID: $user_id"
    print_info "內容: $content"
    
    curl -s -X POST "http://localhost:8080/api/v1/messages" \
        -H "Content-Type: application/json" \
        -d "{
            \"room_id\": \"$room_id\",
            \"sender_id\": \"$user_id\",
            \"content\": \"$content\",
            \"type\": \"text\"
        }" | jq '.'
}

# 獲取聊天室消息
get_room_messages() {
    local room_id=$1
    local user_id=${2:-"user_alice"}
    local limit=${3:-10}
    
    if [ -z "$room_id" ]; then
        print_error "請提供房間 ID"
        print_info "用法: $0 messages <room_id> [user_id] [limit]"
        return 1
    fi
    
    print_header "獲取聊天室消息"
    print_info "房間 ID: $room_id"
    print_info "用戶 ID: $user_id"
    print_info "限制: $limit"
    
    curl -s "http://localhost:8080/api/v1/messages?room_id=${room_id}&user_id=${user_id}&limit=${limit}" | jq '.'
}

# 顯示幫助
show_help() {
    echo "WebSocket 聊天室測試工具"
    echo ""
    echo "用法: $0 <command> [options]"
    echo ""
    echo "命令:"
    echo "  connect <room_id> <user_id>    連接到聊天室 WebSocket"
    echo "  create <type> [user_id]        創建測試聊天室 (direct/group)"
    echo "  list [user_id]                 列出用戶聊天室"
    echo "  send <room_id> <user_id> [msg] 發送測試消息"
    echo "  messages <room_id> [user_id]   獲取聊天室消息"
    echo "  test                           運行完整測試"
    echo "  help                           顯示此幫助"
    echo ""
    echo "範例:"
    echo "  $0 create direct user_alice"
    echo "  $0 connect room_123 user_alice"
    echo "  $0 send room_123 user_alice 'Hello World'"
    echo "  $0 test"
}

# 運行完整測試
run_full_test() {
    print_header "WebSocket 聊天室完整測試"
    
    # 檢查依賴
    check_wscat
    if ! check_websocket_service; then
        exit 1
    fi
    
    # 創建一對一聊天室
    print_info "創建一對一聊天室..."
    direct_room=$(create_test_room "direct")
    if [ -n "$direct_room" ]; then
        print_success "一對一聊天室創建成功: $direct_room"
    else
        print_error "一對一聊天室創建失敗"
        return 1
    fi
    
    # 創建群組聊天室
    print_info "創建群組聊天室..."
    group_room=$(create_test_room "group")
    if [ -n "$group_room" ]; then
        print_success "群組聊天室創建成功: $group_room"
    else
        print_error "群組聊天室創建失敗"
        return 1
    fi
    
    # 發送測試消息
    print_info "發送測試消息到一對一聊天室..."
    send_test_message "$direct_room" "user_alice" "一對一聊天測試消息"
    
    print_info "發送測試消息到群組聊天室..."
    send_test_message "$group_room" "user_alice" "群組聊天測試消息"
    
    # 列出聊天室
    print_info "列出用戶聊天室..."
    list_user_rooms "user_alice"
    
    print_success "測試完成！"
    print_info "你可以使用以下命令進行 WebSocket 測試："
    print_info "  $0 connect $direct_room user_alice"
    print_info "  $0 connect $group_room user_bob"
}

# 主函數
main() {
    case "${1:-help}" in
        "connect")
            check_wscat && check_websocket_service && test_websocket_connection "$2" "$3"
            ;;
        "create")
            create_test_room "$2" "$3"
            ;;
        "list")
            list_user_rooms "$2"
            ;;
        "send")
            send_test_message "$2" "$3" "$4"
            ;;
        "messages")
            get_room_messages "$2" "$3" "$4"
            ;;
        "test")
            run_full_test
            ;;
        "help"|*)
            show_help
            ;;
    esac
}

# 執行主函數
main "$@"
