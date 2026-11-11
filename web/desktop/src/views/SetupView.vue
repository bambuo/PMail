<template>
  <div id="main">
    <a-steps
      id="status"
      :current="active"
    >
      <a-step :title="lang.welcome" />
      <a-step :title="lang.setDatabase" />
      <a-step :title="lang.setAdminPassword" />
      <a-step :title="lang.SetDomail" />
      <a-step :title="lang.setDNS" />
      <a-step :title="lang.setSSL" />
    </a-steps>

    <div
      v-if="active === 0"
      class="ctn"
    >
      <div class="desc">
        <h2>{{ lang.tks_pmail }}</h2>
        <div style="margin-top: 10px">
          {{ lang.guid_desc }}
        </div>
      </div>
    </div>

    <div
      v-if="active === 1"
      class="ctn"
    >
      <div class="desc">
        <h2>{{ lang.select_db }}</h2>
        <div style="margin-top: 10px">
          {{ lang.db_desc }}
        </div>
      </div>
      <div class="form setup-form">
        <a-form
          :model="dbSettings"
          layout="vertical"
        >
          <a-form-item :label="lang.type">
            <a-select
              v-model="dbSettings.type"
              :placeholder="lang.db_select_ph"
              @change="dbSettings.dsn = ''"
            >
              <a-option
                label="MySQL"
                value="mysql"
              />
              <a-option
                label="SQLite3"
                value="sqlite"
              />
              <a-option
                label="PostgreSQL"
                value="postgres"
              />
            </a-select>
          </a-form-item>

          <a-form-item
            v-if="dbSettings.type === 'mysql'"
            :label="lang.mysql_dsn"
          >
            <a-textarea
              v-model="dbSettings.dsn"
              :auto-size="{ minRows: 2, maxRows: 3 }"
              :placeholder="mysqlPlaceholder"
            />
          </a-form-item>

          <a-form-item
            v-if="dbSettings.type === 'postgres'"
            :label="lang.pg_dsn"
          >
            <a-textarea
              v-model="dbSettings.dsn"
              :auto-size="{ minRows: 2, maxRows: 3 }"
              :placeholder="pgPlaceholder"
            />
          </a-form-item>

          <a-form-item
            v-if="dbSettings.type === 'sqlite'"
            :label="lang.sqlite_db_path"
          >
            <a-input
              v-model="dbSettings.dsn"
              placeholder="./config/pmail.db"
            />
          </a-form-item>
        </a-form>
      </div>
    </div>

    <div
      v-if="active === 2"
      class="ctn"
    >
      <div class="desc">
        <h2>{{ lang.setAdminPassword }}</h2>
      </div>
      <div class="form setup-form">
        <a-form
          :model="adminSettings"
          layout="vertical"
        >
          <a-form-item :label="lang.admin_account">
            <a-input
              v-model="adminSettings.account"
              :disabled="adminSettings.hadSeted"
              placeholder="admin"
            />
          </a-form-item>

          <a-form-item :label="lang.password">
            <a-input-password
              v-model="adminSettings.password"
              :disabled="adminSettings.hadSeted"
            />
          </a-form-item>

          <a-form-item :label="lang.enter_again">
            <a-input-password
              v-model="adminSettings.password2"
              :disabled="adminSettings.hadSeted"
            />
          </a-form-item>
        </a-form>
      </div>
    </div>

    <div
      v-if="active === 3"
      class="ctn"
    >
      <div class="desc">
        <h2>{{ lang.SetDomail }}</h2>
      </div>
      <div class="form setup-form">
        <a-form
          :model="domainSettings"
          layout="vertical"
        >
          <a-form-item :label="lang.smtp_domain">
            <a-input
              v-model="domainSettings.smtp_domain"
              placeholder="domain.com"
              prepend="smtp."
            />
          </a-form-item>

          <a-form-item :label="lang.web_domain">
            <a-input
              v-model="domainSettings.web_domain"
              placeholder="pmail.domain.com"
            />
          </a-form-item>

          <a-form-item :label="lang.multi_domain_setting">
            <div class="multi-domain">
              <span>{{ lang.multi_domain_setting_desc }}</span>
              <a-button
                size="small"
                type="primary"
                @click="addDomain"
              >
                <template #icon>
                  <IconPlus />
                </template>
              </a-button>
            </div>
            <a-input
              v-for="(item, i) in domainSettings.multi_domain"
              :key="i"
              v-model="domainSettings.multi_domain[i]"
              :placeholder="'domain' + i + '.com'"
              class="multi-domain-input"
            />
          </a-form-item>
        </a-form>
      </div>
    </div>

    <div
      v-if="active === 4"
      class="ctn_s"
    >
      <div class="desc">
        <h2>{{ lang.setDNS }}</h2>
        <div style="margin-top: 10px">
          {{ lang.dns_desc }}
        </div>
      </div>
      <div
        v-for="(info, domain) in dnsInfos"
        :key="domain"
        class="form dns-form"
      >
        <h3>{{ domain }}</h3>
        <a-table
          :data="info"
          :columns="dnsColumns"
          :pagination="false"
          bordered
        >
          <template #host="{ record }">
            <a-tooltip
              v-if="record.host === '' || record.host === '@'"
              :content="lang.dns_root_desc"
              position="top"
            >
              <span>{{ record.host }}</span>
            </a-tooltip>
            <span v-else>{{ record.host }}</span>
          </template>
          <template #value="{ record }">
            <a-tooltip
              v-if="record.tips !== ''"
              :content="record.tips"
              position="top"
            >
              <span>{{ record.value }}</span>
            </a-tooltip>
            <span v-else>{{ record.value }}</span>
          </template>
        </a-table>
      </div>
    </div>

    <a-alert
      v-if="active === 5 && sslSettings.type === '0' && port !== 80"
      :title="'Warning!'"
      type="error"
      :closable="false"
      center
    >
      {{ lang.autoSSLWarn }}
    </a-alert>

    <div
      v-if="active === 5"
      class="ctn"
    >
      <div class="desc">
        <h2>{{ lang.setSSL }}</h2>
        <div style="margin-top: 10px">
          {{ lang.setSSL }}
        </div>
      </div>
      <div class="form setup-form">
        <a-form
          :model="sslSettings"
          layout="vertical"
        >
          <a-form-item :label="lang.type">
            <a-select
              v-model="sslSettings.type"
              :placeholder="lang.ssl_auto"
              :disabled="dnsChecking"
            >
              <a-option
                :label="lang.ssl_auto"
                value="0"
              />
              <a-option
                :label="lang.ssl_manuallyf"
                value="1"
              />
            </a-select>
          </a-form-item>

          <a-form-item
            v-if="sslSettings.type === '0'"
            :label="lang.ssl_challenge_type"
          >
            <div class="challenge-row">
              <a-select
                v-model="sslSettings.challenge"
                :placeholder="lang.ssl_auto_http"
                :disabled="dnsChecking"
              >
                <a-option
                  :label="lang.ssl_auto_http"
                  value="http"
                />
                <a-option
                  :label="lang.ssl_auto_dns"
                  value="dns"
                />
              </a-select>
              <a-tooltip
                :content="lang.challenge_typ_desc"
                position="tl"
              >
                <span class="challenge-help">?</span>
              </a-tooltip>
            </div>
          </a-form-item>

          <a-form-item
            v-if="sslSettings.type === '1'"
            :label="lang.ssl_key_path"
          >
            <a-input
              v-model="sslSettings.key_path"
              placeholder="./config/ssl/private.key"
            />
          </a-form-item>

          <a-form-item
            v-if="sslSettings.type === '1'"
            :label="lang.ssl_crt_path"
          >
            <a-input
              v-model="sslSettings.crt_path"
              placeholder="./config/ssl/public.crt"
            />
          </a-form-item>
        </a-form>
      </div>
    </div>

    <div v-if="dnsChecking">
      <label>{{ lang.dns_desc }}</label>
      <a-table
        :data="sslSettings.paramsList"
        :columns="dnsColumns"
        :pagination="false"
        :loading="sslSettings.paramsList.length === 0"
        bordered
      >
        <template #value="{ record }">
          <a-tooltip
            v-if="record.tips !== ''"
            :content="record.tips"
            position="top"
          >
            <span>{{ record.value }}</span>
          </a-tooltip>
          <span v-else>{{ record.value }}</span>
        </template>
      </a-table>
    </div>

    <a-button
      id="next"
      type="primary"
      size="large"
      :loading="fullscreenLoading"
      @click="next"
    >
      {{ lang.next }}
    </a-button>

    <div
      v-if="fullscreenLoading"
      class="fullscreen-loading"
    >
      <a-spin
        :loading="true"
        :tip="waitDesc"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconPlus } from '@arco-design/web-vue/es/icon'
