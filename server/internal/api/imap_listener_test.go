package api

import (
	"bufio"
	"crypto/tls"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/infra"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/password"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
)

var imapClientUnLogin *imapclient.Client
var imapClientLogin *imapclient.Client
var imapImapTestAddr string

func TestMain(m *testing.M) {
	bindTestDependencies()
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	config.ROOT_PATH = filepath.Clean(filepath.Join(wd, "../..")) + string(os.PathSeparator)
	config.Init()
	if err := infra.InitDatabase(""); err != nil {
		panic(err)
	}
	imapSeedIMAPTestData()
	crt, err := tls.LoadX509KeyPair(config.Instance.SSLPublicKeyPath, config.Instance.SSLPrivateKeyPath)
	if err != nil {
		panic(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{crt}})
	if err != nil {
		panic(err)
	}
	imapImapTestAddr = ln.Addr().String()
	imapInstanceTLS = imapNewIMAPServer()
	go func() {
		if err := imapInstanceTLS.Serve(ln); err != nil {
			panic(err)
		}
	}()

	options := &imapclient.Options{
		WordDecoder: &mime.WordDecoder{CharsetReader: charset.Reader},
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	imapClientUnLogin, err = imapclient.DialTLS(imapImapTestAddr, options)
	if err != nil {
		panic(err)
	}

	imapClientLogin, err = imapclient.DialTLS(imapImapTestAddr, options)
	if err != nil {
		panic(err)
	}

	err = imapClientLogin.Login("testCase", "testCase").Wait()
	if err != nil {
		panic(err)
	}

	code := m.Run()
	imapClientUnLogin.Close()
	imapClientLogin.Close()
	StopIMAP()
	os.Exit(code)
}

func imapSeedIMAPTestData() {
	var user models.User
	_, err := infra.Database.Table(&models.User{}).Where("account=?", "testCase").Get(&user)
	if err != nil {
		panic(err)
	}
	if user.ID == 0 {
		user = models.User{Account: "testCase", Name: "testCase", Password: password.Hash("testCase")}
		if _, err = infra.Database.Insert(&user); err != nil {
			panic(err)
		}
	}

	var num int
	_, err = infra.Database.Table(&models.UserEmail{}).Where("user_id=?", user.ID).Select("count(1)").Get(&num)
	if err != nil {
		panic(err)
	}
	if num > 0 {
		return
	}

	statuses := []int8{3}
	for i := 0; i < 10; i++ {
		statuses = append(statuses, 0)
	}
	for _, status := range statuses {
		email := models.Email{Subject: "test", Status: status}
		if _, err = infra.Database.Insert(&email); err != nil {
			panic(err)
		}
		ue := models.UserEmail{UserID: user.ID, EmailID: email.Id, Status: status}
		if _, err = infra.Database.Insert(&ue); err != nil {
			panic(err)
		}
	}
}

func TestCapability(t *testing.T) {

	res, err := imapClientUnLogin.Capability().Wait()
	if err != nil {
		t.Error(err)
	}
	if _, ok := res["IMAP4rev1"]; !ok {
		t.Error("Capability Error")
	}

	res, err = imapClientLogin.Capability().Wait()
	if err != nil {
		t.Error(err)
	}
	if _, ok := res["IDLE"]; !ok {
		t.Error("IDLE Capability Error")
	}

}

func TestLogin(t *testing.T) {
	err := imapClientUnLogin.Login("testCase", "testCaseasdfsadf").Wait()
	sErr := err.(*imap.Error)
	if sErr.Code != "AUTHENTICATIONFAILED" {
		t.Error("Login Error")
	}
}

func TestCreate(t *testing.T) {
	err := imapClientLogin.Create("一级菜单", nil).Wait()
	if err != nil {
		t.Error(err)
	}

	err = imapClientLogin.Create("一级菜单/二级菜单", nil).Wait()
	if err != nil {
		t.Error(err)
	}

	res, err := imapClientLogin.List("", "*", nil).Collect()
	if err != nil {
		t.Error(err)
	}
	var mailbox []string
	for _, v := range res {
		mailbox = append(mailbox, v.Mailbox)
	}

	if !array.InArray("一级菜单", mailbox) || !array.InArray("一级菜单/二级菜单", mailbox) {
		t.Error(mailbox)
	}

}

func TestRename(t *testing.T) {

	err := imapClientLogin.Rename("一级菜单", "主菜单").Wait()
	if err != nil {
		t.Error(err)
	}

	res, err := imapClientLogin.List("", "*", nil).Collect()
	if err != nil {
		t.Error(err)
	}
	var mailbox []string
	for _, v := range res {
		mailbox = append(mailbox, v.Mailbox)
	}

	if !array.InArray("主菜单", mailbox) {
		t.Error(mailbox)
	}
}

func TestList(t *testing.T) {
	res, err := imapClientUnLogin.List("", "", &imap.ListOptions{}).Collect()

	if err == nil {
		t.Logf("%+v", res)
		t.Error("List Unlogin error")
	}

	res, err = imapClientLogin.List("", "", &imap.ListOptions{}).Collect()
	if err != nil {
		t.Error(err)
	}
	if len(res) == 0 {
		t.Error("List Error")
	}

	res, err = imapClientLogin.List("", "*", &imap.ListOptions{}).Collect()
	if err != nil {
		t.Error(err)
	}
	if len(res) == 0 {
		t.Error("List Error")
	}

	res, err = imapClientLogin.List("", "一级菜单/%", &imap.ListOptions{}).Collect()
	if err != nil {
		t.Error(err)
	}
	if len(res) == 0 {
		t.Error("List Error")
	}

	if len(res) != 1 {
		t.Error("List Error")
	}

	res, err = imapClientLogin.List("", "一级菜单/*", &imap.ListOptions{}).Collect()
	if err != nil {
		t.Error(err)
	}
	if len(res) != 1 {
		t.Error("List Error")
	}
	if len(res) == 0 {
		t.Error("List Error")
	}

}

func TestDelete(t *testing.T) {

	imapClientLogin.Create("一级菜单/二级菜单", nil).Wait()

	err := imapClientLogin.Delete("二级菜单").Wait()
	if err != nil {
		t.Error(err)
	}
	res, err := imapClientLogin.List("", "*", nil).Collect()
	if err != nil {
		t.Error(err)
	}
	var mailbox []string
	for _, v := range res {
		mailbox = append(mailbox, v.Mailbox)
	}

	if array.InArray("二级菜单", mailbox) {
		t.Error(mailbox)
	}

}

func TestAppend(t *testing.T) {

}
func TestSelect(t *testing.T) {
	res, err := imapClientUnLogin.Select("INBOX", &imap.SelectOptions{}).Wait()
	if err == nil {
		t.Logf("%+v", res)
		t.Error("Select Unlogin error")
	}

	res, err = imapClientLogin.Select("INBOX", &imap.SelectOptions{}).Wait()
	if err != nil {
		t.Logf("%+v", res)
		t.Error("Select error")
	}

	if res == nil || res.NumMessages == 0 {
		t.Error("Select Error")
	}

	res, err = imapClientLogin.Select("Deleted Messages", &imap.SelectOptions{}).Wait()
	if err != nil {
		t.Logf("%+v", res)
		t.Error("Select error")
	}

	if res == nil || res.NumMessages == 0 {
		t.Error("Select Error")
	}

}

func TestStatus(t *testing.T) {
	res, err := imapClientUnLogin.Status("INBOX", &imap.StatusOptions{}).Wait()
	if err == nil {
		t.Logf("%+v", res)
		t.Error("Select Unlogin error")
	}

	res, err = imapClientLogin.Status("INBOX", &imap.StatusOptions{}).Wait()
	if err != nil {
		t.Logf("%+v", res)
		t.Error("Select error")
	}

}

func TestFetch(t *testing.T) {
	res2, err := imapClientLogin.Select("INBOX", &imap.SelectOptions{}).Wait()
	if err != nil {
		t.Logf("%+v", res2)
		t.Error("Fetch error")
	}

	res, err := imapClientLogin.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		UID:          true,
		BodySection: []*imap.FetchItemBodySection{
			{
				Specifier: imap.PartSpecifierText,
				Peek:      true,
			},
		},
	}).Collect()
	if err != nil {
		t.Logf("%+v", res)
		t.Error("Fetch error")
	}

	res, err = imapClientLogin.Fetch(imap.SeqSetNum(1, 2, 3, 4, 5, 6, 7, 8, 9), &imap.FetchOptions{
		Flags: true,
		UID:   true,
	}).Collect()
	if err != nil {
		t.Logf("%+v", res)
		t.Error("Fetch error")
	}

	res, err = imapClientLogin.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		UID:          true,
		BodySection: []*imap.FetchItemBodySection{
			{
				Specifier:    imap.PartSpecifierHeader,
				HeaderFields: []string{"subject"},
				Peek:         true,
			},
		},
	}).Collect()
	if err != nil {
		t.Logf("%+v", res)
		t.Error("Fetch error")
	}

	res, err = imapClientLogin.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{
		UID: true,
		BodySection: []*imap.FetchItemBodySection{
			{
				Specifier: imap.PartSpecifierHeader,
				Peek:      true,
			},
		},
	}).Collect()
	if err != nil {
		t.Logf("%+v", res)
		t.Error("Fetch error")
	}
}
func TestStore(t *testing.T) {
	res, err := imapClientLogin.Store(
		imap.UIDSetNum(1),
		&imap.StoreFlags{
			Op:    imap.StoreFlagsAdd,
			Flags: []imap.Flag{"\\Seen"},
		},
		&imap.StoreOptions{}).Collect()
	if err != nil {
		t.Errorf("%+v", err)
	}
	t.Logf("%+v", res)

}
func TestClose(t *testing.T) {

}
func TestExpunge(t *testing.T) {

	imapClientLogin.Select("INBOX", &imap.SelectOptions{}).Wait()

	res, err := imapClientLogin.UIDExpunge(imap.UIDSetNum(1, 2)).Collect()

	if err != nil {
		t.Errorf("%+v", err)
	}
	t.Logf("%+v", res)
	var ues []models.UserEmail
	infra.Database.
		Table("user_email").Where("id=1 or id=2").Find(&ues)
	if len(ues) > 0 {
		t.Errorf("TestExpunge Error")
	}

}
func TestExamine(t *testing.T) {

}
func TestSubscribe(t *testing.T) {

}
func TestUnSubscribe(t *testing.T) {

}
func TestLSub(t *testing.T) {

}

