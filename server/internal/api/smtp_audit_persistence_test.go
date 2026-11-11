package api

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/consts"
	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/hooks/contract"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/infra"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	smtp "github.com/emersion/go-smtp"
	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
	"xorm.io/xorm"
)

func TestOutgoingAuditInsertFailureDoesNotDeliver(t *testing.T) {
	engine, dsn := smtpNewAuditTestEngine(t)
	blocker := smtpBeginAuditReadTransaction(t, dsn)

	var deliveries atomic.Int32
	session := smtpNewAuditTestSession()
	err := session.deliverOutgoingEmail(smtpNewAuditTestEmail("insert-busy@example.com"), func(*context.Context, *parsemail.Email) (error, map[string]error) {
		deliveries.Add(1)
		return nil, nil
	})

	smtpAssertSMTPCode(t, err, 451)
	if got := deliveries.Load(); got != 0 {
		t.Fatalf("downstream deliveries = %d, want 0", got)
	}
	smtpAssertAuditCounts(t, engine, 0, 0)

	if err := blocker.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := session.deliverOutgoingEmail(smtpNewAuditTestEmail("insert-busy@example.com"), func(*context.Context, *parsemail.Email) (error, map[string]error) {
		deliveries.Add(1)
		return nil, nil
	}); err != nil {
		t.Fatalf("delivery after lock release failed: %v", err)
	}
	if got := deliveries.Load(); got != 1 {
		t.Fatalf("downstream deliveries after recovery = %d, want 1", got)
	}
	smtpAssertAuditCounts(t, engine, 1, 1)
	smtpAssertAuditStatuses(t, engine, consts.EmailStatusSent)
}

func TestOutgoingAuditRetriesBusyInitialInsertBeforeDeliver(t *testing.T) {
	engine, dsn := smtpNewAuditTestEngine(t)
	blocker := smtpBeginAuditReadTransaction(t, dsn)
	released := make(chan struct{})
	go func() {
		time.Sleep(35 * time.Millisecond)
		_ = blocker.Rollback()
		close(released)
	}()

	var deliveries atomic.Int32
	err := smtpNewAuditTestSession().deliverOutgoingEmail(smtpNewAuditTestEmail("initial-retry@example.com"), func(*context.Context, *parsemail.Email) (error, map[string]error) {
		deliveries.Add(1)
		return nil, nil
	})
	<-released
	if err != nil {
		t.Fatalf("deliverOutgoingEmail() error = %v", err)
	}
	if got := deliveries.Load(); got != 1 {
		t.Fatalf("downstream deliveries = %d, want 1", got)
	}
	smtpAssertAuditCounts(t, engine, 1, 1)
	smtpAssertAuditStatuses(t, engine, consts.EmailStatusSent)
}

func TestOutgoingAuditRelationInsertRollsBackEmail(t *testing.T) {
	engine, _ := smtpNewAuditTestEngine(t)
	if _, err := engine.Exec(`CREATE TRIGGER fail_user_email_insert
		BEFORE INSERT ON user_email
		BEGIN
			SELECT RAISE(ABORT, 'forced user_email insert failure');
		END`); err != nil {
		t.Fatal(err)
	}

	email := smtpNewAuditTestEmail("relation-failure@example.com")
	_, _, err := smtpSaveEmail(smtpNewAuditTestSession().Ctx, email.Size, email, 1, int(consts.EmailTypeSend), nil, true, true)
	if err == nil {
		t.Fatal("saveEmail() error = nil, want relation insert failure")
	}
	if email.MessageId != 0 {
		t.Fatalf("email.MessageId = %d, want 0 after rollback", email.MessageId)
	}
	smtpAssertAuditCounts(t, engine, 0, 0)
}

