//go:build wasip1

// WeChatPush WASM 插件：管理员收到新未读邮件时，通过微信模板消息推送；
// 能识别验证码时只推送验证码。
package main

import (
	"encoding/base64"
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Jinnrry/pmail/hooks/wasm_plugins/internal/pluginabi"
)

const (
	hookReceiveSaveAfter = 5
	hookGetName          = 6
	hookSettingsHTML     = 7
	maxHTTPResponse      = 64 * 1024
)

const verificationKeywordExpression = `(?:验证码|校验码|动态码|认证码|一次性(?:密码|口令)|verification[\s-]*code|security[\s-]*code|authentication[\s-]*code|one[\s-]*time[\s-]*(?:password|code)|passcode|\botp\b)`

var (
	verificationCodeAfterPattern  = regexp.MustCompile(`(?i)` + verificationKeywordExpression + `(?:[[:space:][:punct:]：，。！【】（）]|是|为|is|equals|如下|your|please|use){0,20}\b([A-Z0-9]{4,8})\b`)
	verificationCodeBeforePattern = regexp.MustCompile(`(?i)\b([A-Z0-9]{4,8})\b(?:[[:space:][:punct:]：，。！【】（）]|是|为|is|your|您的){0,20}` + verificationKeywordExpression)
	htmlTagPattern                = regexp.MustCompile(`<[^>]*>`)
)

var (
	token        string
	tokenExpires int64
)

//go:wasmexport pmail_alloc
func pmailAlloc(size uint32) uint32 { return pluginabi.Alloc(size) }

//go:wasmexport pmail_free
func pmailFree(ptr, size uint32) { pluginabi.Free(ptr, size) }

//go:wasmexport pmail_hook
func pmailHook(hookID, reqPtr, reqLen uint32) uint32 {
	switch hookID {
	case hookReceiveSaveAfter:
		var req hookRequest
		if err := json.Unmarshal(pluginabi.Guest(reqPtr, reqLen), &req); err != nil {
			pluginabi.Log(3, "wechat_push: bad request: "+err.Error())
			return 0
		}
		handleReceiveSaveAfter(req)
	case hookGetName:
		pluginabi.Emit([]byte(`{"name":"WeChatPushHook"}`))
	case hookSettingsHTML:
		body, _ := json.Marshal(map[string]string{"html": settingsHTML})
		pluginabi.Emit(body)
	}
	return 0
}

type hookRequest struct {
	Hook      string            `json:"hook"`
	Email     *email            `json:"email"`
	UserEmail []*userEmail      `json:"userEmail"`
	Config    map[string]string `json:"config"`
}

type email struct {
	Subject   string `json:"Subject"`
	Text      string `json:"Text"`
	HTML      string `json:"HTML"`
	MessageId int64  `json:"MessageId"`
}

type userEmail struct {
	UserID int `json:"UserID"`
	IsRead int `json:"is_read"`
	Status int `json:"status"`
}

type sendMsgRequest struct {
	Touser     string   `json:"touser"`
	TemplateID string   `json:"template_id"`
	URL        string   `json:"url"`
	Data       sendData `json:"data"`
}

type sendData struct {
	Content dataItem `json:"Content"`
}

type dataItem struct {
	Value string `json:"value"`
	Color string `json:"color"`
}

func handleReceiveSaveAfter(req hookRequest) {
	if req.Email == nil {
		return
	}
	appID := req.Config["app_id"]
	secret := req.Config["secret"]
	templateID := req.Config["template_id"]
	pushUser := req.Config["user_id"]
	if appID == "" || secret == "" || pushUser == "" {
		return
	}
	apiBase := req.Config["api_base"]
	if apiBase == "" {
		apiBase = "https://api.weixin.qq.com"
	}
	webURL := req.Config["web_url"]
	adminID := 1
	if v := req.Config["admin_user_id"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			adminID = n
		}
	}
	for _, ue := range req.UserEmail {
		if ue == nil || ue.UserID != adminID || ue.IsRead != 0 || ue.Status != 0 || req.Email.MessageId <= 0 {
			continue
		}
		accessToken := getToken(apiBase, appID, secret)
		if accessToken == "" {
			return
		}
		content := buildNotificationContent(req.Email)
		sendTemplate(apiBase, accessToken, pushUser, templateID, webURL, content)
	}
}

