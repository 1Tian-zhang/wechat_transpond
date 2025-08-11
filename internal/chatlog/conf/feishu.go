package conf

// FeishuSyncConfig 飞书同步配置
type FeishuSyncConfig struct {
	AppID     string              `mapstructure:"APP_ID" json:"APP_ID"`
	AppSecret string              `mapstructure:"APP_SECRET" json:"APP_SECRET"`
	SyncMap   []FeishuSyncMapItem `mapstructure:"sync_map" json:"sync_map"`
}

// FeishuSyncMapItem 飞书同步映射项
type FeishuSyncMapItem struct {
	ChatName  string `mapstructure:"chat_name" json:"chat_name"`
	TargetURL string `mapstructure:"target_url" json:"target_url"`
}

// GetFeishuConfig 从ProcessConfig中获取飞书配置
func (pc *ProcessConfig) GetFeishuConfig() (*FeishuSyncConfig, error) {
	if pc.Sync2Web == nil {
		return nil, nil
	}

	feishuData, exists := pc.Sync2Web["feishu"]
	if !exists {
		return nil, nil
	}

	// 将interface{}转换为map[string]interface{}
	feishuMap, ok := feishuData.(map[string]interface{})
	if !ok {
		return nil, nil
	}

	config := &FeishuSyncConfig{}

	// 解析APP_ID
	if appID, exists := feishuMap["app_id"]; exists {
		if str, ok := appID.(string); ok {
			config.AppID = str
		}
	}

	// 解析APP_SECRET
	if appSecret, exists := feishuMap["app_secret"]; exists {
		if str, ok := appSecret.(string); ok {
			config.AppSecret = str
		}
	}

	// 解析sync_map
	if syncMapData, exists := feishuMap["sync_map"]; exists {
		if syncMapSlice, ok := syncMapData.([]interface{}); ok {
			for _, item := range syncMapSlice {
				if itemMap, ok := item.(map[string]interface{}); ok {
					syncItem := FeishuSyncMapItem{}

					if chatName, exists := itemMap["chat_name"]; exists {
						if str, ok := chatName.(string); ok {
							syncItem.ChatName = str
						}
					}

					if targetURL, exists := itemMap["target_url"]; exists {
						if str, ok := targetURL.(string); ok {
							syncItem.TargetURL = str
						}
					}

					config.SyncMap = append(config.SyncMap, syncItem)
				}
			}
		}
	}

	return config, nil
}