func TestIncomingAuditRelationInsertRollsBackEmail(t *testing.T) {
	engine, _ := smtpNewAuditTestEngine(t)
	recipient := &models.User{Account: "recipient", Name: "Recipient"}
	if _, err := engine.Insert(recipient); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exec(`CREATE TRIGGER fail_incoming_user_email_insert
		BEFORE INSERT ON user_email
		BEGIN
			SELECT RAISE(ABORT, 'forced incoming user_email insert failure');
		END`); err != nil {
		t.Fatal(err)
	}

	from := &parsemail.User{Name: "External", EmailAddress: "external@example.net"}
	email := &parsemail.Email{
		From:   from,
		Sender: from,
		To:     []*parsemail.User{{EmailAddress: "recipient@example.com"}},
		Text:   []byte("body"),
		Size:   4,
		MsgID:  "incoming-relation-failure@example.net",
	}
	_, _, err := smtpSaveEmail(&context.Context{}, email.Size, email, 0, int(consts.EmailTypeReceive), []string{"recipient@example.com"}, true, true)
	if err == nil {
		t.Fatal("saveEmail() error = nil, want incoming relation insert failure")
	}
	smtpAssertAuditCounts(t, engine, 0, 0)
}

func TestOutgoingAuditRetriesBusyFinalUpdateAtomically(t *testing.T) {
	engine, dsn := smtpNewAuditTestEngine(t)
	var blocker *sql.Tx

	session := smtpNewAuditTestSession()
	err := session.deliverOutgoingEmail(smtpNewAuditTestEmail("final-retry@example.com"), func(*context.Context, *parsemail.Email) (error, map[string]error) {
		blocker = smtpBeginAuditReadTransaction(t, dsn)
		go func() {
			time.Sleep(35 * time.Millisecond)
			_ = blocker.Rollback()
		}()
		return nil, nil
	})
	if err != nil {
		t.Fatalf("deliverOutgoingEmail() error = %v", err)
	}
	smtpAssertAuditStatuses(t, engine, consts.EmailStatusSent)
	smtpAssertAuditError(t, engine, "", false)
}

func TestOutgoingAuditFinalUpdateIsAtomic(t *testing.T) {
	engine, _ := smtpNewAuditTestEngine(t)
	if _, err := engine.Exec(fmt.Sprintf(`CREATE TRIGGER fail_user_email_success
		BEFORE UPDATE OF status ON user_email
		WHEN NEW.status = %d
		BEGIN
			SELECT RAISE(ABORT, 'forced user_email update failure');
		END`, consts.EmailStatusSent)); err != nil {
		t.Fatal(err)
	}

	session := smtpNewAuditTestSession()
	err := session.deliverOutgoingEmail(smtpNewAuditTestEmail("atomic-update@example.com"), smtpSuccessfulAuditSender)
	if err != nil {
		t.Fatalf("accepted delivery must remain successful, got %v", err)
	}
	smtpAssertAuditStatuses(t, engine, consts.EmailStatusDeliveryPending)
	smtpAssertAuditError(t, engine, smtpDeliveryPendingError, true)
}