import axios from 'axios'
import lang from '@/i18n/i18n'
import { http } from '@/utils/axios'

interface DnsInfo {
  host: string
  type: string
  value: string
  ttl: string
  tips: string
}

const mysqlPlaceholder = 'root:12345@tcp(127.0.0.1:3306)/pmail?parseTime=True&loc=Local'
const pgPlaceholder = 'postgres://postgres:12345@127.0.0.1:5432/pmail?sslmode=disable'

const waitDesc = ref(lang.wait_desc)

const adminSettings = reactive({
  account: 'admin',
  password: '',
  password2: '',
  hadSeted: false
})

const dbSettings = reactive({
  type: 'sqlite',
  dsn: './config/pmail.db',
  lable: ''
})

const domainSettings = reactive({
  web_domain: '',
  smtp_domain: '',
  multi_domain: [] as string[]
})

const sslSettings = reactive({
  type: '0',
  challenge: 'http',
  key_path: './config/ssl/private.key',
  crt_path: './config/ssl/public.crt',
  paramsList: [] as DnsInfo[]
})

const active = ref(0)
const fullscreenLoading = ref(false)
const dnsChecking = ref(false)
const dnsInfos = ref<Record<string, DnsInfo[]>>({})
const port = ref(80)

const dnsColumns = [
  { title: 'HOSTNAME', dataIndex: 'host', slotName: 'host', width: 110 },
  { title: 'TYPE', dataIndex: 'type', width: 110 },
  { title: 'VALUE', dataIndex: 'value', slotName: 'value' },
  { title: 'TTL', dataIndex: 'ttl', width: 110 }
]

