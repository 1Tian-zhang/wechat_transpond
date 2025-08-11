package feishu

import (
	"fmt"
	"sync"
	"time"

	"github.com/sjzar/chatlog/internal/chatlog/conf"
	"github.com/sjzar/chatlog/internal/chatlog/database"
)

// Manager 飞书同步管理器
type Manager struct {
	services map[string]*Service
	db       *database.Service
	mutex    sync.RWMutex
}

// NewManager 创建飞书同步管理器
func NewManager(db *database.Service) *Manager {
	return &Manager{
		services: make(map[string]*Service),
		db:       db,
	}
}

// AddService 添加飞书服务
func (m *Manager) AddService(account string, config *conf.FeishuSyncConfig) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	service, err := NewService(config)
	if err != nil {
		return fmt.Errorf("failed to create feishu service for account %s: %w", account, err)
	}

	m.services[account] = service
	return nil
}

// RemoveService 移除飞书服务
func (m *Manager) RemoveService(account string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.services, account)
}

// GetService 获取飞书服务
func (m *Manager) GetService(account string) *Service {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.services[account]
}

// SyncChatlog 同步聊天记录到飞书
func (m *Manager) SyncChatlog(account string, syncItem *conf.FeishuSyncMapItem, startTime, endTime time.Time) error {
	service := m.GetService(account)
	if service == nil {
		return fmt.Errorf("feishu service not found for account: %s", account)
	}

	// 获取聊天记录
	messages, err := m.db.GetMessages(startTime, endTime, syncItem.ChatName, "", "", 0, 0)
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	if len(messages) == 0 {
		return nil // 没有消息需要同步
	}

	// 同步到飞书
	return service.SyncChatlog(syncItem, messages)
}

// SyncAllAccounts 同步所有账户的聊天记录
func (m *Manager) SyncAllAccounts(processConfigs map[string]conf.ProcessConfig) error {
	for account, config := range processConfigs {
		// 检查是否有飞书同步配置
		feishuConfig, err := config.GetFeishuConfig()
		if err != nil {
			continue // 跳过配置错误的账户
		}

		if feishuConfig == nil {
			continue // 跳过没有飞书配置的账户
		}

		// 添加或更新飞书服务
		if err := m.AddService(account, feishuConfig); err != nil {
			continue // 跳过服务创建失败的账户
		}

		// 同步每个聊天记录
		for _, syncItem := range feishuConfig.SyncMap {
			// 计算同步时间范围（比如最近24小时）
			endTime := time.Now()
			startTime := endTime.Add(-24 * time.Hour)

			if err := m.SyncChatlog(account, &syncItem, startTime, endTime); err != nil {
				// 记录错误但继续处理其他同步项
				fmt.Printf("Failed to sync chatlog for account %s, chat %s: %v\n",
					account, syncItem.ChatName, err)
			}
		}
	}

	return nil
}

// StartPeriodicSync 启动定期同步
func (m *Manager) StartPeriodicSync(processConfigs map[string]conf.ProcessConfig, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.SyncAllAccounts(processConfigs); err != nil {
				fmt.Printf("Periodic sync failed: %v\n", err)
			}
		}
	}
}
