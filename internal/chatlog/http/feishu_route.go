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

	// 设置默认时间范围
	if req.StartTime.IsZero() {
		req.StartTime = time.Now().Add(-24 * time.Hour)
	}
	if req.EndTime.IsZero() {
		req.EndTime = time.Now()
	}

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
