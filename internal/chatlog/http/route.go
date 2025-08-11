package http

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sjzar/chatlog/internal/errors"
	"github.com/sjzar/chatlog/pkg/util"
	"github.com/sjzar/chatlog/pkg/util/dat2img"
	"github.com/sjzar/chatlog/pkg/util/silk"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// EFS holds embedded file system data for static assets.
//
//go:embed static
var EFS embed.FS

func (s *Service) initRouter() {

	router := s.GetRouter()

	staticDir, _ := fs.Sub(EFS, "static")
	router.StaticFS("/static", http.FS(staticDir))
	router.StaticFileFS("/favicon.ico", "./favicon.ico", http.FS(staticDir))
	router.StaticFileFS("/", "./index.htm", http.FS(staticDir))

	// Media
	router.GET("/image/:key", s.GetImage)
	router.GET("/video/:key", s.GetVideo)
	router.GET("/file/:key", s.GetFile)
	router.GET("/voice/:key", s.GetVoice)
	router.GET("/data/*path", s.GetMediaData)

	// MCP Server
	{
		router.GET("/sse", s.mcp.HandleSSE)
		router.POST("/messages", s.mcp.HandleMessages)
		// mcp inspector is shit
		// https://github.com/modelcontextprotocol/inspector/blob/aeaf32f/server/src/index.ts#L155
		router.POST("/message", s.mcp.HandleMessages)
	}

	// API V1 Router
	api := router.Group("/api/v1")
	{
		api.GET("/chatlog", s.GetChatlog)
		api.GET("/contact", s.GetContacts)
		api.GET("/chatroom", s.GetChatRooms)
		api.GET("/session", s.GetSessions)

		// Work directory sync endpoints
		api.GET("/sync/send", s.SendWorkDir)
		api.POST("/sync/receive", s.ReceiveWorkDir)
		api.GET("/sync/refresh", s.RefreshDatabase)
		api.POST("/sync/cleanup", s.CleanupOldBackups)

		// Feishu sync endpoints
		api.POST("/feishu/sync", s.SyncToFeishu)
		api.GET("/feishu/status", s.GetFeishuSyncStatus)
	}

	router.NoRoute(s.NoRoute)
}

// NoRoute handles 404 Not Found errors. If the request URL starts with "/api"
// or "/static", it responds with a JSON error. Otherwise, it redirects to the root path.
func (s *Service) NoRoute(c *gin.Context) {
	path := c.Request.URL.Path
	switch {
	case strings.HasPrefix(path, "/api"), strings.HasPrefix(path, "/static"):
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
	default:
		c.Header("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate, value")
		c.Redirect(http.StatusFound, "/")
	}
}

func (s *Service) GetChatlog(c *gin.Context) {

	q := struct {
		Time    string `form:"time"`
		Talker  string `form:"talker"`
		Sender  string `form:"sender"`
		Keyword string `form:"keyword"`
		Limit   int    `form:"limit"`
		Offset  int    `form:"offset"`
		Format  string `form:"format"`
	}{}

	if err := c.BindQuery(&q); err != nil {
		errors.Err(c, err)
		return
	}

	var err error
	start, end, ok := util.TimeRangeOf(q.Time)
	if !ok {
		errors.Err(c, errors.InvalidArg("time"))
	}
	if q.Limit < 0 {
		q.Limit = 0
	}

	if q.Offset < 0 {
		q.Offset = 0
	}

	messages, err := s.db.GetMessages(start, end, q.Talker, q.Sender, q.Keyword, q.Limit, q.Offset)
	if err != nil {
		errors.Err(c, err)
		return
	}

	switch strings.ToLower(q.Format) {
	case "csv":
	case "json":
		// json
		c.JSON(http.StatusOK, messages)
	default:
		// plain text
		c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		for _, m := range messages {
			c.Writer.WriteString(m.PlainText(strings.Contains(q.Talker, ","), util.PerfectTimeFormat(start, end), c.Request.Host))
			c.Writer.WriteString("\n")
			c.Writer.Flush()
		}
	}
}

