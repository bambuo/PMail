package infra

import (
	"testing"

	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/pkg/context"
)

func TestMailboxQueriesBindVariadicArguments(t *testing.T) {
	engine := newDeletionTestStore(t)
	id := addDeletionTestMail(t, engine, 1)
	ctx := &context.Context{UserID: 1}
	tag := dto.SearchTag{Type: -1, Status: -1, GroupId: -1}
	emails, total := (PersistenceRepository{}).QueryEmailList(ctx, tag, "retained", false, 0, 10)
	if len(emails) != 1 || total != 1 || emails[0].Id != id || emails[0].Subject != "retained content" {
		t.Fatalf("multi-parameter list: emails=%+v total=%d, want saved email %d", emails, total, id)
	}
	uids := (PersistenceRepository{}).EmailListGetUEListByUID(ctx, "INBOX", 0, 0, nil)
	if len(uids) != 1 || uids[0].EmailID != id {
		t.Fatalf("multi-parameter UID list=%+v, want email %d", uids, id)
	}
	other := &context.Context{UserID: 2}
	emails, total = (PersistenceRepository{}).QueryEmailList(other, tag, "retained", false, 0, 10)
	if len(emails) != 0 || total != 0 {
		t.Fatalf("query leaked another user's mail: emails=%+v total=%d", emails, total)
	}
	if uids := (PersistenceRepository{}).EmailListGetUEListByUID(other, "INBOX", 0, 0, nil); len(uids) != 0 {
		t.Fatalf("UID query leaked another user's mail: %+v", uids)
	}
}
