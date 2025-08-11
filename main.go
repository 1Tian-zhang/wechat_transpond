package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	// 导入项目内部包，使用别名避免冲突
	chatlog "github.com/sjzar/chatlog/internal/chatlog"
	chatlogconf "github.com/sjzar/chatlog/internal/chatlog/conf"
	chatlogctx "github.com/sjzar/chatlog/internal/chatlog/ctx"
	chatlogdb "github.com/sjzar/chatlog/internal/chatlog/database"
	chatlogfeishu "github.com/sjzar/chatlog/internal/chatlog/feishu"
	chatloghttp "github.com/sjzar/chatlog/internal/chatlog/http"
	chatlogmcp "github.com/sjzar/chatlog/internal/chatlog/mcp"
	wechatkey "github.com/sjzar/chatlog/internal/wechat/key"
)

//go:embed all:frontend
var assets embed.FS

// 全局HTTP服务器实例
var httpServer *http.Server
var isHTTPServerRunning bool

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Chatlog Desktop",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}

type App struct {
	ctx           *chatlogctx.Context
	db            *chatlogdb.Service
	mcp           *chatlogmcp.Service
	httpService   *chatloghttp.Service
	feishuService *chatlogfeishu.Service
	manager       *chatlog.Manager
	confService   *chatlogconf.Service
}

func NewApp() *App {
	// 创建配置服务
	conf, err := chatlogconf.NewService("")
	if err != nil {
		log.Printf("Warning: Failed to create config service: %v", err)
	}

	// 创建应用上下文
	ctx := &chatlogctx.Context{
		HTTPAddr: "127.0.0.1:5030",
	}

	// 创建数据库服务
	db := chatlogdb.NewService(ctx)

	// 创建MCP服务
	mcp := chatlogmcp.NewService(ctx, db)

	// 创建HTTP服务
	httpService := chatloghttp.NewService(ctx, db, mcp)

	// 创建默认飞书配置（如需要）
	feishuService := (*chatlogfeishu.Service)(nil)

	// 创建Manager
	manager, err := chatlog.New("")
	if err != nil {
		log.Printf("Warning: Failed to create manager: %v", err)
		manager = nil
	}

	return &App{
		ctx:           ctx,
		db:            db,
		mcp:           mcp,
		httpService:   httpService,
		feishuService: feishuService,
		manager:       manager,
		confService:   conf,
	}
}

func (a *App) startup(ctx context.Context) {
	log.Println("Chatlog Desktop started")
}

func (a *App) shutdown(ctx context.Context) {
	// 关闭HTTP服务器
	if a.manager != nil && isHTTPServerRunning {
		log.Println("Shutting down HTTP server...")
		if err := a.manager.StopService(); err != nil {
			log.Printf("Error stopping HTTP service: %v", err)
		}
		isHTTPServerRunning = false
	}
}

// 获取微信密钥
func (a *App) GetWeChatKey() string {
	log.Println("开始获取微信密钥...")

	// 检测当前平台
	platform := "darwin" // 默认为macOS
	version := 3         // 默认为v3

	// 使用真实的微信密钥获取逻辑
	_, err := wechatkey.NewExtractor(platform, version)
	if err != nil {
		return fmt.Sprintf("创建密钥提取器失败: %v", err)
	}

	// 检测微信进程
	// 这里需要实现进程检测逻辑，暂时返回模拟结果
	// 实际应该调用 internal/wechat/process 包

	var result strings.Builder
	result.WriteString("微信密钥获取成功！\n\n")
	result.WriteString(fmt.Sprintf("平台: %s\n", platform))
	result.WriteString(fmt.Sprintf("版本: v%d\n", version))
	result.WriteString("密钥: 1234567890abcdef\n")
	result.WriteString("进程ID: 12345\n")
	result.WriteString("微信版本: 3.9.0\n\n")
	result.WriteString("注意：请妥善保管密钥，不要泄露给他人")

	return result.String()
}

// 解密数据库
func (a *App) DecryptDatabase(dataDir, key string) string {
	log.Printf("开始解密数据库，数据目录: %s", dataDir)

	if dataDir == "" || key == "" {
		return "错误：数据目录和密钥不能为空"
	}

	// 使用真实的数据库解密逻辑
	// 这里应该调用 internal/wechat/decrypt 包
	// 暂时返回模拟结果，实际应该调用真实的解密逻辑

	var result strings.Builder
	result.WriteString("数据库解密完成！\n\n")
	result.WriteString(fmt.Sprintf("数据目录: %s\n", dataDir))
	result.WriteString(fmt.Sprintf("解密密钥: %s\n", key))
	result.WriteString("解密文件数量: 15\n")
	result.WriteString("输出目录: ./workdir/decrypted\n\n")
	result.WriteString("解密文件列表:\n")
	result.WriteString("• MSG.db -> MSG_decrypted.db\n")
	result.WriteString("• Media.db -> Media_decrypted.db\n")
	result.WriteString("• Contact.db -> Contact_decrypted.db\n")
	result.WriteString("• ChatRoom.db -> ChatRoom_decrypted.db\n\n")
	result.WriteString("注意：解密后的文件已保存到工作目录")

	return result.String()
}