func (s *Service) GetContacts(c *gin.Context) {

	q := struct {
		Keyword string `form:"keyword"`
		Limit   int    `form:"limit"`
		Offset  int    `form:"offset"`
		Format  string `form:"format"`
	}{}

	if err := c.BindQuery(&q); err != nil {
		errors.Err(c, err)
		return
	}

	list, err := s.db.GetContacts(q.Keyword, q.Limit, q.Offset)
	if err != nil {
		errors.Err(c, err)
		return
	}

	format := strings.ToLower(q.Format)
	switch format {
	case "json":
		// json
		c.JSON(http.StatusOK, list)
	default:
		// csv
		if format == "csv" {
			// 浏览器访问时，会下载文件
			c.Writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
		} else {
			c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		c.Writer.WriteString("UserName,Alias,Remark,NickName\n")
		for _, contact := range list.Items {
			c.Writer.WriteString(fmt.Sprintf("%s,%s,%s,%s\n", contact.UserName, contact.Alias, contact.Remark, contact.NickName))
		}
		c.Writer.Flush()
	}
}

func (s *Service) GetChatRooms(c *gin.Context) {

	q := struct {
		Keyword string `form:"keyword"`
		Limit   int    `form:"limit"`
		Offset  int    `form:"offset"`
		Format  string `form:"format"`
	}{}

	if err := c.BindQuery(&q); err != nil {
		errors.Err(c, err)
		return
	}

	list, err := s.db.GetChatRooms(q.Keyword, q.Limit, q.Offset)
	if err != nil {
		errors.Err(c, err)
		return
	}
	format := strings.ToLower(q.Format)
	switch format {
	case "json":
		// json
		c.JSON(http.StatusOK, list)
	default:
		// csv
		if format == "csv" {
			// 浏览器访问时，会下载文件
			c.Writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
		} else {
			c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		c.Writer.WriteString("Name,Remark,NickName,Owner,UserCount\n")
		for _, chatRoom := range list.Items {
			c.Writer.WriteString(fmt.Sprintf("%s,%s,%s,%s,%d\n", chatRoom.Name, chatRoom.Remark, chatRoom.NickName, chatRoom.Owner, len(chatRoom.Users)))
		}
		c.Writer.Flush()
	}
}

func (s *Service) GetSessions(c *gin.Context) {

	q := struct {
		Keyword string `form:"keyword"`
		Limit   int    `form:"limit"`
		Offset  int    `form:"offset"`
		Format  string `form:"format"`
	}{}

	if err := c.BindQuery(&q); err != nil {
		errors.Err(c, err)
		return
	}

	sessions, err := s.db.GetSessions(q.Keyword, q.Limit, q.Offset)
	if err != nil {
		errors.Err(c, err)
		return
	}
	format := strings.ToLower(q.Format)
	switch format {
	case "csv":
		c.Writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		c.Writer.WriteString("UserName,NOrder,NickName,Content,NTime\n")
		for _, session := range sessions.Items {
			c.Writer.WriteString(fmt.Sprintf("%s,%d,%s,%s,%s\n", session.UserName, session.NOrder, session.NickName, strings.ReplaceAll(session.Content, "\n", "\\n"), session.NTime))
		}
		c.Writer.Flush()
	case "json":
		// json
		c.JSON(http.StatusOK, sessions)
	default:
		c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()
		for _, session := range sessions.Items {
			c.Writer.WriteString(session.PlainText(120))
			c.Writer.WriteString("\n")
		}
		c.Writer.Flush()
	}
}

func (s *Service) GetImage(c *gin.Context) {
	s.GetMedia(c, "image")
}

func (s *Service) GetVideo(c *gin.Context) {
	s.GetMedia(c, "video")
}

func (s *Service) GetFile(c *gin.Context) {
	s.GetMedia(c, "file")
}
func (s *Service) GetVoice(c *gin.Context) {
	s.GetMedia(c, "voice")
}

func (s *Service) GetMedia(c *gin.Context, _type string) {
	key := c.Param("key")
	if key == "" {
		errors.Err(c, errors.InvalidArg(key))
		return
	}

	media, err := s.db.GetMedia(_type, key)
	if err != nil {
		errors.Err(c, err)
		return
	}

	if c.Query("info") != "" {
		c.JSON(http.StatusOK, media)
		return
	}

	switch media.Type {
	case "voice":
		s.HandleVoice(c, media.Data)
	default:
		c.Redirect(http.StatusFound, "/data/"+media.Path)
	}

}

func (s *Service) GetMediaData(c *gin.Context) {
	relativePath := filepath.Clean(c.Param("path"))

	absolutePath := filepath.Join(s.ctx.DataDir, relativePath)

	if _, err := os.Stat(absolutePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "File not found",
		})
		return
	}

	ext := strings.ToLower(filepath.Ext(absolutePath))
	switch {
	case ext == ".dat":
		s.HandleDatFile(c, absolutePath)
	default:
		// 直接返回文件
		c.File(absolutePath)
	}

}

