#!/bin/bash

# gRPC 聊天室測試腳本
# 需要安裝 grpcurl: brew install grpcurl

GRPC_HOST="localhost:8081"

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

# 檢查 gRPC 服務是否運行
check_grpc_service() {
    print_header "檢查 gRPC 服務狀態"
    
    if grpcurl -plaintext ${GRPC_HOST} list > /dev/null 2>&1; then
        print_success "gRPC 服務正在運行"
        return 0
    else
        print_error "gRPC 服務未運行，請先啟動服務"
        print_info "使用命令: go run cmd/api/main.go"
        return 1
    fi
}

# 測試 1對1 聊天
test_one_on_one_chat() {
    print_header "測試 1對1 聊天"
    
    # 創建 1對1 聊天室
    print_info "創建 1對1 聊天室..."
    room_response=$(grpcurl -plaintext -d '{
      "name": "Alice 和 Bob 的私聊",
      "description": "私人對話",
      "type": "direct",
      "owner_id": "user_alice",
      "member_ids": ["user_alice", "user_bob"],
      "settings": {
        "allow_invite": false,
        "allow_edit_messages": true,
        "allow_delete_messages": true,
        "max_members": 2
      }
    }' ${GRPC_HOST} chat.ChatRoomService/CreateRoom)
    
    echo "$room_response"
    
    # 提取房間 ID
    room_id=$(echo "$room_response" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    if [ -n "$room_id" ]; then
        print_success "1對1 聊天室創建成功，ID: $room_id"
        
        # 發送消息
        print_info "發送消息..."
        message_response=$(grpcurl -plaintext -d "{
          \"room_id\": \"$room_id\",
          \"sender_id\": \"user_alice\",
          \"content\": \"你好 Bob，這是我們的私聊！\",
          \"type\": \"text\"
        }" ${GRPC_HOST} chat.ChatRoomService/SendMessage)
        
        echo "$message_response"
        
        # 獲取消息
        print_info "獲取消息..."
        messages_response=$(grpcurl -plaintext -d "{
          \"room_id\": \"$room_id\",
          \"user_id\": \"user_alice\",
          \"limit\": 10
        }" ${GRPC_HOST} chat.ChatRoomService/GetMessages)
        
        echo "$messages_response"
        
        echo "$room_id"
    else
        print_error "1對1 聊天室創建失敗"
        return 1
    fi
}

# 測試群組聊天
test_group_chat() {
    print_header "測試群組聊天"
    
    # 創建群組聊天室
    print_info "創建群組聊天室..."
    room_response=$(grpcurl -plaintext -d '{
      "name": "開發團隊討論組",
      "description": "技術討論",
      "type": "group",
      "owner_id": "user_alice",
      "member_ids": ["user_alice", "user_bob", "user_charlie", "user_david"],
      "settings": {
        "allow_invite": true,
        "allow_edit_messages": true,
        "allow_delete_messages": true,
        "max_members": 100,
        "welcome_message": "歡迎加入開發團隊討論組！"
      }
    }' ${GRPC_HOST} chat.ChatRoomService/CreateRoom)
    
    echo "$room_response"
    
    # 提取房間 ID
    room_id=$(echo "$room_response" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    if [ -n "$room_id" ]; then
        print_success "群組聊天室創建成功，ID: $room_id"
        
        # 發送多條消息
        print_info "發送多條消息..."
        for i in {1..5}; do
            sender="user_alice"
            case $((i % 4)) in
                0) sender="user_alice" ;;
                1) sender="user_bob" ;;
                2) sender="user_charlie" ;;
                3) sender="user_david" ;;
            esac
            
            message_response=$(grpcurl -plaintext -d "{
              \"room_id\": \"$room_id\",
              \"sender_id\": \"$sender\",
              \"content\": \"這是 $sender 發送的消息 #$i\",
              \"type\": \"text\"
            }" ${GRPC_HOST} chat.ChatRoomService/SendMessage)
            
            echo "消息 #$i 發送結果: $message_response"
            sleep 0.1
        done
        
        # 獲取消息
        print_info "獲取消息..."
        messages_response=$(grpcurl -plaintext -d "{
          \"room_id\": \"$room_id\",
          \"user_id\": \"user_alice\",
          \"limit\": 10
        }" ${GRPC_HOST} chat.ChatRoomService/GetMessages)
        
        echo "$messages_response"
        
        echo "$room_id"
    else
        print_error "群組聊天室創建失敗"
        return 1
    fi
}

# 測試流式消息
test_stream_messages() {
    print_header "測試流式消息"
    
    room_id=$1
    if [ -z "$room_id" ]; then
        print_error "需要聊天室 ID"
        return 1
    fi
    
    print_info "開始流式接收消息（按 Ctrl+C 停止）..."
    print_info "在另一個終端發送消息來測試流式功能"
    
    grpcurl -plaintext -d "{
      \"room_id\": \"$room_id\",
      \"user_id\": \"user_alice\"
    }" ${GRPC_HOST} chat.ChatRoomService/StreamMessages
}

# 列出用戶聊天室
list_user_rooms() {
    print_header "列出用戶聊天室"
    
    print_info "列出 Alice 的所有聊天室"
    grpcurl -plaintext -d '{
      "user_id": "user_alice",
      "limit": 10,
      "offset": 0
    }' ${GRPC_HOST} chat.ChatRoomService/ListUserRooms
}

# 主測試流程
main() {
    print_header "gRPC 聊天室功能測試"
    
    # 檢查服務
    if ! check_grpc_service; then
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
    else
        print_error "群組聊天測試失敗"
    fi
    
    echo ""
    
    # 列出用戶聊天室
    list_user_rooms
    
    echo ""
    
    # 提供流式測試選項
    print_info "如需測試流式消息，請執行："
    print_info "./scripts/test_grpc.sh stream $group_room"
    
    print_header "測試完成"
}

# 處理命令行參數
case "${1:-}" in
    "stream")
        if [ -z "${2:-}" ]; then
            print_error "請提供聊天室 ID"
            exit 1
        fi
        check_grpc_service && test_stream_messages "$2"
        ;;
    *)
        main
        ;;
esac