func TestAcceptedDeliveryWithPermanentAuditLockReturnsSuccessAndAlerts(t *testing.T) {
	engine, dsn := smtpNewAuditTestEngine(t)
	var blocker *sql.Tx
	var deliveries atomic.Int32

	oldOutput := log.StandardLogger().Out
	oldLevel := log.GetLevel()
	var logs bytes.Buffer
	log.SetOutput(&logs)
	log.SetLevel(log.ErrorLevel)
	t.Cleanup(func() {
		log.SetOutput(oldOutput)
		log.SetLevel(oldLevel)
	})

	err := smtpNewAuditTestSession().deliverOutgoingEmail(smtpNewAuditTestEmail("accepted-unknown@example.com"), func(*context.Context, *parsemail.Email) (error, map[string]error) {
		deliveries.Add(1)
		blocker = smtpBeginAuditReadTransaction(t, dsn)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("accepted delivery must remain successful, got %v", err)
	}
	if got := deliveries.Load(); got != 1 {
		t.Fatalf("downstream deliveries = %d, want 1", got)
	}
	if !strings.Contains(logs.String(), "audit_event=delivery_result_persist_failed") {
		t.Fatalf("missing CRITICAL persistence alert in logs: %s", logs.String())
	}
	if err = blocker.Rollback(); err != nil {
		t.Fatal(err)
	}
	smtpAssertAuditStatuses(t, engine, consts.EmailStatusDeliveryPending)
	smtpAssertAuditError(t, engine, smtpDeliveryPendingError, true)
}

func TestTemporaryDeliveryFailureWithUnpersistedResultCanRetryAsNewAttempt(t *testing.T) {
	engine, dsn := smtpNewAuditTestEngine(t)
	var blocker *sql.Tx
	var deliveries atomic.Int32
	session := smtpNewAuditTestSession()
	messageID := "temporary-unpersisted@example.com"

	err := session.deliverOutgoingEmail(smtpNewAuditTestEmail(messageID), func(*context.Context, *parsemail.Email) (error, map[string]error) {
		deliveries.Add(1)
		blocker = smtpBeginAuditReadTransaction(t, dsn)
		return errors.New("delivery failed"), map[string]error{
			"example.net": &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
		}
	})
	smtpAssertSMTPCode(t, err, 451)
	if err = blocker.Rollback(); err != nil {
		t.Fatal(err)
	}

	if err = session.deliverOutgoingEmail(smtpNewAuditTestEmail(messageID), func(*context.Context, *parsemail.Email) (error, map[string]error) {
		deliveries.Add(1)
		return nil, nil
	}); err != nil {
		t.Fatalf("retry after temporary failure: %v", err)
	}
	if got := deliveries.Load(); got != 2 {
		t.Fatalf("downstream deliveries = %d, want 2", got)
	}
	smtpAssertAuditCounts(t, engine, 2, 2)
	smtpAssertMessageIDCount(t, engine, messageID, 2)
}

func TestOutgoingAuditPersistsDeliveryFailure(t *testing.T) {
	tests := []struct {
		name      string
		domainErr error
		wantCode  int
	}{
		{
			name:      "temporary",
			domainErr: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			wantCode:  451,
		},
		{
			name:      "permanent",
			domainErr: &textproto.Error{Code: 550, Msg: "mailbox unavailable"},
			wantCode:  550,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, _ := smtpNewAuditTestEngine(t)
			deliveryErr := errors.New("delivery failed")
			err := smtpNewAuditTestSession().deliverOutgoingEmail(smtpNewAuditTestEmail(tt.name+"-failure@example.com"), func(*context.Context, *parsemail.Email) (error, map[string]error) {
				return deliveryErr, map[string]error{"example.net": tt.domainErr}
			})
			smtpAssertSMTPCode(t, err, tt.wantCode)
			smtpAssertAuditStatuses(t, engine, consts.EmailStatusFail)
			smtpAssertAuditError(t, engine, deliveryErr.Error(), true)
		})
	}
}

func TestPermanentDeliveryFailureSurvivesFinalAuditLock(t *testing.T) {
	engine, dsn := smtpNewAuditTestEngine(t)
	var blocker *sql.Tx
	err := smtpNewAuditTestSession().deliverOutgoingEmail(smtpNewAuditTestEmail("permanent-failure@example.com"), func(*context.Context, *parsemail.Email) (error, map[string]error) {
		blocker = smtpBeginAuditReadTransaction(t, dsn)
		failure := &textproto.Error{Code: 550, Msg: "mailbox unavailable"}
		return errors.New("delivery failed"), map[string]error{"example.net": failure}
	})
	smtpAssertSMTPCode(t, err, 550)
	if err = blocker.Rollback(); err != nil {
		t.Fatal(err)
	}
	smtpAssertAuditStatuses(t, engine, consts.EmailStatusDeliveryPending)
	smtpAssertAuditError(t, engine, smtpDeliveryPendingError, true)
}

func TestSameMessageIDCreatesIndependentSubmissions(t *testing.T) {
	engine, _ := smtpNewAuditTestEngine(t)
	var deliveries atomic.Int32
	sender := func(*context.Context, *parsemail.Email) (error, map[string]error) {
		deliveries.Add(1)
		return nil, nil
	}

	session := smtpNewAuditTestSession()
	for i := 0; i < 2; i++ {
		email := smtpNewAuditTestEmail("reused-message-id@example.com")
		email.Text = []byte(fmt.Sprintf("body %d", i))
		if err := session.deliverOutgoingEmail(email, sender); err != nil {
			t.Fatalf("submission %d: %v", i+1, err)
		}
	}
	if got := deliveries.Load(); got != 2 {
		t.Fatalf("downstream deliveries = %d, want 2", got)
	}
	smtpAssertAuditCounts(t, engine, 2, 2)
	smtpAssertMessageIDCount(t, engine, "reused-message-id@example.com", 2)
}