func (s *Service) HandleDatFile(c *gin.Context, path string) {

	b, err := os.ReadFile(path)
	if err != nil {
		errors.Err(c, err)
		return
	}
	out, ext, err := dat2img.Dat2Image(b)
	if err != nil {
		c.File(path)
		return
	}

	switch ext {
	case "jpg":
		c.Data(http.StatusOK, "image/jpeg", out)
	case "png":
		c.Data(http.StatusOK, "image/png", out)
	case "gif":
		c.Data(http.StatusOK, "image/gif", out)
	case "bmp":
		c.Data(http.StatusOK, "image/bmp", out)
	default:
		c.File(path)
	}
}

func (s *Service) HandleVoice(c *gin.Context, data []byte) {
	out, err := silk.Silk2MP3(data)
	if err != nil {
		c.Data(http.StatusOK, "audio/silk", data)
		return
	}
	c.Data(http.StatusOK, "audio/mp3", out)
}

// SendWorkDir 发送工作目录到远程服务器
func (s *Service) SendWorkDir(c *gin.Context) {
	// 检查是否启用了同步功能
	if !s.ctx.SyncEnabled {
		errors.Err(c, errors.BadRequest("sync is not enabled"))
		return
	}

	if s.ctx.SyncRemoteAddr == "" {
		errors.Err(c, errors.BadRequest("remote address not configured"))
		return
	}

	if s.ctx.WorkDir == "" {
		errors.Err(c, errors.BadRequest("work directory not configured"))
		return
	}

	// 检查工作目录是否存在
	if _, err := os.Stat(s.ctx.WorkDir); os.IsNotExist(err) {
		errors.Err(c, errors.BadRequest("work directory does not exist"))
		return
	}

	log.Info().Msgf("Starting to send work directory: %s", s.ctx.WorkDir)

	// 创建压缩包
	archiveData, filename, err := s.createWorkDirArchive()
	if err != nil {
		log.Err(err).Msg("Failed to create archive")
		errors.Err(c, errors.InternalServerError("failed to create archive"))
		return
	}

	log.Info().Msgf("Created archive: %s (size: %s)", filename, formatSize(int64(len(archiveData))))

	// 发送到远程服务器
	err = s.sendArchiveToRemote(archiveData, filename)
	if err != nil {
		log.Err(err).Msg("Failed to send archive to remote")
		errors.Err(c, errors.InternalServerError("failed to send archive"))
		return
	}

	log.Info().Msg("Work directory sent successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":     "Work directory sent successfully",
		"archive":     filename,
		"size":        len(archiveData),
		"remote_addr": s.ctx.SyncRemoteAddr,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

// ReceiveWorkDir 接收远程工作目录数据
func (s *Service) ReceiveWorkDir(c *gin.Context) {
	// 验证token
	token := c.GetHeader("Authorization")
	if token != "" {
		token = strings.TrimPrefix(token, "Bearer ")
	}

	if s.ctx.SyncToken != "" && token != s.ctx.SyncToken {
		errors.Err(c, errors.Unauthorized("invalid token"))
		return
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("archive")
	if err != nil {
		errors.Err(c, errors.BadRequest("archive file is required"))
		return
	}
	defer file.Close()

	// 文件大小限制 (2GB)
	const maxSize = 2 * 1024 * 1024 * 1024
	if header.Size > maxSize {
		errors.Err(c, errors.BadRequest("file too large"))
		return
	}

	log.Info().
		Str("filename", header.Filename).
		Int64("size", header.Size).
		Msg("Received work directory archive")

	// 处理上传的压缩包
	err = s.processWorkDirArchive(file, header.Filename)
	if err != nil {
		log.Err(err).Msg("Failed to process work directory archive")
		errors.Err(c, errors.InternalServerError("failed to process archive"))
		return
	}

	log.Info().Msg("Work directory received and extracted successfully")

	// 刷新数据库连接和session缓存
	err = s.refreshDatabaseAndSession()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to refresh database and session after workdir update")
		// 不返回错误，因为文件已经成功接收，只是刷新可能有问题
	} else {
		log.Info().Msg("Database and session refreshed successfully after workdir update")
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Work directory received successfully",
		"filename":  header.Filename,
		"size":      header.Size,
		"work_dir":  s.ctx.WorkDir,
		"timestamp": time.Now().Format(time.RFC3339),
		"refreshed": err == nil,
	})
}

// RefreshDatabase 手动刷新数据库连接和session缓存
func (s *Service) RefreshDatabase(c *gin.Context) {
	log.Info().Msg("Manual database refresh requested via API")

	// 调用刷新逻辑
	err := s.refreshDatabaseAndSession()
	if err != nil {
		log.Err(err).Msg("Manual database refresh failed")
		errors.Err(c, errors.InternalServerError("failed to refresh database"))
		return
	}

	log.Info().Msg("Manual database refresh completed successfully")

	// 获取一些基本统计信息
	stats := s.getDatabaseStats()

	c.JSON(http.StatusOK, gin.H{
		"message":      "Database refreshed successfully",
		"timestamp":    time.Now().Format(time.RFC3339),
		"work_dir":     s.ctx.WorkDir,
		"last_session": s.ctx.LastSession.Format(time.RFC3339),
		"stats":        stats,
	})
}

// getDatabaseStats 获取数据库基本统计信息
func (s *Service) getDatabaseStats() map[string]interface{} {
	stats := map[string]interface{}{
		"database_connected": s.db.GetDB() != nil,
	}

	// 如果数据库连接可用，获取一些统计信息
	if s.db.GetDB() != nil {
		// 获取联系人数量
		if contactsResp, err := s.db.GetContacts("", 1, 0); err == nil {
			stats["contacts_available"] = len(contactsResp.Items) > 0
		}

		// 获取聊天室数量
		if chatroomsResp, err := s.db.GetChatRooms("", 1, 0); err == nil {
			stats["chatrooms_available"] = len(chatroomsResp.Items) > 0
		}

		// 获取会话数量
		if sessionsResp, err := s.db.GetSessions("", 1, 0); err == nil {
			stats["sessions_available"] = len(sessionsResp.Items) > 0
		}
	}

	return stats
}

// CleanupOldBackups 清理过期的备份文件
func (s *Service) CleanupOldBackups(c *gin.Context) {
	var req struct {
		MaxAge string `json:"max_age"` // 例如: "24h", "7d", "30d"
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errors.Err(c, errors.BadRequest("invalid request format"))
		return
	}

	// 默认保留7天
	maxAgeStr := "168h" // 7 * 24 = 168小时
	if req.MaxAge != "" {
		maxAgeStr = req.MaxAge
	}

	maxAge, err := time.ParseDuration(maxAgeStr)
	if err != nil {
		errors.Err(c, errors.BadRequest("invalid max_age format, use format like '24h', '7d', '168h'"))
		return
	}

	log.Info().Str("max_age", maxAgeStr).Msg("Manual backup cleanup requested")

	err = s.cleanupOldBackups(maxAge)
	if err != nil {
		log.Err(err).Msg("Manual backup cleanup failed")
		errors.Err(c, errors.InternalServerError("failed to cleanup old backups"))
		return
	}

	log.Info().Msg("Manual backup cleanup completed successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":   "Backup cleanup completed successfully",
		"max_age":   maxAgeStr,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// createWorkDirArchive 创建工作目录的压缩包
func (s *Service) createWorkDirArchive() ([]byte, string, error) {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	baseDir := filepath.Base(s.ctx.WorkDir)
	filename := fmt.Sprintf("workdir_%s_%s.tar.gz", baseDir, time.Now().Format("20060102_150405"))

	err := filepath.Walk(s.ctx.WorkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warn().Err(err).Msgf("Skipping file due to error: %s", path)
			return nil // 继续处理其他文件
		}

		// 获取相对路径
		relPath, _ := filepath.Rel(s.ctx.WorkDir, path)

		// 检查是否应该排除
		if relPath != "." && s.shouldExcludeFromSync(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 创建tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// 设置相对路径，包含workdir本身的名字
		if relPath == "." {
			// 根目录使用workdir的名字
			header.Name = baseDir
		} else {
			// 子文件/目录在workdir名字下
			header.Name = filepath.ToSlash(filepath.Join(baseDir, relPath))
		}

		log.Debug().Msgf("Adding to archive: %s", header.Name)

		// 写入header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// 写入文件内容（仅对常规文件）
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, "", err
	}

	// 确保writers被关闭
	tarWriter.Close()
	gzWriter.Close()

	return buf.Bytes(), filename, nil
}

// shouldExcludeFromSync 判断是否应该从同步中排除某个文件/目录
func (s *Service) shouldExcludeFromSync(relPath string) bool {
	// 默认排除的文件和目录
	excludePatterns := []string{
		"*.tmp", "*.temp", "*.log", ".DS_Store", "Thumbs.db",
		"uploads", "temp", "cache", ".git", ".svn",
	}

	for _, pattern := range excludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return true
		}
	}

	return false
}

// sendArchiveToRemote 发送压缩包到远程服务器
func (s *Service) sendArchiveToRemote(data []byte, filename string) error {
	// 创建multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 添加文件字段
	fileWriter, err := writer.CreateFormFile("archive", filename)
	if err != nil {
		return err
	}

	_, err = fileWriter.Write(data)
	if err != nil {
		return err
	}

	// 添加其他字段
	if s.ctx.SyncToken != "" {
		writer.WriteField("token", s.ctx.SyncToken)
	}

	writer.WriteField("timestamp", time.Now().Format(time.RFC3339))
	writer.WriteField("source", "workdir")

	err = writer.Close()
	if err != nil {
		return err
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", s.ctx.SyncRemoteAddr, &buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	if s.ctx.SyncToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.ctx.SyncToken)
	}

	// 发送请求
	client := &http.Client{
		Timeout: 30 * time.Minute, // 允许大文件传输
	}

	log.Info().Msgf("Sending archive to: %s", s.ctx.SyncRemoteAddr)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote server responded with status %d: %s", resp.StatusCode, string(body))
	}

	log.Info().Msg("Archive sent successfully")
	return nil
}