const addDomain = () => {
  domainSettings.multi_domain.push('')
}

const setPassword = () => {
  if (adminSettings.hadSeted) {
    active.value++
    getDomainConfig()
    return
  }

  if (adminSettings.password !== adminSettings.password2) {
    Message.error(lang.err_pwd_diff)
    return
  }
  http
    .post('/api/setup', {
      action: 'set',
      step: 'password',
      account: adminSettings.account,
      password: adminSettings.password
    })
    .then((res) => {
      if (res.errorNo !== 0) {
        Message.error(res.errorMsg)
      } else {
        active.value++
        getDomainConfig()
      }
    })
}

const getPassword = () => {
  http.post<string>('/api/setup', { action: 'get', step: 'password' }).then((res) => {
    if (res.errorNo !== 0) {
      Message.error(res.errorMsg)
      return
    }
    adminSettings.hadSeted = res.data !== ''
    if (adminSettings.hadSeted) {
      adminSettings.account = res.data
      adminSettings.password = '*******'
      adminSettings.password2 = '*******'
    }
  })
}

const getDbConfig = () => {
  http.post<{ db_type: string; db_dsn: string }>('/api/setup', { action: 'get', step: 'database' }).then((res) => {
    if (res.errorNo !== 0) {
      Message.error(res.errorMsg)
      return
    }
    dbSettings.type = res.data.db_type
    dbSettings.dsn = res.data.db_dsn
  })
}

const getDomainConfig = () => {
  http
    .post<{ web_domain: string; smtp_domain: string; domains: string[] }>('/api/setup', { action: 'get', step: 'domain' })
    .then((res) => {
      if (res.errorNo !== 0) {
        Message.error(res.errorMsg)
        return
      }
      domainSettings.web_domain = res.data.web_domain
      domainSettings.smtp_domain = res.data.smtp_domain
      domainSettings.multi_domain = res.data.domains ?? []
    })
}

const setDbConfig = () => {
  if (dbSettings.type === 'sqlite' && !dbSettings.dsn) {
    dbSettings.dsn = './config/pmail.db'
  } else if (!dbSettings.dsn) {
    Message.error(lang.err_db_dsn_empty)
    return
  }
  http
    .post('/api/setup', {
      action: 'set',
      step: 'database',
      db_type: dbSettings.type,
      db_dsn: dbSettings.dsn
    })
    .then((res) => {
      if (res.errorNo !== 0) {
        Message.error(res.errorMsg)
      } else {
        active.value++
        getPassword()
      }
    })
}

const getDNSConfig = () => {
  http.post<Record<string, DnsInfo[]>>('/api/setup', { action: 'get', step: 'dns' }).then((res) => {
    if (res.errorNo !== 0) {
      Message.error(res.errorMsg)
      return
    }
    dnsInfos.value = res.data ?? {}
  })
}

const getSSLConfig = () => {
  http.post<{ type: string; port: number }>('/api/setup', { action: 'get', step: 'ssl' }).then((res) => {
    if (res.errorNo !== 0) {
      Message.error(res.errorMsg)
      return
    }
    sslSettings.type = res.data.type
    if (sslSettings.type === '2') {
      sslSettings.type = '0'
      sslSettings.challenge = 'dns'
    }
    port.value = res.data.port
  })
}

