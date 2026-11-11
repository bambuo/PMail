package infra

import (
	oContext "context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/hooks/framework"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

// HookList
var PluginHookList map[string]framework.EmailHook

// hookListMu 保护 HookList。插件进程退出时会并发地从 HookList 移除条目，
// 而收发邮件的 goroutine 会并发读取，必须加锁避免数据竞争。
var pluginHookListMu sync.RWMutex

// AllHooks 返回当前已注册插件的快照，供收发邮件时并发安全地遍历。
func PluginAllHooks() []framework.EmailHook {
	pluginHookListMu.RLock()
	defer pluginHookListMu.RUnlock()
	list := make([]framework.EmailHook, 0, len(PluginHookList))
	for _, h := range PluginHookList {
		list = append(list, h)
	}
	return list
}

// HookNames 返回已注册插件的名称列表。
func PluginHookNames() []string {
	pluginHookListMu.RLock()
	defer pluginHookListMu.RUnlock()
	names := make([]string, 0, len(PluginHookList))
	for name := range PluginHookList {
		names = append(names, name)
	}
	return names
}

// GetHook 返回指定名称的插件。
func PluginGetHook(name string) (framework.EmailHook, bool) {
	pluginHookListMu.RLock()
	defer pluginHookListMu.RUnlock()
	h, ok := PluginHookList[name]
	return h, ok
}

// RegisterHook 注册一个插件。
func PluginRegisterHook(name string, h framework.EmailHook) {
	pluginHookListMu.Lock()
	defer pluginHookListMu.Unlock()
	PluginHookList[name] = h
}

// RemoveHook 移除指定名称的插件。
func PluginRemoveHook(name string) {
	pluginHookListMu.Lock()
	defer pluginHookListMu.Unlock()
	delete(PluginHookList, name)
}

type PluginHookSender struct {
	httpc  http.Client
	name   string
	socket string
}

func (h *PluginHookSender) ReceiveSaveAfter(ctx *context.Context, email *parsemail.Email, ue []*models.UserEmail) {
	log.WithContext(ctx).Debugf("[%s]Plugin ReceiveSaveAfter Start", h.name)

	dto := framework.HookDTO{
		Ctx:       ctx,
		Email:     email,
		UserEmail: ue,
	}
	body, _ := json.Marshal(dto)

	_, err := h.httpc.Post("http://unix/ReceiveSaveAfter", "application/json", strings.NewReader(string(body)))
	if err != nil {
		log.WithContext(ctx).Errorf("[%s] Error! %v", h.name, err)
		return
	}

	log.WithContext(ctx).Debugf("[%s]Plugin ReceiveSaveAfter End", h.name)
}

func (h *PluginHookSender) SendBefore(ctx *context.Context, email *parsemail.Email) {
	log.WithContext(ctx).Debugf("[%s]Plugin SendBefore Start", h.name)

	dto := framework.HookDTO{
		Ctx:   ctx,
		Email: email,
	}
	body, _ := json.Marshal(dto)

	ret, err := h.httpc.Post("http://unix/SendBefore", "application/json", strings.NewReader(string(body)))
	if err != nil {
		log.WithContext(ctx).Errorf("[%s] Error! %v", h.name, err)
		return
	}

	body, _ = io.ReadAll(ret.Body)
	json.Unmarshal(body, &dto)

	ctx = dto.Ctx
	email = dto.Email
	log.WithContext(ctx).Debugf("[%s]Plugin SendBefore End", h.name)

}

func (h *PluginHookSender) SendAfter(ctx *context.Context, email *parsemail.Email, err map[string]error) {
	log.WithContext(ctx).Debugf("[%s]Plugin SendAfter Start", h.name)
	dto := framework.HookDTO{
		Ctx:    ctx,
		Email:  email,
		ErrMap: err,
	}
	body, _ := json.Marshal(dto)

	_, errL := h.httpc.Post("http://unix/SendAfter", "application/json", strings.NewReader(string(body)))
	if errL != nil {
		log.WithContext(ctx).Errorf("[%s] Error! %v", h.name, errL)
		return
	}

	log.WithContext(ctx).Debugf("[%s]Plugin SendAfter End", h.name)

}

func (h *PluginHookSender) ReceiveParseBefore(ctx *context.Context, email *[]byte) {
	log.WithContext(ctx).Debugf("[%s]Plugin ReceiveParseBefore Start", h.name)

	dto := framework.HookDTO{
		Ctx:       ctx,
		EmailByte: email,
	}
	body, _ := json.Marshal(dto)

	ret, errL := h.httpc.Post("http://unix/ReceiveParseBefore", "application/json", strings.NewReader(string(body)))
	if errL != nil {
		log.WithContext(ctx).Errorf("[%s] Error! %v", h.name, errL)
		return
	}

	body, _ = io.ReadAll(ret.Body)
	json.Unmarshal(body, &dto)

	ctx = dto.Ctx
	email = dto.EmailByte
	log.WithContext(ctx).Debugf("[%s]Plugin ReceiveParseBefore End", h.name)

}

func (h *PluginHookSender) ReceiveParseAfter(ctx *context.Context, email *parsemail.Email) {
	log.WithContext(ctx).Debugf("[%s]Plugin ReceiveParseAfter Start", h.name)

	dto := framework.HookDTO{
		Ctx:   ctx,
		Email: email,
	}
	body, _ := json.Marshal(dto)

	ret, errL := h.httpc.Post("http://unix/ReceiveParseAfter", "application/json", strings.NewReader(string(body)))
	if errL != nil {
		log.WithContext(ctx).Errorf("[%s] Error! %v", h.name, errL)
		return
	}

	body, _ = io.ReadAll(ret.Body)
	json.Unmarshal(body, &dto)

	ctx = dto.Ctx
	email = dto.Email
	log.WithContext(ctx).Debugf("[%s]Plugin ReceiveParseAfter End", h.name)

}

// GetName 获取插件名称
func (h *PluginHookSender) GetName(ctx *context.Context) string {

	dto := framework.HookDTO{
		Ctx: ctx,
	}
	body, _ := json.Marshal(dto)

	ret, errL := h.httpc.Post("http://unix/GetName", "application/json", strings.NewReader(string(body)))
	if errL != nil {
		log.WithContext(ctx).Errorf("[%s] Error! %v", h.name, errL)
		return ""
	}

	body, _ = io.ReadAll(ret.Body)

	return string(body)
}

// SettingsHtml 插件页面
func (h *PluginHookSender) SettingsHtml(ctx *context.Context, url string, requestData string) string {

	dto := framework.SettingsHtmlRequest{
		Ctx:         ctx,
		URL:         url,
		RequestData: requestData,
	}
	body, _ := json.Marshal(dto)

	ret, errL := h.httpc.Post("http://unix/SettingsHtml", "application/json", strings.NewReader(string(body)))
	if errL != nil {
		log.WithContext(ctx).Errorf("[%s] Error! %v", h.name, errL)
		return ""
	}

	body, _ = io.ReadAll(ret.Body)

	return string(body)

}

func PluginNewHookSender(socketPath string, name string, serverVersion string) *PluginHookSender {
	httpc := http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			DialContext: func(ctx oContext.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}
	return &PluginHookSender{
		httpc:  httpc,
		socket: socketPath,
		name:   name,
	}
}

var pluginProcessList []*os.Process

// Init 注册hook对象
func PluginInit(serverVersion string) {

	PluginHookList = map[string]framework.EmailHook{}
	env := os.Environ()
	procAttr := &os.ProcAttr{
		Env: env,
		Files: []*os.File{
			os.Stdin,
			os.Stdout,
			os.Stderr,
		},
	}

	root := "./plugins"

	pluginNo := 1
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info != nil && !info.IsDir() && (!strings.Contains(info.Name(), ".") || strings.Contains(info.Name(), ".exe")) {

			socketPath := fmt.Sprintf("%s/%d.socket", root, pluginNo)

			os.Remove(socketPath)

			//socketPath = "/PMail/server/hooks/spam_block/1555.socket"  //debug

			log.Infof("[%s] Plugin Load", info.Name())
			p, err := os.StartProcess(path, []string{
				info.Name(),
				fmt.Sprintf("%d.socket", pluginNo),
			}, procAttr)
			if err != nil {
				log.Errorf("Plug Load Error! %v", err)
				return nil
			}
			fmt.Printf("[%s] Plugin Start! PID:%d", info.Name(), p.Pid)
			pluginProcessList = append(pluginProcessList, p)

			pluginNo++

			// registeredName 记录该插件最终注册进 HookList 使用的键。
			// 清理协程在插件进程退出后必须用同一个键去移除，
			// 否则会残留指向已死亡 socket 的 HookSender。
			var registeredName string
			var registeredNameMu sync.Mutex

			go func() {
				stat, err := p.Wait()
				log.Errorf("[%s] Plugin Stop. Error:%v Stat:%v", info.Name(), err, stat.String())
				registeredNameMu.Lock()
				name := registeredName
				registeredNameMu.Unlock()
				if name != "" {
					PluginRemoveHook(name)
					log.Infof("[%s] Plugin Removed From HookList", name)
				}
				os.Remove(socketPath)
			}()

			loadSucc := false
			for i := 0; i < 5; i++ {
				time.Sleep(1 * time.Second)
				if _, err := os.Stat(socketPath); err == nil {
					loadSucc = true
					break
				}
				if i == 4 {
					log.Errorf("[%s] Start Fail!", info.Name())
				}
			}
			if loadSucc {
				hk := PluginNewHookSender(socketPath, info.Name(), serverVersion)
				hkName := hk.GetName(&context.Context{})
				if hkName == "" {
					// GetName 失败时退回插件文件名作为键，避免出现空键。
					hkName = info.Name()
				}
				PluginRegisterHook(hkName, hk)
				registeredNameMu.Lock()
				registeredName = hkName
				registeredNameMu.Unlock()
				log.Infof("[%s] Plugin Load Success!", hkName)
			}

		}

		return nil
	})

}

func PluginStop() {
	log.Info("Plugin Stop")
	for _, process := range pluginProcessList {
		process.Kill()
	}
}
