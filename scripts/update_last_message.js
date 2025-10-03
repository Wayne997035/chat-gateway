// 更新所有聊天室的 last_message 和 last_message_time
// 使用方式: mongosh mongodb://localhost:27017/chatroom scripts/update_last_message.js

const db = db.getSiblingDB('chatroom');

print('開始更新聊天室的最後訊息...\n');

let updatedCount = 0;
let noMessageCount = 0;

db.chatroom.find().forEach(function(room) {
    // 查找該聊天室的最後一條訊息
    const lastMsg = db.message.find({room_id: room.id})
        .sort({created_at: -1})
        .limit(1)
        .toArray()[0];
    
    if (lastMsg) {
        // 生成預覽（最多50字）
        let preview = lastMsg.content;
        if (preview.length > 50) {
            preview = preview.substring(0, 50) + '...';
        }
        
        // 更新聊天室
        db.chatroom.updateOne(
            {_id: room._id},
            {$set: {
                last_message: preview,
                last_message_time: lastMsg.created_at
            }}
        );
        
        updatedCount++;
        print(`✅ 更新: ${room.name} - "${preview}"`);
    } else {
        noMessageCount++;
        print(`⚠️  無訊息: ${room.name}`);
    }
});

print(`\n完成！`);
print(`✅ 已更新: ${updatedCount} 個聊天室`);
print(`⚠️  無訊息: ${noMessageCount} 個聊天室`);

