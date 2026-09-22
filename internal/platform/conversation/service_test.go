package conversation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/chat_settings"
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
	if err := db.AutoMigrate(&conversation.Conversation{}, &conversation.Message{}, &chat_settings.ChatSetting{}); err != nil {
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
	db := setupConversationTestDB(t)
	return conversation.NewService(db, conversation.NewRepository(db), chat_settings.NewRepository(db), denyAccessLevel)
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
	db := setupConversationTestDB(t)
	service := conversation.NewService(db, conversation.NewRepository(db), chat_settings.NewRepository(db), allowAccessLevel)
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

// TestGet_AllowsNonOwnerViaAccessToOwnerAsUserResource is the actual "a
// support role can read a regular user's conversations" capability: no
// grant exists directly on the conversation itself, only on its owner as
// a "user" resource (which is how rbac.Service.HasAccessLevel's
// role-cascade — access to a role reaching every user who holds it —
// actually reaches conversations at all, since the cascade only knows
// about resourceType "user").
func TestGet_AllowsNonOwnerViaAccessToOwnerAsUserResource(t *testing.T) {
	check := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		if resourceType == "user" {
			return true, nil // access granted on the owner as a user resource
		}
		return false, nil // nothing granted directly on the conversation itself
	}
	db := setupConversationTestDB(t)
	service := conversation.NewService(db, conversation.NewRepository(db), chat_settings.NewRepository(db), check)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "private", Provider: "gemini", Model: "gemini-2.0-flash",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := service.Get(ctx, 2, conv.ID)
	if err != nil {
		t.Fatalf("expected access to the owner (as a user resource) to be sufficient, got %v", err)
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

// appendPlainMessages is a small helper for ListMessages tests below —
// it appends N distinguishable user messages via AppendHistory, the same
// path SendMessage uses, without needing tool calls/results in the fixture.
func appendPlainMessages(t *testing.T, service conversation.Service, conversationID uint, contents ...string) {
	t.Helper()
	turn := make([]ai_model.Message, 0, len(contents))
	for _, content := range contents {
		turn = append(turn, ai_model.Message{Role: ai_model.RoleUser, Content: content})
	}
	if err := service.AppendHistory(context.Background(), conversationID, turn); err != nil {
		t.Fatalf("AppendHistory failed: %v", err)
	}
}

// TestListMessages_Page1ReturnsMostRecentInChronologicalOrder checks the
// core contract: page 1 is the *last* pageSize messages, but returned
// oldest-to-newest within that page — "newest first" only decides which
// page a message lands on, not the order within one page.
func TestListMessages_Page1ReturnsMostRecentInChronologicalOrder(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "chat", Provider: "gemini", Model: "m",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	appendPlainMessages(t, service, conv.ID, "one", "two", "three", "four", "five")

	page, total, err := service.ListMessages(ctx, 1, conv.ID, shared.Pagination{PageNumber: 1, PageSize: 3})
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total=5, got %d", total)
	}
	if len(page) != 3 {
		t.Fatalf("expected 3 messages on page 1, got %d", len(page))
	}
	got := []string{page[0].Content, page[1].Content, page[2].Content}
	want := []string{"three", "four", "five"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected page 1 = %v (oldest-to-newest within the most recent 3), got %v", want, got)
		}
	}
}

// TestListMessages_Page2ReachesFurtherIntoThePast checks that higher page
// numbers walk backward through history, and that the boundary page
// (fewer than a full pageSize left) still comes back in order.
func TestListMessages_Page2ReachesFurtherIntoThePast(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "chat", Provider: "gemini", Model: "m",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	appendPlainMessages(t, service, conv.ID, "one", "two", "three", "four", "five")

	page, total, err := service.ListMessages(ctx, 1, conv.ID, shared.Pagination{PageNumber: 2, PageSize: 3})
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total=5, got %d", total)
	}
	if len(page) != 2 {
		t.Fatalf("expected 2 messages on the boundary page, got %d", len(page))
	}
	if page[0].Content != "one" || page[1].Content != "two" {
		t.Fatalf("expected page 2 = [one two], got [%s %s]", page[0].Content, page[1].Content)
	}
}

func TestListMessages_DeniesNonOwnerWithNoResourceAccess(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "private", Provider: "gemini", Model: "m",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	appendPlainMessages(t, service, conv.ID, "secret")

	_, _, err = service.ListMessages(ctx, 2, conv.ID, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound for a non-owner with no resource access, got %v", err)
	}
}