func buildNotificationContent(email *email) string {
	if code, ok := extractVerificationCode(email); ok {
		return "验证码:" + code
	}
	return "<<" + email.Subject + ">>  " + decodeText(email.Text)
}

func extractVerificationCode(email *email) (string, bool) {
	content := email.Subject + "\n" + decodeText(email.Text)
	if email.HTML != "" {
		content += "\n" + html.UnescapeString(htmlTagPattern.ReplaceAllString(decodeText(email.HTML), " "))
	}
	for _, pattern := range []*regexp.Regexp{verificationCodeAfterPattern, verificationCodeBeforePattern} {
		for _, match := range pattern.FindAllStringSubmatch(content, -1) {
			code := match[1]
			if strings.IndexFunc(code, unicode.IsDigit) >= 0 {
				return code, true
			}
		}
	}
	return "", false
}

func getToken(apiBase, appID, secret string) string {
	now := time.Now().Unix()
	if token != "" && tokenExpires > now+60 {
		return token
	}
	target := apiBase + "/cgi-bin/token?grant_type=client_credential&appid=" +
		url.QueryEscape(appID) + "&secret=" + url.QueryEscape(secret)
	body, code := pluginabi.HTTPGet(target, maxHTTPResponse)
	if code < 0 {
		pluginabi.Log(3, "wechat_push: token request failed: "+strconv.Itoa(int(code)))
		return ""
	}
	var r struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.AccessToken == "" {
		pluginabi.Log(3, "wechat_push: invalid token response")
		return ""
	}
	ttl := r.ExpiresIn
	if ttl > 300 {
		ttl -= 300
	}
	token = r.AccessToken
	tokenExpires = now + ttl
	return token
}

func sendTemplate(apiBase, accessToken, pushUser, templateID, webURL, content string) {
	payload, err := json.Marshal(sendMsgRequest{
		Touser:     pushUser,
		TemplateID: templateID,
		URL:        webURL,
		Data:       sendData{Content: dataItem{Value: content, Color: "#000000"}},
	})
	if err != nil {
		return
	}
	target := apiBase + "/cgi-bin/message/template/send?access_token=" + url.QueryEscape(accessToken)
	body, code := pluginabi.HTTPPost(target, payload, maxHTTPResponse)
	if code < 0 {
		pluginabi.Log(3, "wechat_push: send failed: "+strconv.Itoa(int(code)))
		return
	}
	var r struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if json.Unmarshal(body, &r) == nil && r.ErrCode != 0 {
		pluginabi.Log(2, "wechat_push: api error: "+strconv.Itoa(r.ErrCode)+" "+r.ErrMsg)
	}
}

func decodeText(b64 string) string {
	if b64 == "" {
		return ""
	}
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ""
	}
	return string(b)
}

const settingsHTML = `<div style="line-height:1.6">
<p>微信推送在 <code>plugins/wasm/wechat_push/manifest.json</code> 的 <code>config</code> 中配置：</p>
<ul>
<li><code>app_id</code>、<code>secret</code>：微信公众号凭据</li>
<li><code>template_id</code>：模板消息 ID</li>
<li><code>user_id</code>：接收推送的微信用户</li>
<li><code>web_url</code>：消息点击后跳转的邮箱地址</li>
<li><code>admin_user_id</code>：只推送该用户收到的新邮件，默认 1</li>
<li><code>api_base</code>：默认 https://api.weixin.qq.com</li>
</ul>
<p>修改后重载插件生效；<code>http_allow</code> 需包含 <code>api.weixin.qq.com</code> 或自定义 api_base 的主机。</p>
</div>`

func main() {}