const setSSLConfig = () => {
  fullscreenLoading.value = true

  let sslType = sslSettings.type
  if (sslType === '0' && sslSettings.challenge === 'dns') {
    sslType = '2'
  }

  http
    .post('/api/setup', {
      action: 'set',
      step: 'ssl',
      ssl_type: sslType,
      key_path: sslSettings.key_path,
      crt_path: sslSettings.crt_path
    })
    .then((res) => {
      if (res.errorNo !== 0) {
        fullscreenLoading.value = false
        Message.error(res.errorMsg)
        return
      }
      if (sslType === '2') {
        fullscreenLoading.value = false
        dnsChecking.value = true
        getSSLDNSParams()
      }
      checkStatus()
    })
}

const checkStatus = () => {
  axios
    .post('/api/ping', {})
    .then((res) => {
      if (res.data.errorNo !== 0) {
        setTimeout(checkStatus, 1000)
        return
      }
      window.location.href =
        sslSettings.type === '1' ? `http://${domainSettings.web_domain}` : `https://${domainSettings.web_domain}`
    })
    .catch(() => {
      setTimeout(checkStatus, 1000)
    })
}

const setDomainConfig = () => {
  http
    .post('/api/setup', {
      action: 'set',
      step: 'domain',
      web_domain: domainSettings.web_domain,
      smtp_domain: domainSettings.smtp_domain,
      multi_domain: domainSettings.multi_domain.join(',')
    })
    .then((res) => {
      if (res.errorNo !== 0) {
        Message.error(res.errorMsg)
      } else {
        active.value++
        getDNSConfig()
      }
    })
}

const getSSLDNSParams = () => {
  http.post<DnsInfo[]>('/api/setup', { action: 'getParams', step: 'ssl' }).then((res) => {
    if (res.errorNo !== 0) {
      Message.error(res.errorMsg)
      return
    }
    sslSettings.paramsList = res.data ?? []
  })

  if (sslSettings.paramsList.length === 0) {
    setTimeout(getSSLDNSParams, 1000)
  }
}

const next = () => {
  switch (active.value) {
    case 0:
      active.value++
      getDbConfig()
      break
    case 1:
      setDbConfig()
      break
    case 2:
      setPassword()
      break
    case 3:
      setDomainConfig()
      break
    case 4:
      getSSLConfig()
      active.value++
      break
    case 5:
      if (dnsChecking.value) {
        fullscreenLoading.value = true
        waitDesc.value = lang.dns_challenge_wait
      } else {
        setSSLConfig()
      }
      break
  }
}
</script>

<style scoped>
#main {
  width: 100%;
  height: 100%;
  background:
    radial-gradient(circle at 10% 10%, rgba(0, 113, 227, 0.09) 0%, transparent 34%),
    var(--pm-bg-primary);
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 26px;
  overflow-y: auto;
}

.desc {
  padding-right: 20px;
  margin-bottom: 18px;
}

.desc h2 {
  font-size: 32px;
  font-weight: 600;
  letter-spacing: -0.02em;
}

#status {
  padding: 18px 22px;
  border-radius: var(--pm-radius-xl);
  background: var(--pm-surface-glass);
  border: 1px solid var(--pm-glass-border);
  box-shadow: var(--pm-shadow-sm);
  backdrop-filter: blur(12px);
  animation: pm-rise-in 0.38s var(--pm-ease-out);
}

.ctn {
  display: flex;
  justify-content: center;
  align-items: flex-start;
  gap: 24px;
  padding: 24px;
  border-radius: var(--pm-radius-xl);
  background: var(--pm-surface-glass);
  border: 1px solid var(--pm-glass-border);
  box-shadow: var(--pm-shadow-md);
  animation: pm-rise-in 0.46s var(--pm-ease-out);
}

.ctn_s {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 22px;
  border-radius: var(--pm-radius-xl);
  background: var(--pm-surface-glass);
  border: 1px solid var(--pm-glass-border);
  box-shadow: var(--pm-shadow-md);
  animation: pm-rise-in 0.46s var(--pm-ease-out);
}

.setup-form {
  width: 400px;
}

.dns-form {
  width: 100%;
}

.multi-domain {
  display: flex;
  align-items: center;
  gap: 8px;
}

.multi-domain-input {
  margin-top: 8px;
}

.challenge-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.challenge-row :deep(.arco-select) {
  flex: 1;
}

.challenge-help {
  font-size: 18px;
  font-weight: bolder;
  cursor: help;
}

#next {
  align-self: flex-end;
}

.fullscreen-loading {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.35);
  z-index: 1000;
}
</style>