func TestSendAfterHookCompletesBeforeFinalPersistAndSMTPResult(t *testing.T) {
	engine, _ := smtpNewAuditTestEngine(t)
	hook := &smtpBlockingSendAfterHook{
		smtpAuthenticationCaptureHook: &smtpAuthenticationCaptureHook{},
		started:                       make(chan struct{}),
		release:                       make(chan struct{}),
		done:                          make(chan struct{}),
	}
	infra.PluginHookList = map[string]contract.EmailHook{"blocking": hook}

	result := make(chan error, 1)
	go func() {
		result <- smtpNewAuditTestSession().deliverOutgoingEmail(smtpNewAuditTestEmail("sync-hook@example.com"), smtpSuccessfulAuditSender)
	}()
	t.Cleanup(func() {
		select {
		case <-hook.release:
		default:
			close(hook.release)
		}
	})

	select {
	case <-hook.started:
	case <-time.After(time.Second):
		t.Fatal("SendAfter hook was not started")
	}
	smtpAssertAuditStatuses(t, engine, consts.EmailStatusDeliveryPending)

	select {
	case err := <-result:
		t.Fatalf("delivery returned before SendAfter hook completed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(hook.release)

	select {
	case <-hook.done:
	case <-time.After(time.Second):
		t.Fatal("SendAfter hook did not finish")
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("deliverOutgoingEmail() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("delivery did not return after SendAfter hook completed")
	}
	smtpAssertAuditStatuses(t, engine, consts.EmailStatusSent)
}

func TestOutgoingPendingAuditMailboxVisibility(t *testing.T) {
	engine, _ := smtpNewAuditTestEngine(t)
	session := smtpNewAuditTestSession()
	email := smtpNewAuditTestEmail("pending-visibility@example.com")
	_, saved, err := smtpSaveEmail(session.Ctx, email.Size, email, session.Ctx.UserID, int(consts.EmailTypeSend), nil, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != consts.EmailStatusDeliveryPending {
		t.Fatalf("saved status = %d, want %d", saved.Status, consts.EmailStatusDeliveryPending)
	}
	smtpAssertAuditStatuses(t, engine, consts.EmailStatusDeliveryPending)

	outgoing, _ := app.ListEmails(session.Ctx, dto.SearchTag{
		Type:    consts.EmailTypeSend,
		Status:  -1,
		GroupId: -1,
	}, "", false, 0, 10)
	found := false
	for _, item := range outgoing {
		if item.Id == saved.Id {
			found = true
			if item.Status != consts.EmailStatusDeliveryPending || !item.Error.Valid || item.Error.String != smtpDeliveryPendingError {
				t.Fatalf("outgoing pending audit = status:%d error:%#v", item.Status, item.Error)
			}
		}
	}
	if !found {
		t.Fatalf("pending audit email %d is missing from web outgoing list", saved.Id)
	}

	for _, mailbox := range []string{"INBOX", "Sent Messages"} {
		for _, item := range app.EmailListGetUEListByUID(session.Ctx, mailbox, 0, 0, nil) {
			if item.EmailID == saved.Id {
				t.Fatalf("pending audit email %d unexpectedly appears in IMAP mailbox %q", saved.Id, mailbox)
			}
		}
	}
}

type smtpBlockingSendAfterHook struct {
	*smtpAuthenticationCaptureHook
	started chan struct{}
	release chan struct{}
	done    chan struct{}
}

func (h *smtpBlockingSendAfterHook) SendAfter(*context.Context, *parsemail.Email, map[string]error) {
	defer close(h.done)
	close(h.started)
	<-h.release
}

func smtpNewAuditTestEngine(t *testing.T) (*xorm.Engine, string) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "audit.db") + "?_pragma=busy_timeout(20)"
	engine, err := xorm.NewEngine("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	engine.SetMaxOpenConns(1)
	engine.SetMaxIdleConns(1)
	if err = engine.Sync2(&models.User{}, &models.Email{}, &models.UserEmail{}); err != nil {
		engine.Close()
		t.Fatal(err)
	}
	if _, err = engine.Insert(&models.Email{Subject: "lock seed", MsgID: "lock-seed@example.com"}); err != nil {
		engine.Close()
		t.Fatal(err)
	}
	if _, err = engine.Insert(&models.UserEmail{UserID: 99, EmailID: 1}); err != nil {
		engine.Close()
		t.Fatal(err)
	}

	oldDB := infra.Database
	oldConfig := config.Instance
	oldHooks := infra.PluginHookList
	infra.Database = engine
	config.Instance = &config.Config{DbType: config.DBTypeSQLite, Domain: "example.com", Domains: []string{"example.com"}}
	infra.PluginHookList = nil
	t.Cleanup(func() {
		infra.Database = oldDB
		config.Instance = oldConfig
		infra.PluginHookList = oldHooks
		_ = engine.Close()
	})
	return engine, dsn
}

func smtpBeginAuditReadTransaction(t *testing.T, dsn string) *sql.Tx {
	t.Helper()
	blocker, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = blocker.Close() })
	tx, err := blocker.Begin()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = tx.QueryRow("SELECT count(*) FROM email").Scan(&count); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	return tx
}

func smtpNewAuditTestSession() *SMTPSession {
	return &SMTPSession{Ctx: &context.Context{UserID: 1, UserAccount: "sender", IsAdmin: true}}
}

func smtpNewAuditTestEmail(messageID string) *parsemail.Email {
	from := &parsemail.User{Name: "Sender", EmailAddress: "sender@example.com"}
	return &parsemail.Email{
		From:    from,
		Sender:  from,
		To:      []*parsemail.User{{EmailAddress: "recipient@example.net"}},
		Subject: "audit reliability",
		Text:    []byte("body"),
		Size:    4,
		MsgID:   messageID,
	}
}

func smtpSuccessfulAuditSender(*context.Context, *parsemail.Email) (error, map[string]error) {
	return nil, nil
}

func smtpAssertSMTPCode(t *testing.T, err error, want int) {
	t.Helper()
	var smtpErr *smtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("error = %v (%T), want *smtp.SMTPError", err, err)
	}
	if smtpErr.Code != want {
		t.Fatalf("SMTP code = %d, want %d", smtpErr.Code, want)
	}
}

