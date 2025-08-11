package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sjzar/chatlog/internal/chatlog/conf"
	"github.com/sjzar/chatlog/internal/chatlog/feishu"
	"github.com/sjzar/chatlog/internal/errors"
)

// SyncToFeishuRequest 飞书同步请求
type SyncToFeishuRequest struct {
	Account   string    `json:"account" binding:"required"`
	ChatName  string    `json:"chat_name" binding:"required"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// SyncToFeishuResponse 飞书同步响应
type SyncToFeishuResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SyncToFeishu 手动触发飞书同步
func (s *Service) SyncToFeishu(c *gin.Context) {
	var req SyncToFeishuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errors.Err(c, errors.InvalidArg(err.Error()))
		return
	}

	// 获取账户配置
	configs := s.ctx.History
	config, exists := configs[req.Account]
	if !exists {
		errors.Err(c, errors.InvalidArg("account not found"))
		return
	}

	// 获取飞书配置
	feishuConfig, err := config.GetFeishuConfig()
	if err != nil {
		errors.Err(c, errors.New(nil, 500, "failed to get feishu config"))
		return
	}

	if feishuConfig == nil {
		errors.Err(c, errors.InvalidArg("feishu config not found"))
		return
	}

	// 查找对应的同步项
	var syncItem *conf.FeishuSyncMapItem
	for _, item := range feishuConfig.SyncMap {
		if item.ChatName == req.ChatName {
			syncItem = &item
			break
		}
	}

	if syncItem == nil {
		errors.Err(c, errors.InvalidArg("chat name not found in sync map"))
		return
	}

	// 设置默认时间范围（使用UTC+8时区）
	beijingLoc := time.FixedZone("Asia/Shanghai", 8*3600) // UTC+8

	if req.StartTime.IsZero() {
		// 默认开始时间：24小时前，使用北京时间
		req.StartTime = time.Now().In(beijingLoc).Add(-24 * time.Hour)
	} else {
		// 如果提供了时间但没有时区信息，假设是北京时间
		if req.StartTime.Location() == time.UTC {
			req.StartTime = req.StartTime.In(beijingLoc)
		}
	}

	if req.EndTime.IsZero() {
		// 默认结束时间：当前时间，使用北京时间
		req.EndTime = time.Now().In(beijingLoc)
	} else {
		// 如果提供了时间但没有时区信息，假设是北京时间
		if req.EndTime.Location() == time.UTC {
			req.EndTime = req.EndTime.In(beijingLoc)
		}
	}

	// 打印调试信息
	fmt.Printf("Debug: Time range - Start: %s (%s), End: %s (%s)\n",
		req.StartTime.Format("2006-01-02 15:04:05"),
		req.StartTime.Location().String(),
		req.EndTime.Format("2006-01-02 15:04:05"),
		req.EndTime.Location().String())

	// 创建飞书服务
	feishuService, err := feishu.NewService(feishuConfig)
	if err != nil {
		errors.Err(c, errors.New(nil, 500, "failed to create feishu service"))
		return
	}

	// 获取聊天记录
	messages, err := s.db.GetMessages(req.StartTime, req.EndTime, req.ChatName, "", "", 0, 0)
	if err != nil {
		errors.Err(c, errors.New(nil, 500, "failed to get messages"))
		return
	}

	if len(messages) == 0 {
		c.JSON(http.StatusOK, SyncToFeishuResponse{
			Success: true,
			Message: "No messages to sync",
		})
		return
	}

	// 同步到飞书
	if err := feishuService.SyncChatlog(syncItem, messages); err != nil {
		errors.Err(c, errors.New(nil, 500, "failed to sync to feishu: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, SyncToFeishuResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully synced %d messages to feishu", len(messages)),
	})
}

// GetFeishuSyncStatus 获取飞书同步状态
func (s *Service) GetFeishuSyncStatus(c *gin.Context) {
	account := c.Query("account")
	if account == "" {
		errors.Err(c, errors.InvalidArg("account parameter required"))
		return
	}

	// 获取账户配置
	configs := s.ctx.History
	config, exists := configs[account]
	if !exists {
		errors.Err(c, errors.InvalidArg("account not found"))
		return
	}

	// 获取飞书配置
	feishuConfig, err := config.GetFeishuConfig()
	if err != nil {
		errors.Err(c, errors.New(nil, 500, "failed to get feishu config"))
		return
	}

	if feishuConfig == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"message": "Feishu sync not configured",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":   true,
		"app_id":    feishuConfig.AppID,
		"sync_maps": feishuConfig.SyncMap,
	})
}