func TestCheck(t *testing.T) {

}
func TestSearch(t *testing.T) {
	imapClientLogin.Select("INBOX", &imap.SelectOptions{}).Wait()

	res, err := imapClientLogin.Search(&imap.SearchCriteria{
		UID: []imap.UIDSet{
			[]imap.UIDRange{
				{Start: 1},
			},
			[]imap.UIDRange{
				{Start: 2},
			},
			[]imap.UIDRange{
				{Start: 2, Stop: 5},
			},
		},
	}, &imap.SearchOptions{}).Wait()
	if err != nil {
		t.Errorf("%+v", err)
	}
	t.Logf("%+v", res)
}
func TestMove(t *testing.T) {
	imapClientLogin.Select("INBOX", &imap.SelectOptions{}).Wait()

	res, err := imapClientLogin.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		UID:          true,
		BodySection: []*imap.FetchItemBodySection{
			{
				Specifier: imap.PartSpecifierText,
				Peek:      true,
			},
		},
	}).Collect()
	if err != nil {
		t.Logf("%+v", res)
		t.Error("Fetch error")
	}

	if len(res) > 0 {
		uid := res[0].UID

		_, err = imapClientLogin.Move(imap.UIDSetNum(uid), "Junk").Wait()
		if err != nil {
			t.Errorf("%+v", err)
		}

	}

}

