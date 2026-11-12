package httpapi

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	olog "log"
	"net/http"
	"time"

	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/id"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

// 这个服务是为了拦截http请求转发到https
var httpServerHttpServer *http.Server

func StopHTTP() {
	if httpServerHttpServer != nil {
		httpServerHttpServer.Close()
	}
}

func httpServerRouter(mux *http.ServeMux) {
	fe, err := fs.Sub(httpServerLocal, "dist")
	if err != nil {
		panic(err)
	}
	userHandler := &UserHandler{users: app.UserSvc, mfa: app.MFASvc}
	ruleHandler := &RuleHandler{rules: app.RuleSvc}
	groupHandler := &GroupHandler{groups: app.GroupSvc}
	emailHandler := &EmailHandler{queries: app.EmailQuerySvc, emails: app.EmailSvc}
	mfaHandler := &MFAHandler{mfa: app.MFASvc}
	pluginHandler := &PluginHandler{plugins: app.PluginSvc}
	mux.Handle("/", httpServerSPAHandler(fe))
	// 挑战请求类似这样 /.well-known/acme-challenge/QPyMAyaWw9s5JvV1oruyqWHG7OqkHMJEHPoUz2046KM
	mux.HandleFunc("/.well-known/", HTTPAcmeChallenge)
	mux.HandleFunc("/api/ping", HTTPPing)
	mux.HandleFunc("/api/login", httpServerContextIterceptor(userHandler.Login))
	mux.HandleFunc("/api/logout", httpServerContextIterceptor(userHandler.Logout))
	mux.HandleFunc("/api/group", httpServerContextIterceptor(groupHandler.GetUserGroup))
	mux.HandleFunc("/api/group/list", httpServerContextIterceptor(groupHandler.GetUserGroupList))
	mux.HandleFunc("/api/group/add", httpServerContextIterceptor(groupHandler.AddGroup))
	mux.HandleFunc("/api/group/del", httpServerContextIterceptor(groupHandler.DelGroup))
	mux.HandleFunc("/api/email/list", httpServerContextIterceptor(emailHandler.List))
	mux.HandleFunc("/api/email/del", httpServerContextIterceptor(emailHandler.Delete))
	mux.HandleFunc("/api/email/read", httpServerContextIterceptor(emailHandler.MarkRead))
	mux.HandleFunc("/api/email/detail", httpServerContextIterceptor(emailHandler.Detail))
	mux.HandleFunc("/api/email/move", httpServerContextIterceptor(emailHandler.Move))
	mux.HandleFunc("/api/email/send", httpServerContextIterceptor(emailHandler.Send))
	mux.HandleFunc("/api/settings/modify_password", httpServerContextIterceptor(userHandler.ModifyPassword))
	mux.HandleFunc("/api/rule/get", httpServerContextIterceptor(ruleHandler.GetRule))
	mux.HandleFunc("/api/rule/add", httpServerContextIterceptor(ruleHandler.UpsertRule))
	mux.HandleFunc("/api/rule/update", httpServerContextIterceptor(ruleHandler.UpsertRule))
	mux.HandleFunc("/api/rule/del", httpServerContextIterceptor(ruleHandler.DelRule))
	mux.HandleFunc("/attachments/", httpServerContextIterceptor(emailHandler.GetAttachments))
	mux.HandleFunc("/attachments/download/", httpServerContextIterceptor(emailHandler.Download))
	mux.HandleFunc("/api/user/create", httpServerContextIterceptor(userHandler.Create))
	mux.HandleFunc("/api/user/edit", httpServerContextIterceptor(userHandler.Edit))
	mux.HandleFunc("/api/user/info", httpServerContextIterceptor(userHandler.Info))
	mux.HandleFunc("/api/user/list", httpServerContextIterceptor(userHandler.List))
	mux.HandleFunc("/api/plugin/settings/", httpServerContextIterceptor(pluginHandler.SettingsHtml))
	mux.HandleFunc("/api/plugin/list", httpServerContextIterceptor(pluginHandler.GetList))
	mux.HandleFunc("/api/plugin/manage/list", httpServerContextIterceptor(pluginHandler.ManageList))
	mux.HandleFunc("/api/plugin/manage/approve", httpServerContextIterceptor(pluginHandler.ManageApprove))
	mux.HandleFunc("/api/plugin/manage/enable", httpServerContextIterceptor(pluginHandler.ManageEnable))
	mux.HandleFunc("/api/plugin/manage/disable", httpServerContextIterceptor(pluginHandler.ManageDisable))
	mux.HandleFunc("/api/plugin/manage/reject", httpServerContextIterceptor(pluginHandler.ManageReject))
	mux.HandleFunc("/api/plugin/manage/reload", httpServerContextIterceptor(pluginHandler.ManageReload))
	mux.HandleFunc("/api/plugin/manage/upload", httpServerContextIterceptor(pluginHandler.ManageUpload))
	mux.HandleFunc("/api/mfa/status", httpServerContextIterceptor(mfaHandler.Status))
	mux.HandleFunc("/api/mfa/setup", httpServerContextIterceptor(mfaHandler.Setup))
	mux.HandleFunc("/api/mfa/confirm", httpServerContextIterceptor(mfaHandler.Confirm))
	mux.HandleFunc("/api/mfa/disable", httpServerContextIterceptor(mfaHandler.Disable))
	mux.HandleFunc("/api/mfa/reset", httpServerContextIterceptor(mfaHandler.Reset))
	mux.HandleFunc("/api/login/mfa", httpServerContextIterceptor(mfaHandler.LoginMFA))
}

