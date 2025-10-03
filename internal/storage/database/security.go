package database

import (
	"fmt"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ValidateObjectID 驗證 MongoDB ObjectID 格式
func ValidateObjectID(id string) error {
	if len(id) != 24 {
		return fmt.Errorf("無效的 ObjectID 格式")
	}
	
	// 只允許十六進制字符
	matched, _ := regexp.MatchString("^[a-fA-F0-9]{24}$", id)
	if !matched {
		return fmt.Errorf("無效的 ObjectID 格式")
	}
	
	return nil
}

// SanitizeFieldName 消毒字段名（防止 MongoDB 操作符注入）
func SanitizeFieldName(fieldName string) string {
	// 移除 $ 符號（MongoDB 操作符）
	fieldName = strings.ReplaceAll(fieldName, "$", "")
	
	// 移除 . 符號（嵌套字段訪問）
	fieldName = strings.ReplaceAll(fieldName, ".", "")
	
	return fieldName
}

// SafeRegexQuery 創建安全的正則表達式查詢（防止 ReDoS）
func SafeRegexQuery(pattern string) bson.M {
	// 轉義特殊字符
	pattern = regexp.QuoteMeta(pattern)
	
	return bson.M{
		"$regex":   pattern,
		"$options": "i", // 不區分大小寫
	}
}

// ValidateQueryOperators 驗證查詢中不包含危險的操作符
func ValidateQueryOperators(query interface{}) error {
	switch v := query.(type) {
	case bson.M:
		for key := range v {
			if strings.HasPrefix(key, "$") {
				// 只允許白名單中的操作符
				allowedOps := map[string]bool{
					"$eq":  true,
					"$ne":  true,
					"$gt":  true,
					"$gte": true,
					"$lt":  true,
					"$lte": true,
					"$in":  true,
					"$nin": true,
					"$and": true,
					"$or":  true,
				}
				
				if !allowedOps[key] {
					return fmt.Errorf("不允許的查詢操作符: %s", key)
				}
			}
		}
	case map[string]interface{}:
		for key := range v {
			if strings.HasPrefix(key, "$") {
				allowedOps := map[string]bool{
					"$eq":  true,
					"$ne":  true,
					"$gt":  true,
					"$gte": true,
					"$lt":  true,
					"$lte": true,
					"$in":  true,
					"$nin": true,
					"$and": true,
					"$or":  true,
				}
				
				if !allowedOps[key] {
					return fmt.Errorf("不允許的查詢操作符: %s", key)
				}
			}
		}
	}
	
	return nil
}

// SafeUpdateQuery 創建安全的更新查詢
func SafeUpdateQuery(updates map[string]interface{}) (bson.M, error) {
	// 驗證更新字段名
	safeUpdates := make(map[string]interface{})
	
	for key, value := range updates {
		// 不允許以 $ 開頭的字段名（操作符）
		if strings.HasPrefix(key, "$") {
			return nil, fmt.Errorf("不允許的更新字段: %s", key)
		}
		
		// 消毒字段名
		safeKey := SanitizeFieldName(key)
		safeUpdates[safeKey] = value
	}
	
	return bson.M{"$set": safeUpdates}, nil
}

// SafeStringValue 消毒字符串值（防止注入）
func SafeStringValue(value string) string {
	// 移除 NULL 字符
	value = strings.ReplaceAll(value, "\x00", "")
	
	// 移除 MongoDB 特殊字符
	value = strings.ReplaceAll(value, "$", "")
	value = strings.ReplaceAll(value, "{", "")
	value = strings.ReplaceAll(value, "}", "")
	
	return value
}

// BuildSafeFilter 構建安全的過濾器
func BuildSafeFilter(filters map[string]interface{}) (bson.M, error) {
	safeFilter := bson.M{}
	
	for key, value := range filters {
		// 驗證字段名
		if strings.HasPrefix(key, "$") {
			return nil, fmt.Errorf("不允許的過濾字段: %s", key)
		}
		
		// 消毒字段名
		safeKey := SanitizeFieldName(key)
		
		// 如果值是字符串，消毒它
		if strValue, ok := value.(string); ok {
			safeFilter[safeKey] = SafeStringValue(strValue)
		} else {
			safeFilter[safeKey] = value
		}
	}
	
	return safeFilter, nil
}

// ValidateLimit 驗證並限制查詢數量
func ValidateLimit(limit int) int {
	const maxLimit = 1000
	const defaultLimit = 20
	
	if limit <= 0 {
		return defaultLimit
	}
	
	if limit > maxLimit {
		return maxLimit
	}
	
	return limit
}

// ValidateSkip 驗證並限制跳過數量
func ValidateSkip(skip int) int {
	const maxSkip = 100000
	
	if skip < 0 {
		return 0
	}
	
	if skip > maxSkip {
		return maxSkip
	}
	
	return skip
}