func TestCopy(t *testing.T) {
	imapClientLogin.Select("INBOX", &imap.SelectOptions{}).Wait()

	res, err := imapClientLogin.Fetch(imap.SeqSetNum(1), &imap.FetchOptions{
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		UID:          true,
		BodySection: []*imap.FetchItemBodySection{
			{
				Specifier: imap.PartSpecifierText,
				Peek:      true,
			},
		},
	}).Collect()
	if err != nil {
		t.Logf("%+v", res)
		t.Error("Fetch error")
	}

	if len(res) > 0 {
		_, err = imapClientLogin.Copy(imap.UIDSetNum(res[0].UID), "Junk").Wait()
		if err != nil {
			t.Errorf("%+v", err)
		}
	} else {
		t.Error("No Fetch Result")
	}

}

func TestNoop(t *testing.T) {
	err := imapClientLogin.Noop().Wait()
	if err != nil {
		t.Error(err)
	}
}
func TestIDLE(t *testing.T) {
	conn, err := tls.Dial("tcp", imapImapTestAddr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "OK") {
		t.Fatalf("unexpected greeting: %s", line)
	}

	if _, err = conn.Write([]byte("a001 LOGIN testCase testCase\r\n")); err != nil {
		t.Fatal(err)
	}
	if line, err = imapReadTaggedLine(reader, "a001"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "OK") {
		t.Fatalf("unexpected LOGIN response: %s", line)
	}

	if _, err = conn.Write([]byte("a002 SELECT INBOX\r\n")); err != nil {
		t.Fatal(err)
	}
	if line, err = imapReadTaggedLine(reader, "a002"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "OK") {
		t.Fatalf("unexpected SELECT response: %s", line)
	}

	if _, err = conn.Write([]byte("a003 IDLE\r\n")); err != nil {
		t.Fatal(err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, "+") || !strings.Contains(strings.ToLower(line), "idling") {
		t.Fatalf("unexpected IDLE continuation: %s", line)
	}

	if _, err = conn.Write([]byte("DONE\r\n")); err != nil {
		t.Fatal(err)
	}
	if line, err = imapReadTaggedLine(reader, "a003"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "OK") {
		t.Fatalf("unexpected DONE response: %s", line)
	}
}

func TestIdleNotice(t *testing.T) {
	updates := make(chan uint32, 1)
	options := &imapclient.Options{
		WordDecoder: &mime.WordDecoder{CharsetReader: charset.Reader},
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if data.NumMessages != nil {
					updates <- *data.NumMessages
				}
			},
		},
	}

	client, err := imapclient.DialTLS(imapImapTestAddr, options)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err = client.Login("testCase", "testCase").Wait(); err != nil {
		t.Fatal(err)
	}

	selected, err := client.Select("INBOX", &imap.SelectOptions{}).Wait()
	if err != nil {
		t.Fatal(err)
	}

	idleCmd, err := client.Idle()
	if err != nil {
		t.Fatal(err)
	}

	var user models.User
	_, err = infra.Database.Table(&models.User{}).Where("account=?", "testCase").Get(&user)
	if err != nil {
		t.Fatal(err)
	}
	if user.ID == 0 {
		t.Fatal("test user not found")
	}

	ctx := &pcontext.Context{UserID: user.ID}
	if err = NewIMAPIdleNotifier(app.GroupSvc).Notify(ctx, user.ID, &models.Email{Id: 1}); err != nil {
		t.Fatal(err)
	}

	select {
	case numMessages := <-updates:
		if numMessages != selected.NumMessages {
			t.Fatalf("unexpected IDLE EXISTS update: got %d, want %d", numMessages, selected.NumMessages)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for IDLE update")
	}

	if err = idleCmd.Close(); err != nil {
		t.Fatal(err)
	}
	if err = idleCmd.Wait(); err != nil {
		t.Fatal(err)
	}
	imapWaitIdleConnectionsDeleted(t, user.ID)
}

func imapReadTaggedLine(reader *bufio.Reader, tag string) (string, error) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return line, err
		}
		if strings.HasPrefix(line, tag+" ") {
			return line, nil
		}
	}
}

func imapWaitIdleConnectionsDeleted(t *testing.T, userId int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := imapUserConnects.Load(userId); !ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("idle connections were not cleaned up")
}
func TestUnselect(t *testing.T) {

}

func TestLogout(t *testing.T) {
	err := imapClientLogin.Logout().Wait()
	if err != nil {
		t.Error(err)
	}
}
