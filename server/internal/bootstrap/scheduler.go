package bootstrap

import (
	"time"

	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/internal/infra/certificate"
	log "github.com/sirupsen/logrus"

	// 新增：HTTP 就绪探测
	"fmt"
	"net/http"
)

var certificateSchedulerExpiredTime time.Time

func CertificateSchedulerStart() {

	// 第一次启动，等待到初始化完成
	if config.Instance == nil || config.IsInit == false {
		for {
			time.Sleep(1 * time.Minute)
			if config.Instance != nil && config.IsInit {
				break
			}
		}
	}

	if config.Instance.SSLType == config.SSLTypeAutoHTTP || config.Instance.SSLType == config.SSLTypeAutoDNS {
		go certificateSchedulerSslUpdateLoop()
	} else {
		go certificateSchedulerSslCheck()
	}

}

// 每天检查一遍SSL证书是否更新，更新就重启
func certificateSchedulerSslCheck() {
	var err error
	_, certificateSchedulerExpiredTime, _, err = certificate.CheckSSLCrtInfo()
	if err != nil {
		panic(err)
	}

	for {
		time.Sleep(24 * time.Hour)
		_, newExpTime, _, err := certificate.CheckSSLCrtInfo()
		if err != nil {
			log.Errorf("SSL Check Error! %+v", err)
		}
		if newExpTime != certificateSchedulerExpiredTime {
			certificateSchedulerExpiredTime = newExpTime
			log.Infoln("SSL certificate had update! restarting")
			app.LifecycleRestartChan <- true
		}

	}
}

// 每天检查一遍SSL证书是否即将过期，即将过期就重新生成
func certificateSchedulerSslUpdateLoop() {
	for {
		// 等待 HTTP 就绪后再执行 ACME 续期，避免挑战失败
		certificateSchedulerWaitHTTPReady()
		certificate.Update(true)
		// 每24小时检测一次证书有效期
		time.Sleep(24 * time.Hour)
	}
}

// 新增：HTTP 就绪探测（最多等待 ~90 秒）
func certificateSchedulerWaitHTTPReady() {
	port := 80
	if config.Instance != nil && config.Instance.HttpPort > 0 {
		port = config.Instance.HttpPort
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/ping", port)

	for i := 0; i < 90; i++ {
		resp, err := http.Get(url)
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Infof("HTTP ready: %s", url)
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	log.Warnf("HTTP not ready after 90s, skipping SSL update this round")
}
