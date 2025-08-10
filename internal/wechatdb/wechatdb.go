package wechatdb

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sjzar/chatlog/internal/model"
	"github.com/sjzar/chatlog/internal/wechatdb/datasource"
	"github.com/sjzar/chatlog/internal/wechatdb/repository"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	path        string
	platform    string
	version     int
	ds          datasource.DataSource
	repo        *repository.Repository
	initialized bool
}

func New(path string, platform string, version int) (*DB, error) {

	w := &DB{
		path:        path,
		platform:    platform,
		version:     version,
		initialized: false,
	}

	// 尝试初始化，如果失败则记录日志但不返回错误
	if err := w.Initialize(); err != nil {
		log.Warn().Err(err).Str("path", path).Msg("Database initialization failed, will retry when data is available")
	} else {
		w.initialized = true
	}

	return w, nil
}

func (w *DB) Close() error {
	if w.repo != nil {
		return w.repo.Close()
	}
	return nil
}

func (w *DB) Initialize() error {
	var err error
	w.ds, err = datasource.New(w.path, w.platform, w.version)
	if err != nil {
		return err
	}

	w.repo, err = repository.New(w.ds)
	if err != nil {
		return err
	}

	w.initialized = true
	return nil
}

// IsInitialized 检查数据库是否已初始化
func (w *DB) IsInitialized() bool {
	return w.initialized
}

// TryInitialize 尝试初始化数据库，如果失败则返回错误但不影响服务运行
func (w *DB) TryInitialize() error {
	if w.initialized {
		return nil
	}

	err := w.Initialize()
	if err != nil {
		log.Warn().Err(err).Str("path", w.path).Msg("Database initialization failed, will retry later")
		return err
	}

	log.Info().Str("path", w.path).Msg("Database initialized successfully")
	return nil
}

func (w *DB) GetMessages(start, end time.Time, talker string, sender string, keyword string, limit, offset int) ([]*model.Message, error) {
	// 如果未初始化，尝试初始化
	if !w.initialized {
		if err := w.TryInitialize(); err != nil {
			return nil, err
		}
	}

	ctx := context.Background()

	// 使用 repository 获取消息
	messages, err := w.repo.GetMessages(ctx, start, end, talker, sender, keyword, limit, offset)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

type GetContactsResp struct {
	Items []*model.Contact `json:"items"`
}

func (w *DB) GetContacts(key string, limit, offset int) (*GetContactsResp, error) {
	// 如果未初始化，尝试初始化
	if !w.initialized {
		if err := w.TryInitialize(); err != nil {
			return nil, err
		}
	}

	ctx := context.Background()

	contacts, err := w.repo.GetContacts(ctx, key, limit, offset)
	if err != nil {
		return nil, err
	}

	return &GetContactsResp{
		Items: contacts,
	}, nil
}

type GetChatRoomsResp struct {
	Items []*model.ChatRoom `json:"items"`
}

func (w *DB) GetChatRooms(key string, limit, offset int) (*GetChatRoomsResp, error) {
	// 如果未初始化，尝试初始化
	if !w.initialized {
		if err := w.TryInitialize(); err != nil {
			return nil, err
		}
	}

	ctx := context.Background()

	chatRooms, err := w.repo.GetChatRooms(ctx, key, limit, offset)
	if err != nil {
		return nil, err
	}

	return &GetChatRoomsResp{
		Items: chatRooms,
	}, nil
}

type GetSessionsResp struct {
	Items []*model.Session `json:"items"`
}

func (w *DB) GetSessions(key string, limit, offset int) (*GetSessionsResp, error) {
	// 如果未初始化，尝试初始化
	if !w.initialized {
		if err := w.TryInitialize(); err != nil {
			return nil, err
		}
	}

	ctx := context.Background()

	// 使用 repository 获取会话列表
	sessions, err := w.repo.GetSessions(ctx, key, limit, offset)
	if err != nil {
		return nil, err
	}

	return &GetSessionsResp{
		Items: sessions,
	}, nil
}

func (w *DB) GetMedia(_type string, key string) (*model.Media, error) {
	// 如果未初始化，尝试初始化
	if !w.initialized {
		if err := w.TryInitialize(); err != nil {
			return nil, err
		}
	}

	return w.repo.GetMedia(context.Background(), _type, key)
}
