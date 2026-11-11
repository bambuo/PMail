package infra

import (
	"strconv"
	"testing"

	"github.com/Jinnrry/pmail/consts"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"xorm.io/xorm"
)

func newDeletionTestStore(t *testing.T) *xorm.Engine {
	t.Helper()
	engine, err := xorm.NewEngine("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	engine.SetMaxOpenConns(1)
	if err = engine.Sync2(&models.Email{}, &models.UserEmail{}); err != nil {
		t.Fatal(err)
	}
	previousDB, previousSession := Database, app.OpenEmailDeletionSession
	Database = engine
	app.OpenEmailDeletionSession = NewEmailDeletionSession
	t.Cleanup(func() { Database = previousDB; app.OpenEmailDeletionSession = previousSession; engine.Close() })
	return engine
}

func addDeletionTestMail(t *testing.T, engine *xorm.Engine, owners ...int) int {
	t.Helper()
	email := models.Email{Subject: "retained content"}
	if _, err := engine.Insert(&email); err != nil {
		t.Fatal(err)
	}
	for _, owner := range owners {
		if _, err := engine.Insert(&models.UserEmail{EmailID: email.Id, UserID: owner}); err != nil {
			t.Fatal(err)
		}
	}
	return email.Id
}

func TestEmailDeletionRollsBackOnLaterFailure(t *testing.T) {
	engine := newDeletionTestStore(t)
	first, second := addDeletionTestMail(t, engine, 1), addDeletionTestMail(t, engine, 1)
	if _, err := engine.Exec("CREATE TRIGGER reject_second_owner_delete BEFORE DELETE ON user_email WHEN OLD.email_id=" + strconv.Itoa(second) + " BEGIN SELECT RAISE(ABORT, 'injected deletion failure'); END"); err != nil {
		t.Fatal(err)
	}
	if err := app.EmailDeleteDelEmail(&context.Context{UserID: 1}, []int{first, second}, true); err == nil {
		t.Fatal("expected the later deletion failure")
	}
	for _, id := range []int{first, second} {
		count, err := engine.Table(&models.Email{}).Where("id=?", id).Count()
		if err != nil || count != 1 {
			t.Fatalf("email %d after rollback: count=%d err=%v", id, count, err)
		}
		count, err = engine.Table(&models.UserEmail{}).Where("email_id=? and user_id=1", id).Count()
		if err != nil || count != 1 {
			t.Fatalf("ownership %d after rollback: count=%d err=%v", id, count, err)
		}
	}
}

func TestEmailDeletionPreservesSharedContent(t *testing.T) {
	engine := newDeletionTestStore(t)
	id := addDeletionTestMail(t, engine, 1, 2)
	if err := app.EmailDeleteDelEmail(&context.Context{UserID: 1}, []int{id}, true); err != nil {
		t.Fatal(err)
	}
	var email models.Email
	found, err := engine.ID(id).Get(&email)
	if err != nil || !found || email.Subject != "retained content" {
		t.Fatalf("shared mail lost: found=%t subject=%q err=%v", found, email.Subject, err)
	}
	var owners []models.UserEmail
	if err = engine.Where("email_id=?", id).Find(&owners); err != nil {
		t.Fatal(err)
	}
	if len(owners) != 1 || owners[0].UserID != 2 {
		t.Fatalf("remaining owners=%+v, want only user 2", owners)
	}
	if err = app.EmailDeleteDelEmail(&context.Context{UserID: 2}, []int{id}, true); err != nil {
		t.Fatal(err)
	}
	count, err := engine.Table(&models.Email{}).Where("id=?", id).Count()
	if err != nil || count != 0 {
		t.Fatalf("last-owner removal left count=%d err=%v", count, err)
	}
}

func TestEmailSoftDeleteRetainsContent(t *testing.T) {
	engine := newDeletionTestStore(t)
	id := addDeletionTestMail(t, engine, 1, 2)
	if _, err := engine.Table(&models.UserEmail{}).Where("email_id=? and user_id=1", id).Update(map[string]any{"group_id": 42}); err != nil {
		t.Fatal(err)
	}
	if err := app.EmailDeleteDelEmail(&context.Context{UserID: 1}, []int{id}, false); err != nil {
		t.Fatal(err)
	}
	var relation models.UserEmail
	found, err := engine.Where("email_id=? and user_id=1", id).Get(&relation)
	if err != nil || !found || relation.Status != consts.EmailStatusDel || relation.GroupId != 0 {
		t.Fatalf("soft-deleted relation=%+v found=%t err=%v", relation, found, err)
	}
	count, err := engine.Table(&models.Email{}).Where("id=?", id).Count()
	if err != nil || count != 1 {
		t.Fatalf("soft deletion removed content: count=%d err=%v", count, err)
	}
	var other models.UserEmail
	found, err = engine.Where("email_id=? and user_id=2", id).Get(&other)
	if err != nil || !found || other.Status != 0 {
		t.Fatalf("other owner's state changed: %+v err=%v", other, err)
	}
}