func smtpAssertAuditCounts(t *testing.T, engine *xorm.Engine, wantEmail, wantUserEmail int) {
	t.Helper()
	var emailCount, userEmailCount int
	if _, err := engine.SQL("SELECT count(*) FROM email WHERE id != 1").Get(&emailCount); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.SQL("SELECT count(*) FROM user_email WHERE email_id != 1").Get(&userEmailCount); err != nil {
		t.Fatal(err)
	}
	if emailCount != wantEmail || userEmailCount != wantUserEmail {
		t.Fatalf("audit counts = email:%d user_email:%d, want email:%d user_email:%d", emailCount, userEmailCount, wantEmail, wantUserEmail)
	}
}

func smtpAssertAuditStatuses(t *testing.T, engine *xorm.Engine, want int8) {
	t.Helper()
	var emailStatus, userEmailStatus int8
	if _, err := engine.SQL("SELECT status FROM email WHERE id != 1 ORDER BY id DESC LIMIT 1").Get(&emailStatus); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.SQL("SELECT status FROM user_email WHERE email_id != 1 ORDER BY email_id DESC LIMIT 1").Get(&userEmailStatus); err != nil {
		t.Fatal(err)
	}
	if emailStatus != want || userEmailStatus != want {
		t.Fatalf("audit statuses = email:%d user_email:%d, want %d", emailStatus, userEmailStatus, want)
	}
}

func smtpAssertAuditError(t *testing.T, engine *xorm.Engine, want string, wantValid bool) {
	t.Helper()
	var got sql.NullString
	if _, err := engine.SQL("SELECT error FROM email WHERE id != 1 ORDER BY id DESC LIMIT 1").Get(&got); err != nil {
		t.Fatal(err)
	}
	if got.String != want || got.Valid != wantValid {
		t.Fatalf("audit error = %#v, want string %q valid %t", got, want, wantValid)
	}
}

func smtpAssertMessageIDCount(t *testing.T, engine *xorm.Engine, messageID string, want int) {
	t.Helper()
	var count int
	if _, err := engine.SQL("SELECT count(*) FROM email WHERE msg_id=?", messageID).Get(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("rows for Message-ID %q = %d, want %d", messageID, count, want)
	}
}