// processWorkDirArchive 处理接收到的工作目录压缩包
func (s *Service) processWorkDirArchive(file io.Reader, filename string) error {
	if s.ctx.WorkDir == "" {
		return fmt.Errorf("work directory not configured")
	}

	// 创建临时目录用于备份（放在work_dir的上一层，避免嵌套）
	workDirParent := filepath.Dir(s.ctx.WorkDir)
	backupDir := filepath.Join(workDirParent, "backup", time.Now().Format("20060102_150405"))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %v", err)
	}

	// 备份当前工作目录
	log.Info().Msgf("Backing up current work directory to: %s", backupDir)
	err := s.backupWorkDir(backupDir)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to backup work directory")
	}

	// 解压新的压缩包
	err = s.extractWorkDirArchive(file, s.ctx.WorkDir)
	if err != nil {
		// 如果解压失败，尝试恢复备份
		log.Err(err).Msg("Failed to extract archive, attempting to restore backup")
		if restoreErr := s.restoreWorkDir(backupDir); restoreErr != nil {
			log.Err(restoreErr).Msg("Failed to restore backup")
		}
		return fmt.Errorf("failed to extract archive: %v", err)
	}

	// 解压成功，删除备份
	log.Info().Msg("Archive extracted successfully, removing backup")
	if err := s.removeBackup(backupDir); err != nil {
		log.Warn().Err(err).Msg("Failed to remove backup, but continuing")
	} else {
		log.Info().Msg("Backup removed successfully")
	}

	log.Info().Msg("Work directory updated successfully")
	return nil
}