func TestListMessages_AllowsNonOwnerWithResourceAccess(t *testing.T) {
	db := setupConversationTestDB(t)
	service := conversation.NewService(db, conversation.NewRepository(db), chat_settings.NewRepository(db), allowAccessLevel)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "shared", Provider: "gemini", Model: "m",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	appendPlainMessages(t, service, conv.ID, "hello")

	page, total, err := service.ListMessages(ctx, 2, conv.ID, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("expected a non-owner with resource-level read access to succeed, got %v", err)
	}
	if total != 1 || len(page) != 1 || page[0].Content != "hello" {
		t.Fatalf("expected the shared message to come back, got total=%d page=%+v", total, page)
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

	items, total, err := service.List(ctx, 1, false, shared.Pagination{PageNumber: 1, PageSize: 10})
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

func TestUpdate_OwnerCanRename(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{Title: "old", Provider: "gemini", Model: "m"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := service.Update(ctx, 1, conv.ID, conversation.UpdateConversationRequest{Title: "new"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Title != "new" {
		t.Fatalf("expected title %q, got %q", "new", updated.Title)
	}
}

func TestUpdate_OwnerCanArchiveAndUnarchive(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{Title: "a", Provider: "gemini", Model: "m"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	archived := true
	updated, err := service.Update(ctx, 1, conv.ID, conversation.UpdateConversationRequest{Archived: &archived})
	if err != nil {
		t.Fatalf("Update (archive) failed: %v", err)
	}
	if !updated.Archived {
		t.Fatal("expected Archived to be true")
	}

	notArchived := false
	updated, err = service.Update(ctx, 1, conv.ID, conversation.UpdateConversationRequest{Archived: &notArchived})
	if err != nil {
		t.Fatalf("Update (unarchive) failed: %v", err)
	}
	if updated.Archived {
		t.Fatal("expected Archived to be false")
	}
}

func TestUpdate_NonOwnerWithoutGrantIsDenied(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{Title: "a", Provider: "gemini", Model: "m"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := service.Update(ctx, 2, conv.ID, conversation.UpdateConversationRequest{Title: "hijacked"}); err != shared.ErrNotFound {
		t.Fatalf("expected ErrNotFound for a non-owner with no grant, got %v", err)
	}
}

func TestDelete_OwnerCanDeleteAndItsMessagesGoToo(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{Title: "a", Provider: "gemini", Model: "m"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := service.AppendHistory(ctx, conv.ID, []ai_model.Message{{Role: ai_model.RoleUser, Content: "hi"}}); err != nil {
		t.Fatalf("AppendHistory failed: %v", err)
	}

	if err := service.Delete(ctx, 1, conv.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := service.Get(ctx, 1, conv.ID); err != shared.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDelete_NonOwnerWithoutGrantIsDenied(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{Title: "a", Provider: "gemini", Model: "m"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := service.Delete(ctx, 2, conv.ID); err != shared.ErrNotFound {
		t.Fatalf("expected ErrNotFound for a non-owner with no grant, got %v", err)
	}

	if _, err := service.Get(ctx, 1, conv.ID); err != nil {
		t.Fatalf("expected the conversation to still exist for its owner, got %v", err)
	}
}

func TestList_ArchivedFilterExcludesTheOtherState(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	active, err := service.Create(ctx, 1, conversation.CreateConversationRequest{Title: "active", Provider: "gemini", Model: "m"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	archivedConv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{Title: "archived", Provider: "gemini", Model: "m"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	archived := true
	if _, err := service.Update(ctx, 1, archivedConv.ID, conversation.UpdateConversationRequest{Archived: &archived}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	items, total, err := service.List(ctx, 1, false, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List(archived=false) failed: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != active.ID {
		t.Fatalf("expected only the active conversation, got total=%d items=%+v", total, items)
	}

	items, total, err = service.List(ctx, 1, true, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List(archived=true) failed: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != archivedConv.ID {
		t.Fatalf("expected only the archived conversation, got total=%d items=%+v", total, items)
	}
}

func TestUpdate_CanSwitchToADifferentBuiltinModel(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "a", Provider: "gemini", Model: "gemini-2.0-flash",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := service.Update(ctx, 1, conv.ID, conversation.UpdateConversationRequest{
		Provider: "anthropic", Model: "claude-sonnet-5",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Provider != "anthropic" || updated.Model != "claude-sonnet-5" {
		t.Fatalf("expected provider/model to switch, got %q/%q", updated.Provider, updated.Model)
	}
}

// TestUpdate_SwitchingAwayFromCustomModelClearsIt is the case a
// independently-patched CustomModelID couldn't express: going from a
// custom preset back to a built-in model has to actually clear the old
// preset id, not just leave it dangling alongside the new provider/model.
func TestUpdate_SwitchingAwayFromCustomModelClearsIt(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	presetID := uint(42)
	conv, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "a", Provider: "openai", Model: "gpt-4o", CustomModelID: &presetID,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if conv.CustomModelID == nil || *conv.CustomModelID != presetID {
		t.Fatalf("expected CustomModelID to be set on create, got %v", conv.CustomModelID)
	}

	updated, err := service.Update(ctx, 1, conv.ID, conversation.UpdateConversationRequest{
		Provider: "gemini", Model: "gemini-2.0-flash",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.CustomModelID != nil {
		t.Fatalf("expected CustomModelID to be cleared, got %v", updated.CustomModelID)
	}
}

type failingChatSettingsRepo struct{}

func (failingChatSettingsRepo) Create(ctx context.Context, s *chat_settings.ChatSetting) error {
	return errors.New("boom")
}

func (failingChatSettingsRepo) FindByConversationID(ctx context.Context, conversationID uint) (*chat_settings.ChatSetting, error) {
	return nil, shared.ErrNotFound
}

func (failingChatSettingsRepo) Update(ctx context.Context, s *chat_settings.ChatSetting) error {
	return nil
}

func (r failingChatSettingsRepo) WithTx(tx *gorm.DB) chat_settings.Repository {
	return r
}

func TestCreate_RollsBackConversationWhenChatSettingsInsertFails(t *testing.T) {
	db := setupConversationTestDB(t)
	service := conversation.NewService(db, conversation.NewRepository(db), failingChatSettingsRepo{}, allowAccessLevel)
	ctx := context.Background()

	_, err := service.Create(ctx, 1, conversation.CreateConversationRequest{
		Title: "x", Provider: "gemini", Model: "m",
	})
	if err == nil {
		t.Fatal("expected Create to fail when the chat_settings insert fails")
	}

	var count int64
	db.Model(&conversation.Conversation{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected the conversation insert to be rolled back too, found %d rows", count)
	}
}
