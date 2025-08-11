package feishu

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	"github.com/sjzar/chatlog/internal/chatlog/conf"
	"github.com/sjzar/chatlog/internal/model"
)

// Service 飞书同步服务
type Service struct {
	client *lark.Client
	config *conf.FeishuSyncConfig
}

// NewService 创建飞书服务
func NewService(config *conf.FeishuSyncConfig) (*Service, error) {
	if config == nil || config.AppID == "" || config.AppSecret == "" {
		return nil, fmt.Errorf("invalid feishu config")
	}

	client := lark.NewClient(config.AppID, config.AppSecret)
	return &Service{
		client: client,
		config: config,
	}, nil
}

// ExtractDocumentID 从飞书URL中提取文档ID
func (s *Service) ExtractDocumentID(url string) (string, error) {
	// 匹配飞书文档URL格式
	re := regexp.MustCompile(`https://[^/]+\.feishu\.cn/wiki/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("invalid feishu wiki URL: %s", url)
	}
	return matches[1], nil
}

// GetDocumentBlocks 获取文档块列表
func (s *Service) GetDocumentBlocks(documentID string) ([]*larkdocx.Block, error) {
	req := larkdocx.NewListDocumentBlockReqBuilder().
		DocumentId(documentID).
		PageSize(500).
		DocumentRevisionId(-1).
		Build()

	resp, err := s.client.Docx.V1.DocumentBlock.List(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to list document blocks: %w", err)
	}

	if !resp.Success() {
		return nil, fmt.Errorf("failed to list document blocks: %s", resp.CodeError.Error())
	}

	return resp.Data.Items, nil
}

// DeleteDocumentBlocks 删除指定的块
func (s *Service) DeleteDocumentBlocks(documentID string, blockIDs []string) error {
	if len(blockIDs) == 0 {
		return nil
	}

	// 逐个删除块，使用正确的删除方法
	for _, blockID := range blockIDs {
		// 注意：飞书API中，顶级块（如日期标题）通常不能直接删除
		// 我们采用另一种策略：删除该块下的所有子块，然后更新内容

		// 先尝试删除该块下的所有子块
		req := larkdocx.NewBatchDeleteDocumentBlockChildrenReqBuilder().
			DocumentId(documentID).
			BlockId(blockID).
			DocumentRevisionId(-1).
			Body(larkdocx.NewBatchDeleteDocumentBlockChildrenReqBodyBuilder().
				StartIndex(0).
				EndIndex(1). // 删除所有子块
				Build()).
			Build()

		resp, err := s.client.Docx.V1.DocumentBlockChildren.BatchDelete(context.Background(), req)
		if err != nil {
			// 如果删除子块失败，记录警告但继续
			fmt.Printf("Warning: failed to delete children of block %s: %v\n", blockID, err)
			continue
		}

		if !resp.Success() {
			fmt.Printf("Warning: failed to delete children of block %s: %s\n", blockID, resp.CodeError.Error())
			continue
		}

		fmt.Printf("Debug: Successfully deleted children of block %s\n", blockID)
	}

	return nil
}

// CreateTextBlock 创建文本块
func (s *Service) CreateTextBlock(documentID, parentID, content string) error {
	// 清理内容
	cleanContent := s.cleanContent(content)

	// 确保内容不为空
	if cleanContent == "" {
		cleanContent = " " // 使用空格而不是空字符串
	}

	blockType := 2 // 文本块
	align := 1     // 左对齐

	req := larkdocx.NewCreateDocumentBlockChildrenReqBuilder().
		DocumentId(documentID).
		BlockId(parentID).
		DocumentRevisionId(-1).
		Body(larkdocx.NewCreateDocumentBlockChildrenReqBodyBuilder().
			Children([]*larkdocx.Block{
				{
					BlockType: &blockType,
					Text: &larkdocx.Text{
						Elements: []*larkdocx.TextElement{
							{
								TextRun: &larkdocx.TextRun{
									Content: &cleanContent,
									TextElementStyle: &larkdocx.TextElementStyle{
										Bold:          &[]bool{false}[0],
										InlineCode:    &[]bool{false}[0],
										Italic:        &[]bool{false}[0],
										Strikethrough: &[]bool{false}[0],
										Underline:     &[]bool{false}[0],
									},
								},
							},
						},
						Style: &larkdocx.TextStyle{
							Align: &align,
						},
					},
				},
			}).
			Index(0).
			Build()).
		Build()

	resp, err := s.client.Docx.V1.DocumentBlockChildren.Create(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to create text block: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to create text block: %s", resp.CodeError.Error())
	}

	return nil
}

// CreateHeadingBlock 创建标题块
func (s *Service) CreateHeadingBlock(documentID, parentID, content string, level int) (string, error) {
	// 清理内容
	cleanContent := s.cleanContent(content)

	var blockType int32
	switch level {
	case 1:
		blockType = 3 // heading1
	case 2:
		blockType = 4 // heading2
	case 3:
		blockType = 5 // heading3
	default:
		blockType = 4 // 默认使用heading2
	}

	align := 1 // 左对齐

	// 如果parentID为空，使用documentID作为父级
	if parentID == "" {
		parentID = documentID
	}

	req := larkdocx.NewCreateDocumentBlockChildrenReqBuilder().
		DocumentId(documentID).
		BlockId(parentID).
		DocumentRevisionId(-1).
		Body(larkdocx.NewCreateDocumentBlockChildrenReqBodyBuilder().
			Children([]*larkdocx.Block{
				{
					BlockType: &[]int{int(blockType)}[0],
					Heading2: &larkdocx.Text{
						Elements: []*larkdocx.TextElement{
							{
								TextRun: &larkdocx.TextRun{
									Content: &cleanContent,
									TextElementStyle: &larkdocx.TextElementStyle{
										Bold:          &[]bool{false}[0],
										InlineCode:    &[]bool{false}[0],
										Italic:        &[]bool{false}[0],
										Strikethrough: &[]bool{false}[0],
										Underline:     &[]bool{false}[0],
									},
								},
							},
						},
						Style: &larkdocx.TextStyle{
							Align: &align,
						},
					},
				},
			}).
			Index(0).
			Build()).
		Build()

	resp, err := s.client.Docx.V1.DocumentBlockChildren.Create(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to create heading block: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("failed to create heading block: %s", resp.CodeError.Error())
	}

	// 返回新创建的块ID
	if len(resp.Data.Children) > 0 && resp.Data.Children[0].BlockId != nil {
		return *resp.Data.Children[0].BlockId, nil
	}
	return "", nil
}

// CreateMultipleBlocks 批量创建多个块
func (s *Service) CreateMultipleBlocks(documentID, parentID string, contents []string) error {
	if len(contents) == 0 {
		return nil
	}

	// 构建所有要创建的块
	var blocks []*larkdocx.Block
	blockType := 2 // 文本块
	align := 1     // 左对齐

	for _, content := range contents {
		cleanContent := s.cleanContent(content)
		// 确保内容不为空
		if cleanContent == "" {
			cleanContent = " " // 使用空格而不是空字符串
		}

		blocks = append(blocks, &larkdocx.Block{
			BlockType: &blockType,
			Text: &larkdocx.Text{
				Elements: []*larkdocx.TextElement{
					{
						TextRun: &larkdocx.TextRun{
							Content: &cleanContent,
							TextElementStyle: &larkdocx.TextElementStyle{
								Bold:          &[]bool{false}[0],
								InlineCode:    &[]bool{false}[0],
								Italic:        &[]bool{false}[0],
								Strikethrough: &[]bool{false}[0],
								Underline:     &[]bool{false}[0],
							},
						},
					},
				},
				Style: &larkdocx.TextStyle{
					Align: &align,
				},
			},
		})
	}

	// 如果只有一个块，使用单个创建方法
	if len(blocks) == 1 {
		return s.CreateTextBlock(documentID, parentID, contents[0])
	}

	// 一次性创建所有块
	req := larkdocx.NewCreateDocumentBlockChildrenReqBuilder().
		DocumentId(documentID).
		BlockId(parentID).
		DocumentRevisionId(-1).
		Body(larkdocx.NewCreateDocumentBlockChildrenReqBodyBuilder().
			Children(blocks).
			Index(0).
			Build()).
		Build()

	resp, err := s.client.Docx.V1.DocumentBlockChildren.Create(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to create multiple blocks: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to create multiple blocks: %s", resp.CodeError.Error())
	}

	return nil
}

// CreateTextBlockWithContent 创建包含完整内容的文本块
func (s *Service) CreateTextBlockWithContent(documentID, parentID string, content string) error {
	// 清理内容
	cleanContent := s.cleanContent(content)

	// 确保内容不为空
	if cleanContent == "" {
		cleanContent = " " // 使用空格而不是空字符串
	}

	blockType := 2 // 文本块
	align := 1     // 左对齐

	req := larkdocx.NewCreateDocumentBlockChildrenReqBuilder().
		DocumentId(documentID).
		BlockId(parentID).
		DocumentRevisionId(-1).
		Body(larkdocx.NewCreateDocumentBlockChildrenReqBodyBuilder().
			Children([]*larkdocx.Block{
				{
					BlockType: &blockType,
					Text: &larkdocx.Text{
						Elements: []*larkdocx.TextElement{
							{
								TextRun: &larkdocx.TextRun{
									Content: &cleanContent,
									TextElementStyle: &larkdocx.TextElementStyle{
										Bold:          &[]bool{false}[0],
										InlineCode:    &[]bool{false}[0],
										Italic:        &[]bool{false}[0],
										Strikethrough: &[]bool{false}[0],
										Underline:     &[]bool{false}[0],
									},
								},
							},
						},
						Style: &larkdocx.TextStyle{
							Align: &align,
						},
					},
				},
			}).
			Index(0).
			Build()).
		Build()

	resp, err := s.client.Docx.V1.DocumentBlockChildren.Create(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to create text block: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to create text block: %s", resp.CodeError.Error())
	}

	return nil
}

// CreateQuoteContainer 创建引用容器块
func (s *Service) CreateQuoteContainer(documentID, parentID string) (string, error) {
	blockType := 34 // 引用容器块
	req := larkdocx.NewCreateDocumentBlockChildrenReqBuilder().
		DocumentId(documentID).
		BlockId(parentID).
		DocumentRevisionId(-1).
		Body(larkdocx.NewCreateDocumentBlockChildrenReqBodyBuilder().
			Children([]*larkdocx.Block{
				{
					BlockType:      &blockType,
					QuoteContainer: &larkdocx.QuoteContainer{},
				},
			}).
			Index(0).
			Build()).
		Build()

	resp, err := s.client.Docx.V1.DocumentBlockChildren.Create(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to create quote container: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("failed to create quote container: %s", resp.CodeError.Error())
	}

	// 返回创建的引用容器块ID
	if resp.Data != nil && len(resp.Data.Children) > 0 && resp.Data.Children[0].BlockId != nil {
		return *resp.Data.Children[0].BlockId, nil
	}

	return "", fmt.Errorf("failed to get quote container block ID")
}

// CreateTextBlockInQuote 在引用容器内创建文本块
func (s *Service) CreateTextBlockInQuote(documentID, quoteContainerID, content string) error {
	// 清理内容
	cleanContent := s.cleanContent(content)

	// 确保内容不为空
	if cleanContent == "" {
		cleanContent = " " // 使用空格而不是空字符串
	}

	blockType := 2 // 文本块
	align := 1     // 左对齐

	req := larkdocx.NewCreateDocumentBlockChildrenReqBuilder().
		DocumentId(documentID).
		BlockId(quoteContainerID).
		DocumentRevisionId(-1).
		Body(larkdocx.NewCreateDocumentBlockChildrenReqBodyBuilder().
			Children([]*larkdocx.Block{
				{
					BlockType: &blockType,
					Text: &larkdocx.Text{
						Elements: []*larkdocx.TextElement{
							{
								TextRun: &larkdocx.TextRun{
									Content: &cleanContent,
									TextElementStyle: &larkdocx.TextElementStyle{
										Bold:          &[]bool{false}[0],
										InlineCode:    &[]bool{false}[0],
										Italic:        &[]bool{false}[0],
										Strikethrough: &[]bool{false}[0],
										Underline:     &[]bool{false}[0],
									},
								},
							},
						},
						Style: &larkdocx.TextStyle{
							Align: &align,
						},
					},
				},
			}).
			Index(0).
			Build()).
		Build()

	resp, err := s.client.Docx.V1.DocumentBlockChildren.Create(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to create text block in quote: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to create text block in quote: %s", resp.CodeError.Error())
	}

	return nil
}

// SyncChatlog 同步聊天记录到飞书文档
func (s *Service) SyncChatlog(syncItem *conf.FeishuSyncMapItem, messages []*model.Message) error {
	// 提取文档ID
	documentID, err := s.ExtractDocumentID(syncItem.TargetURL)
	if err != nil {
		return fmt.Errorf("failed to extract document ID: %w", err)
	}

	// 获取现有文档块
	blocks, err := s.GetDocumentBlocks(documentID)
	if err != nil {
		return fmt.Errorf("failed to get document blocks: %w", err)
	}

	// 按日期分组消息
	dateGroups := s.groupMessagesByDate(messages)

	// 查找现有的日期块
	existingDateBlocks := make(map[string]*larkdocx.Block)
	for _, block := range blocks {
		// 调试信息：打印每个块的类型和内容
		if block.BlockType != nil {
			fmt.Printf("Debug: Found block type %d, ID: %s\n", *block.BlockType, *block.BlockId)
		}

		// 二级标题的block_type是4，不是2
		if block.BlockType != nil && *block.BlockType == 4 && block.Heading2 != nil && block.Heading2.Elements != nil {
			for _, element := range block.Heading2.Elements {
				if element.TextRun != nil && element.TextRun.Content != nil {
					content := *element.TextRun.Content
					fmt.Printf("Debug: Found heading2 content: '%s'\n", content)
					if s.isDateContent(content) {
						fmt.Printf("Debug: Found date block: '%s' with ID: %s\n", content, *block.BlockId)
						existingDateBlocks[content] = block
						break
					}
				}
			}
		}
	}

	fmt.Printf("Debug: Found %d existing date blocks\n", len(existingDateBlocks))
	for date, block := range existingDateBlocks {
		fmt.Printf("Debug: Date '%s' -> Block ID: %s\n", date, *block.BlockId)
	}

	// 同步每个日期的消息
	for date, msgs := range dateGroups {
		var dateBlockID string
		var err error

		// 如果该日期已存在，先尝试删除其子块
		if existingBlock, exists := existingDateBlocks[date]; exists {
			fmt.Printf("Debug: Found existing date block for %s, ID: %s\n", date, *existingBlock.BlockId)

			// 删除该日期块下的所有子块
			if err := s.DeleteDocumentBlocks(documentID, []string{*existingBlock.BlockId}); err != nil {
				fmt.Printf("Warning: failed to delete existing date block %s: %v\n", date, err)
			}

			// 使用现有的日期块ID，而不是创建新的
			dateBlockID = *existingBlock.BlockId
			fmt.Printf("Debug: Reusing existing date block ID: %s\n", dateBlockID)
		} else {
			// 创建新的日期标题块
			fmt.Printf("Debug: Creating new date block for %s\n", date)
			dateBlockID, err = s.CreateHeadingBlock(documentID, documentID, date, 2)
			if err != nil {
				return fmt.Errorf("failed to create date heading block for %s: %w", date, err)
			}
			fmt.Printf("Debug: Created new date block ID: %s\n", dateBlockID)
		}

		// 创建引用容器块
		fmt.Printf("Debug: Creating quote container for date %s\n", date)
		quoteContainerID, err := s.CreateQuoteContainer(documentID, dateBlockID)
		if err != nil {
			return fmt.Errorf("failed to create quote container for %s: %w", date, err)
		}
		fmt.Printf("Debug: Created quote container ID: %s\n", quoteContainerID)

		// 将所有消息内容合并为一个文本块
		var allContent strings.Builder
		for _, msg := range msgs {
			// 使用现有的消息格式化逻辑，与GetChatlog保持一致
			content := msg.PlainText(false, "2006-01-02 15:04:05", "")
			allContent.WriteString(content)
			allContent.WriteString("\n") // 每条消息后添加换行
		}

		// 在引用容器内创建包含所有消息的文本块
		if err := s.CreateTextBlockInQuote(documentID, quoteContainerID, allContent.String()); err != nil {
			return fmt.Errorf("failed to create message block in quote: %w", err)
		}

		fmt.Printf("Debug: Successfully synced date %s with %d messages\n", date, len(msgs))
	}

	return nil
}

// groupMessagesByDate 按日期分组消息
func (s *Service) groupMessagesByDate(messages []*model.Message) map[string][]*model.Message {
	groups := make(map[string][]*model.Message)

	for _, msg := range messages {
		date := msg.Time.Format("0102") // MM-DD格式
		if groups[date] == nil {
			groups[date] = make([]*model.Message, 0)
		}
		groups[date] = append(groups[date], msg)
	}

	return groups
}

// findOrCreateDateBlocks 查找或创建日期标题块
func (s *Service) findOrCreateDateBlocks(blocks []*larkdocx.Block, documentID string, dateGroups map[string][]*model.Message) map[string]*larkdocx.Block {
	dateBlockMap := make(map[string]*larkdocx.Block)

	// 查找现有的日期块
	for _, block := range blocks {
		if block.BlockType != nil && *block.BlockType == 4 && block.Heading2 != nil { // heading2类型
			if len(block.Heading2.Elements) > 0 && block.Heading2.Elements[0].TextRun != nil {
				content := block.Heading2.Elements[0].TextRun.Content
				if content != nil {
					// 检查是否是日期格式 (MMDD)
					if len(*content) == 4 && s.isDateContent(*content) {
						dateBlockMap[*content] = block
					}
				}
			}
		}
	}

	// 为缺失的日期创建新块
	for date := range dateGroups {
		if _, exists := dateBlockMap[date]; !exists {
			// 创建新的日期标题块
			blockID, err := s.CreateHeadingBlock(documentID, "", date, 2)
			if err == nil && blockID != "" {
				// 创建一个临时块来存储ID
				tempBlock := &larkdocx.Block{BlockId: &blockID}
				dateBlockMap[date] = tempBlock
			}
		}
	}

	return dateBlockMap
}

// isDateContent 检查内容是否是日期格式
func (s *Service) isDateContent(content string) bool {
	if len(content) != 4 {
		return false
	}

	// 检查是否是数字
	for _, char := range content {
		if char < '0' || char > '9' {
			return false
		}
	}

	// 检查月份和日期是否合理
	month := int(content[0]-'0')*10 + int(content[1]-'0')
	day := int(content[2]-'0')*10 + int(content[3]-'0')

	return month >= 1 && month <= 12 && day >= 1 && day <= 31
}

// findExistingMessageBlocks 查找现有消息块
func (s *Service) findExistingMessageBlocks(blocks []*larkdocx.Block, parentID string) []*larkdocx.Block {
	var messageBlocks []*larkdocx.Block

	for _, block := range blocks {
		if block.ParentId != nil && *block.ParentId == parentID && block.BlockType != nil && *block.BlockType == 2 { // 文本块
			messageBlocks = append(messageBlocks, block)
		}
	}

	return messageBlocks
}

// cleanContent 清理内容，确保适合飞书API
func (s *Service) cleanContent(content string) string {
	// 处理Markdown格式的图片链接
	// 将 ![图片](http://...) 转换为 [图片]
	return content
	re := regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	content = re.ReplaceAllString(content, "[$1]")

	// 处理其他Markdown链接格式
	// 将 [链接|标题](url) 转换为 [链接|标题]
	re2 := regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	content = re2.ReplaceAllString(content, "[$1]")

	// 替换换行符为空格，避免破坏JSON格式
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\r", " ")

	// 移除或替换可能导致问题的特殊字符
	content = strings.ReplaceAll(content, "\t", " ")
	content = strings.ReplaceAll(content, "\"", "\"")
	content = strings.ReplaceAll(content, "\\", "\\\\")

	// 移除控制字符
	var cleaned strings.Builder
	for _, r := range content {
		if r >= 32 || r == '\n' || r == '\r' || r == '\t' {
			cleaned.WriteRune(r)
		} else {
			cleaned.WriteRune(' ')
		}
	}

	// 限制长度，避免内容过长
	result := cleaned.String()
	if len(result) > 10000 {
		result = result[:10000] + "..."
	}

	return result
}