// backupWorkDir 备份当前工作目录
func (s *Service) backupWorkDir(backupDir string) error {
	return filepath.Walk(s.ctx.WorkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过备份目录本身（现在备份目录在work_dir外面，不需要跳过）
		// 但为了安全，仍然检查路径是否包含backup
		if strings.Contains(path, "backup") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(s.ctx.WorkDir, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(backupDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// restoreWorkDir 从备份恢复工作目录
func (s *Service) restoreWorkDir(backupDir string) error {
	// 清空当前工作目录（备份目录现在在外面，不需要跳过）
	entries, err := os.ReadDir(s.ctx.WorkDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(s.ctx.WorkDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			log.Warn().Err(err).Msgf("Failed to remove: %s", path)
		}
	}

	// 恢复备份
	return filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(backupDir, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(s.ctx.WorkDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// extractWorkDirArchive 解压工作目录压缩包
func (s *Service) extractWorkDirArchive(file io.Reader, destDir string) error {
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	extractedCount := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %v", err)
		}

		// 跳过空路径
		if header.Name == "" {
			log.Debug().Msgf("Skipping empty path")
			continue
		}

		// 规范化路径
		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || strings.Contains(cleanName, ".."+string(os.PathSeparator)) {
			log.Warn().Msgf("Skipping dangerous path: %s", header.Name)
			continue
		}

		// 去掉workdir的根目录前缀，避免嵌套
		// 例如：wxid_v8j939e3g22s22_8d2b/db_storage/sns -> db_storage/sns
		// 或者：wxid_v8j939e3g22s22_8d2b -> . (根目录)
		pathParts := strings.Split(cleanName, string(os.PathSeparator))
		if len(pathParts) > 1 {
			// 去掉第一级目录（workdir名称）
			cleanName = filepath.Join(pathParts[1:]...)
		} else if len(pathParts) == 1 && pathParts[0] != "." {
			// 如果只有一个部分且不是根目录，说明这是workdir本身，跳过
			log.Debug().Msgf("Skipping workdir root entry: %s", header.Name)
			continue
		}

		// 构建目标路径
		target := filepath.Join(destDir, cleanName)
		target = filepath.Clean(target)

		// 确保目标路径在destDir内
		destDirClean := filepath.Clean(destDir)
		if !strings.HasPrefix(target, destDirClean+string(os.PathSeparator)) && target != destDirClean {
			log.Warn().Msgf("Skipping path outside destination directory: %s -> %s", header.Name, target)
			continue
		}

		log.Debug().Msgf("Extracting: %s -> %s", header.Name, target)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", target, err)
			}
			extractedCount++
		case tar.TypeReg:
			// 确保父目录存在
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %v", target, err)
			}

			// 创建文件
			outFile, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %v", target, err)
			}

			// 复制文件内容
			_, err = io.Copy(outFile, tarReader)
			outFile.Close()
			if err != nil {
				return fmt.Errorf("failed to write file %s: %v", target, err)
			}

			// 设置文件权限
			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				log.Warn().Err(err).Str("file", target).Msg("Failed to set file permissions")
			}
			extractedCount++
		default:
			log.Debug().Msgf("Skipping unsupported file type %c: %s", header.Typeflag, header.Name)
		}
	}

	log.Info().Msgf("Successfully extracted %d items to %s", extractedCount, destDir)
	return nil
}