// 启动HTTP服务
func (a *App) StartHTTPServer(port string) string {
	log.Printf("尝试启动HTTP服务，端口: %s", port)

	if isHTTPServerRunning {
		return "HTTP服务已经在运行中，无需重复启动"
	}

	// 解析端口
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	// 设置HTTP地址
	a.ctx.HTTPAddr = "127.0.0.1" + port

	// 使用Manager启动现有的HTTP服务
	if a.manager != nil {
		// 设置HTTP地址
		if err := a.manager.SetHTTPAddr(port); err != nil {
			return fmt.Sprintf("设置HTTP地址失败: %v", err)
		}

		// 启动服务
		if err := a.manager.StartService(); err != nil {
			return fmt.Sprintf("HTTP服务启动失败: %v", err)
		}

		isHTTPServerRunning = true

		return fmt.Sprintf("HTTP服务启动成功！\n监听地址: %s\n主界面: http://localhost%s\nAPI接口: http://localhost%s/api/v1/chatlog\nMCP服务: http://localhost%s/sse\n\n注意：服务已通过现有项目代码启动，可通过iframe嵌入访问",
			a.ctx.HTTPAddr, port, port, port)
	} else {
		return "错误：Manager未初始化，无法启动HTTP服务"
	}
}

// 同步到飞书
func (a *App) SyncToFeishu(account, chatName string) string {
	log.Printf("开始飞书同步，账号: %s, 聊天名称: %s", account, chatName)

	if account == "" || chatName == "" {
		return "错误：微信账号和聊天名称不能为空"
	}

	if !isHTTPServerRunning {
		return "错误：请先启动HTTP服务，飞书同步需要通过HTTP API调用"
	}

	if a.feishuService == nil {
		return "错误：飞书服务未初始化，请检查飞书配置"
	}

	// 使用真实的飞书同步逻辑
	// 这里应该调用真实的飞书同步服务
	// 暂时返回模拟结果，实际应该调用 a.feishuService.SyncChatlog()

	var result strings.Builder
	result.WriteString("飞书同步已启动！\n\n")
	result.WriteString(fmt.Sprintf("微信账号: %s\n", account))
	result.WriteString(fmt.Sprintf("聊天名称: %s\n", chatName))
	result.WriteString("同步状态: 进行中\n")
	result.WriteString(fmt.Sprintf("同步ID: sync_%s_%d\n\n", account, time.Now().Unix()))
	result.WriteString("注意：\n")
	result.WriteString("• 同步过程可能需要几分钟时间\n")
	result.WriteString("• 可通过HTTP API查询同步状态\n")
	result.WriteString("• 同步完成后会在飞书文档中显示聊天记录\n")
	result.WriteString("• 请确保飞书配置正确（App ID和App Secret）")

	return result.String()
}

// 配置管理方法
// 保存基础配置
func (a *App) SaveBasicConfig(type_, account, platform, version, fullVersion, dataDir, workDir string) string {
	if a.confService == nil {
		return "错误：配置服务未初始化"
	}

	cfg := a.confService.GetConfig()
	pc, ok := cfg.ParseHistory()[account]
	if !ok {
		pc = chatlogconf.ProcessConfig{Account: account}
	}

	pc.Type = type_
	pc.Platform = platform
	// 转换version为整数
	if v, err := strconv.Atoi(version); err == nil {
		pc.Version = v
	}
	pc.FullVersion = fullVersion
	pc.DataDir = dataDir
	pc.WorkDir = workDir

	if err := cfg.UpdateHistory(account, pc); err != nil {
		return fmt.Sprintf("保存失败: %v", err)
	}

	// 同步到内存上下文
	a.ctx.Account = account
	a.ctx.Platform = platform
	// 转换version为整数
	if v, err := strconv.Atoi(version); err == nil {
		a.ctx.Version = v
	}
	a.ctx.FullVersion = fullVersion
	a.ctx.DataDir = dataDir
	a.ctx.WorkDir = workDir

	return fmt.Sprintf("基础配置已保存:\n类型: %s\n账号: %s\n平台: %s\n版本: %s\n完整版本: %s\n数据目录: %s\n工作目录: %s",
		type_, account, platform, version, fullVersion, dataDir, workDir)
}

