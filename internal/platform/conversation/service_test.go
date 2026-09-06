package conversation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/conversation"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupConversationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&conversation.Conversation{}, &conversation.Message{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func denyAccessLevel(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return false, nil
}

func allowAccessLevel(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return true, nil
}

func newTestService(t *testing.T) conversation.Service {
	t.Helper()
	return conversation.NewService(conversation.NewRepository(setupConversationTestDB(t)), denyAccessLevel)
}

func TestCreate_ThenGet_SucceedsForOwner(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "first chat", Provider: "gemini", Model: "gemini-2.0-flash",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := service.Get(ctx, 1, conv.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Title != "first chat" {
		t.Fatalf("expected title %q, got %q", "first chat", got.Title)
	}
}

func TestGet_DeniesNonOwnerWithNoResourceAccessAsNotFound(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "private", Provider: "gemini", Model: "gemini-2.0-flash",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = service.Get(ctx, 2, conv.ID)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound for a non-owner with no resource access, got %v", err)
	}
}

func TestGet_AllowsNonOwnerWithResourceAccess(t *testing.T) {
	service := conversation.NewService(conversation.NewRepository(setupConversationTestDB(t)), allowAccessLevel)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "private", Provider: "gemini", Model: "gemini-2.0-flash",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := service.Get(ctx, 2, conv.ID)
	if err != nil {
		t.Fatalf("expected a non-owner with resource-level read access to succeed, got %v", err)
	}
	if got.ID != conv.ID {
		t.Fatalf("expected conversation %d, got %d", conv.ID, got.ID)
	}
}

func TestAppendHistory_ThenLoadHistory_RoundTripsMessagesInOrder(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "chat", Provider: "gemini", Model: "gemini-2.0-flash",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	toolCalls := []ai_model.ToolCall{
		{ID: "call-1", Name: "execute_command", Arguments: map[string]any{"program": "pwd"}},
	}
	toolResults := []ai_model.ToolResult{
		{ToolCallID: "call-1", Name: "execute_command", Result: "/home", Error: errors.New("boom")},
	}

	turn := []ai_model.Message{
		{Role: ai_model.RoleUser, Content: "what's my working directory?"},
		{Role: ai_model.RoleAssistant, ToolCalls: &toolCalls},
		{Role: ai_model.RoleTool, ToolResults: &toolResults},
		{Role: ai_model.RoleAssistant, Content: "here you go"},
	}

	if err := service.AppendHistory(ctx, conv.ID, turn); err != nil {
		t.Fatalf("AppendHistory failed: %v", err)
	}

	loaded, err := service.LoadHistory(ctx, 1, conv.ID)
	if err != nil {
		t.Fatalf("LoadHistory failed: %v", err)
	}

	if len(loaded) != len(turn) {
		t.Fatalf("expected %d messages, got %d", len(turn), len(loaded))
	}

	if loaded[0].Role != ai_model.RoleUser || loaded[0].Content != "what's my working directory?" {
		t.Fatalf("unexpected first message: %+v", loaded[0])
	}

	if loaded[1].ToolCalls == nil || len(*loaded[1].ToolCalls) != 1 || (*loaded[1].ToolCalls)[0].Name != "execute_command" {
		t.Fatalf("expected tool call to round-trip, got %+v", loaded[1])
	}

	if loaded[2].ToolResults == nil || len(*loaded[2].ToolResults) != 1 {
		t.Fatalf("expected one tool result, got %+v", loaded[2])
	}
	result := (*loaded[2].ToolResults)[0]
	if result.Result != "/home" {
		t.Fatalf("expected result %q, got %v", "/home", result.Result)
	}
	if result.Error == nil || result.Error.Error() != "boom" {
		t.Fatalf("expected the tool error message to round-trip, got %v", result.Error)
	}

	if loaded[3].Content != "here you go" {
		t.Fatalf("expected final message content %q, got %q", "here you go", loaded[3].Content)
	}
}

func TestLoadHistory_DeniesNonOwnerWithNoResourceAccess(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "chat", Provider: "gemini", Model: "gemini-2.0-flash",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = service.LoadHistory(ctx, 2, conv.ID)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound for a non-owner, got %v", err)
	}
}

func TestList_OnlyReturnsCallersConversations(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if _, err := service.Create(ctx, 1, conversation.CreateConversationRequest{Title: "a", Provider: "gemini", Model: "m"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := service.Create(ctx, 2, conversation.CreateConversationRequest{Title: "b", Provider: "gemini", Model: "m"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	items, total, err := service.List(ctx, 1, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected exactly 1 conversation for user 1, got total=%d len=%d", total, len(items))
	}
	if items[0].Title != "a" {
		t.Fatalf("expected user 1's conversation %q, got %q", "a", items[0].Title)
	}
}
