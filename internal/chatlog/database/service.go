package database

import (
	"strings"
	"time"

	"github.com/sjzar/chatlog/internal/chatlog/ctx"
	"github.com/sjzar/chatlog/internal/errors"
	"github.com/sjzar/chatlog/internal/model"
	"github.com/sjzar/chatlog/internal/wechatdb"
)

type Service struct {
	ctx *ctx.Context
	db  *wechatdb.DB
}

func NewService(ctx *ctx.Context) *Service {
	return &Service{
		ctx: ctx,
	}
}

func (s *Service) Start() error {
	db, err := wechatdb.New(s.ctx.WorkDir, s.ctx.Platform, s.ctx.Version)
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

func (s *Service) Stop() error {
	if s.db != nil {
		s.db.Close()
	}
	s.db = nil
	return nil
}

func (s *Service) GetDB() *wechatdb.DB {
	return s.db
}

// IsInitialized 检查数据库是否已初始化
func (s *Service) IsInitialized() bool {
	if s.db == nil {
		return false
	}
	return s.db.IsInitialized()
}

// TryInitialize 尝试初始化数据库
func (s *Service) TryInitialize() error {
	if s.db == nil {
		return errors.New(nil, 500, "database service not started")
	}
	return s.db.TryInitialize()
}

// 查询关键词预过滤，如果不合法，抛出： KeywordFiltered
func (s *Service) KeywordPreFilter(keyword string) error {
	return nil
	// 检查是否包含必须的关键字
	for _, k := range s.ctx.MustContainKeywords {
		if strings.Contains(keyword, k) {
			return nil
		}
	}
	return errors.KeywordFiltered(keyword)
}

func (s *Service) GetMessages(start, end time.Time, talker string, sender string, keyword string, limit, offset int) ([]*model.Message, error) {
	err := s.KeywordPreFilter(talker)
	if err != nil {
		return nil, err
	}
	return s.db.GetMessages(start, end, talker, sender, keyword, limit, offset)
}

func (s *Service) GetContacts(key string, limit, offset int) (*wechatdb.GetContactsResp, error) {
	err := s.KeywordPreFilter(key)
	if err != nil {
		return nil, err
	}
	return s.db.GetContacts(key, limit, offset)
}

func (s *Service) GetChatRooms(key string, limit, offset int) (*wechatdb.GetChatRoomsResp, error) {
	err := s.KeywordPreFilter(key)
	if err != nil {
		return nil, err
	}
	return s.db.GetChatRooms(key, limit, offset)
}

// GetSession retrieves session information
func (s *Service) GetSessions(key string, limit, offset int) (*wechatdb.GetSessionsResp, error) {
	return s.db.GetSessions(key, limit, offset)
}

func (s *Service) GetMedia(_type string, key string) (*model.Media, error) {
	return s.db.GetMedia(_type, key)
}

// Close closes the database connection
func (s *Service) Close() {
	// Add cleanup code if needed
	s.db.Close()
}