func StartHTTP() {
	mux := http.NewServeMux()

	HttpPort := 80
	if config.Instance.HttpPort > 0 {
		HttpPort = config.Instance.HttpPort
	}

	if config.Instance.HttpsEnabled != 2 {
		// 在重定向模式下，也必须显式处理 ACME 挑战，避免跳转导致验证失败
		mux.HandleFunc("/.well-known/", HTTPAcmeChallenge)
		mux.HandleFunc("/api/ping", HTTPPing)
		mux.HandleFunc("/", HTTPInterceptor)
		httpServerHttpServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", HttpPort),
			Handler:      mux,
			ReadTimeout:  time.Second * 90,
			WriteTimeout: time.Second * 90,
		}
	} else {

		httpServerRouter(mux)

		log.Infof("HttpServer Start On Port :%d", HttpPort)
		httpServerHttpServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", HttpPort),
			Handler:      app.Sessions.LoadAndSave(mux),
			ReadTimeout:  time.Second * 90,
			WriteTimeout: time.Second * 90,
		}
	}

	err := httpServerHttpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

//go:embed dist/*
var httpServerLocal embed.FS

var httpServerHttpsServer *http.Server

type httpServerNullWrite struct {
}

func (w *httpServerNullWrite) Write(p []byte) (int, error) {
	return len(p), nil
}

func StartHTTPS() {

	mux := http.NewServeMux()

	httpServerRouter(mux)

	// go http server会打一堆没用的日志，写一个空的日志处理器，屏蔽掉日志输出
	nullLog := olog.New(&httpServerNullWrite{}, "", olog.Ldate)

	HttpsPort := 443
	if config.Instance.HttpsPort > 0 {
		HttpsPort = config.Instance.HttpsPort
	}

	if config.Instance.HttpsEnabled != 2 {
		log.Infof("Https Server Start On Port :%d", HttpsPort)
		httpServerHttpsServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", HttpsPort),
			Handler:      app.Sessions.LoadAndSave(mux),
			ReadTimeout:  time.Second * 90,
			WriteTimeout: time.Second * 90,
			ErrorLog:     nullLog,
		}
		err := httpServerHttpsServer.ListenAndServeTLS(config.Instance.SSLPublicKeyPath, config.Instance.SSLPrivateKeyPath)
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				// 正常关闭（重启或停机）
				log.Infof("Https Server closed normally on port :%d", HttpsPort)
			} else {
				// 异常错误仍然打印并退出
				log.Errorf("Https Server error on port :%d, %+v", HttpsPort, err)
				panic(err)
			}
		}
	}
}

func StopHTTPS() {
	if httpServerHttpsServer != nil {
		httpServerHttpsServer.Close()
	}
}

// 新增：分类处理关闭错误

// 注入context
func httpServerContextIterceptor(h HTTPHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}

		ctx := &context.Context{}
		ctx.Context = r.Context()
		ctx.SetValue(context.LogID, id.GenLogID())
		lang := r.Header.Get("Lang")
		if lang == "" {
			lang = "en"
		}
		ctx.Lang = lang

		if config.IsInit {
			user := cast.ToString(app.Sessions.Get(ctx, "user"))
			var userInfo models.User
			if user != "" {
				_ = json.Unmarshal([]byte(user), &userInfo)
			}
			if userInfo.ID > 0 {
				ctx.UserID = userInfo.ID
				ctx.UserName = userInfo.Name
				ctx.UserAccount = userInfo.Account
				ctx.IsAdmin = userInfo.IsAdmin == 1
			}

			if userInfo.ID == 0 {
				token := r.Header.Get("Token")
				if token != "" {
					user, err := app.UserSvc.VerifyToken(token)
					if err == nil && user != nil && user.ID > 0 {
						if app.MFAIsEnabled(user) {
							response.NewErrorResponse(response.NeedLogin, app.TextGetText(ctx.Lang, "mfa_login_required"), "").FPrint(w)
							return
						}
						ctx.UserID = user.ID
						ctx.UserName = user.Name
						ctx.UserAccount = user.Account
						ctx.IsAdmin = user.IsAdmin == 1
					} else {
						response.NewErrorResponse(response.NeedLogin, err.Error(), "").FPrint(w)
						return
					}
				}
			}

			if ctx.UserID == 0 {
				if r.URL.Path != "/api/ping" && r.URL.Path != "/api/login" && r.URL.Path != "/api/login/mfa" {
					response.NewErrorResponse(response.NeedLogin, app.TextGetText(ctx.Lang, "login_exp"), "").FPrint(w)
					return
				}
			}
		} else if r.URL.Path != "/api/setup" {
			response.NewErrorResponse(response.NeedSetup, "", "").FPrint(w)
			return
		}
		h(ctx, w, r)
	}
}

func HTTPInterceptor(w http.ResponseWriter, r *http.Request) {
	URL := "https://" + config.Instance.WebDomain + r.URL.Path
	http.Redirect(w, r, URL, http.StatusMovedPermanently)
}

type HTTPHandlerFunc func(*context.Context, http.ResponseWriter, *http.Request)

func HTTPPing(w http.ResponseWriter, req *http.Request) {
	response.NewSuccessResponse("pong").FPrint(w)
}