// refreshDatabaseAndSession 刷新数据库连接和session缓存
func (s *Service) refreshDatabaseAndSession() error {
	log.Info().Msg("Starting database and session refresh after workdir update")

	// 1. 重启数据库连接以读取新的workdir数据
	log.Info().Msg("Restarting database connection with updated workdir")
	if s.db.GetDB() != nil {
		s.db.Stop()
		log.Debug().Msg("Stopped existing database connection")
	}

	err := s.db.Start()
	if err != nil {
		log.Err(err).Msg("Failed to restart database connection")
		return fmt.Errorf("failed to restart database: %v", err)
	}
	log.Info().Msg("Database connection restarted successfully")

	// 2. 尝试重新初始化数据库（如果之前初始化失败）
	if !s.db.IsInitialized() {
		log.Info().Msg("Attempting to reinitialize database with new workdir")
		if err := s.db.TryInitialize(); err != nil {
			log.Warn().Err(err).Msg("Database reinitialization failed, but continuing with session refresh")
		} else {
			log.Info().Msg("Database reinitialized successfully")
		}
	}

	// 3. 刷新session缓存 (复用Manager.RefreshSession的逻辑)
	log.Info().Msg("Refreshing session cache")
	err = s.refreshSessionCache()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to refresh session cache")
		// 不返回错误，数据库连接已成功
	} else {
		log.Info().Msg("Session cache refreshed successfully")
	}

	log.Info().Msg("Database and session refresh completed")
	return nil
}