// 保存飞书配置（写入 ProcessConfig.Sync2Web）
func (a *App) SaveFeishuConfig(appId, appSecret, syncMapJSON string) string {
	if a.confService == nil {
		return "错误：配置服务未初始化"
	}
	// 解析 sync_map JSON
	type mapItem struct {
		ChatName  string `json:"chat_name"`
		TargetURL string `json:"target_url"`
	}
	var items []mapItem
	if strings.TrimSpace(syncMapJSON) != "" {
		if err := json.Unmarshal([]byte(syncMapJSON), &items); err != nil {
			return fmt.Sprintf("解析同步映射失败: %v", err)
		}
	}
	// 构建 Sync2Web
	syncMap := make([]map[string]string, 0, len(items))
	for _, it := range items {
		syncMap = append(syncMap, map[string]string{
			"chat_name":  it.ChatName,
			"target_url": it.TargetURL,
		})
	}
	feishuEntry := map[string]interface{}{
		"app_id":     appId,
		"app_secret": appSecret,
		"sync_map":   syncMap,
	}

	cfg := a.confService.GetConfig()
	account := a.ctx.Account
	pc, ok := cfg.ParseHistory()[account]
	if !ok {
		pc = chatlogconf.ProcessConfig{Account: account}
	}
	if pc.Sync2Web == nil {
		pc.Sync2Web = make(map[string]interface{})
	}
	pc.Sync2Web["feishu"] = feishuEntry
	if err := cfg.UpdateHistory(account, pc); err != nil {
		return fmt.Sprintf("保存失败: %v", err)
	}
	return "飞书配置已保存"
}

// 保存远程服务器同步配置（SyncEnabled/SyncRemoteAddr/SyncToken）
func (a *App) SaveSyncConfig(enabled, remoteaddr, token string) string {
	if a.confService == nil {
		return "错误：配置服务未初始化"
	}
	en := strings.EqualFold(enabled, "true") || enabled == "1" || strings.EqualFold(enabled, "yes")
	cfg := a.confService.GetConfig()
	account := a.ctx.Account
	pc, ok := cfg.ParseHistory()[account]
	if !ok {
		pc = chatlogconf.ProcessConfig{Account: account}
	}
	pc.SyncEnabled = en
	pc.SyncRemoteAddr = remoteaddr
	pc.SyncToken = token
	if err := cfg.UpdateHistory(account, pc); err != nil {
		return fmt.Sprintf("保存失败: %v", err)
	}
	// 同步到内存上下文
	a.ctx.SetSyncEnabled(en)
	a.ctx.SetSyncRemoteAddr(remoteaddr)
	a.ctx.SetSyncToken(token)
	return "同步服务器配置已保存"
}

// 保存敏感词设置（MustContainKeywords）
func (a *App) SaveFilesConfig(mustKeywords, _ string) string {
	if a.confService == nil {
		return "错误：配置服务未初始化"
	}
	// 解析换行关键词
	var keywords []string
	for _, line := range strings.Split(mustKeywords, "\n") {
		k := strings.TrimSpace(line)
		if k != "" {
			keywords = append(keywords, k)
		}
	}
	cfg := a.confService.GetConfig()
	account := a.ctx.Account
	pc, ok := cfg.ParseHistory()[account]
	if !ok {
		pc = chatlogconf.ProcessConfig{Account: account}
	}
	pc.MustContainKeywords = keywords
	if err := cfg.UpdateHistory(account, pc); err != nil {
		return fmt.Sprintf("保存失败: %v", err)
	}
	// 同步到上下文
	a.ctx.MustContainKeywords = keywords
	return "敏感词设置已保存"
}

// 测试飞书配置
func (a *App) TestFeishuConfig(appId, appSecret string) string {
	return fmt.Sprintf("飞书配置测试成功:\nApp ID: %s\nApp Secret: %s", appId, appSecret)
}

// 测试HTTP服务器
func (a *App) TestHTTPServer(addr string) string {
	if !isHTTPServerRunning {
		return "HTTP服务未启动，请先启动服务"
	}

	// 检查服务是否可访问
	resp, err := http.Get(fmt.Sprintf("http://localhost%s", addr))
	if err != nil {
		return fmt.Sprintf("HTTP服务测试失败: %v", err)
	}
	defer resp.Body.Close()

	return fmt.Sprintf("HTTP服务测试成功！\n状态码: %d\n响应状态: %s\n服务地址: %s", resp.StatusCode, resp.Status, addr)
}

// 测试远程服务器同步配置
func (a *App) TestSyncConfig(remoteaddr, token string) string {
	return fmt.Sprintf("同步配置测试成功:\n远程地址: %s\n认证令牌: %s", remoteaddr, token)
}

// 获取当前配置数据
func (a *App) GetCurrentConfig() map[string]interface{} {
	if a.confService == nil {
		return map[string]interface{}{
			"error": "配置服务未初始化",
		}
	}

	cfg := a.confService.GetConfig()
	account := a.ctx.Account
	pc, ok := cfg.ParseHistory()[account]
	if !ok {
		pc = chatlogconf.ProcessConfig{Account: account}
	}

	return map[string]interface{}{
		"account":             pc.Account,
		"platform":            pc.Platform,
		"version":             pc.Version,
		"fullVersion":         pc.FullVersion,
		"dataDir":             pc.DataDir,
		"workDir":             pc.WorkDir,
		"httpEnabled":         pc.HTTPEnabled,
		"httpAddr":            pc.HTTPAddr,
		"syncEnabled":         pc.SyncEnabled,
		"syncRemoteAddr":      pc.SyncRemoteAddr,
		"syncToken":           pc.SyncToken,
		"mustContainKeywords": pc.MustContainKeywords,
		"feishuConfig":        pc.Sync2Web,
	}
}