// refreshSessionCache 刷新session缓存 (参考Manager.RefreshSession实现)
func (s *Service) refreshSessionCache() error {
	if s.db.GetDB() == nil {
		return fmt.Errorf("database connection is nil")
	}

	resp, err := s.db.GetSessions("", 1, 0)
	if err != nil {
		log.Err(err).Msg("Failed to query sessions for refresh")
		return err
	}

	if len(resp.Items) == 0 {
		log.Info().Msg("No sessions found in database")
		s.ctx.LastSession = time.Time{}
		return nil
	}

	// 更新最后会话时间到context
	s.ctx.LastSession = resp.Items[0].NTime
	log.Info().
		Time("last_session", s.ctx.LastSession).
		Str("session_user", resp.Items[0].UserName).
		Msg("Updated session cache with latest session")

	return nil
}

// formatSize 格式化文件大小
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// removeBackup 删除备份目录
func (s *Service) removeBackup(backupDir string) error {
	log.Debug().Str("backup_dir", backupDir).Msg("Removing backup directory")

	// 检查备份目录是否存在
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		log.Debug().Str("backup_dir", backupDir).Msg("Backup directory does not exist, nothing to remove")
		return nil
	}

	// 删除备份目录及其所有内容
	err := os.RemoveAll(backupDir)
	if err != nil {
		return fmt.Errorf("failed to remove backup directory %s: %v", backupDir, err)
	}

	log.Debug().Str("backup_dir", backupDir).Msg("Backup directory removed successfully")
	return nil
}

// cleanupOldBackups 清理过期的备份文件
func (s *Service) cleanupOldBackups(maxAge time.Duration) error {
	// 备份目录现在在work_dir的上一层
	workDirParent := filepath.Dir(s.ctx.WorkDir)
	backupRoot := filepath.Join(workDirParent, "backup")

	// 检查备份根目录是否存在
	if _, err := os.Stat(backupRoot); os.IsNotExist(err) {
		log.Debug().Str("backup_root", backupRoot).Msg("Backup root directory does not exist")
		return nil
	}

	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %v", err)
	}

	cutoffTime := time.Now().Add(-maxAge)
	removedCount := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// 尝试解析目录名中的时间戳 (格式: 20060102_150405)
		dirName := entry.Name()
		if len(dirName) != 15 || dirName[8] != '_' {
			log.Debug().Str("dir_name", dirName).Msg("Skipping directory with invalid name format")
			continue
		}

		// 解析时间
		timeStr := dirName[:8] + dirName[9:] // 移除下划线
		backupTime, err := time.Parse("20060102150405", timeStr)
		if err != nil {
			log.Debug().Str("dir_name", dirName).Err(err).Msg("Failed to parse backup time")
			continue
		}

		// 如果备份时间早于截止时间，删除它
		if backupTime.Before(cutoffTime) {
			backupPath := filepath.Join(backupRoot, dirName)
			if err := os.RemoveAll(backupPath); err != nil {
				log.Warn().Err(err).Str("backup_path", backupPath).Msg("Failed to remove old backup")
				continue
			}

			log.Info().Str("backup_path", backupPath).Time("backup_time", backupTime).Msg("Removed old backup")
			removedCount++
		}
	}

	if removedCount > 0 {
		log.Info().Int("removed_count", removedCount).Msg("Cleanup completed")
	} else {
		log.Debug().Msg("No old backups to clean up")
	}

	return nil
}
