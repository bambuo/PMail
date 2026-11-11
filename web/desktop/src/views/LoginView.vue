<template>
  <div class="login-wrapper">
    <div class="login-container">
      <div class="login-left">
        <div class="brand-info">
          <h1>PMail</h1>
        </div>
      </div>
      <div class="login-right">
        <div class="login-form-box">
          <h2>{{ lang.login }}</h2>
          <p class="subtitle">
            {{ lang.login_subtitle }}
          </p>

          <a-form
            v-if="!mfaRequired"
            :model="form"
            layout="vertical"
            class="auth-form"
            @keyup.enter="onSubmit"
          >
            <a-form-item :label="lang.account">
              <a-input
                v-model="form.account"
                :placeholder="lang.account"
                size="large"
              />
            </a-form-item>
            <a-form-item :label="lang.password">
              <a-input-password
                v-model="form.password"
                :placeholder="lang.password"
                size="large"
              />
            </a-form-item>
            <div class="submit-action">
              <a-button
                type="primary"
                size="large"
                long
                class="login-btn"
                :loading="loading"
                @click="onSubmit"
              >
                {{ lang.login }}
              </a-button>
            </div>
          </a-form>

          <div
            v-else
            class="mfa-login"
          >
            <a-form
              :model="{ code: mfaCode }"
              layout="vertical"
              @keyup.enter="onVerifyMfa"
            >
              <a-form-item :label="lang.mfa_login_title">
                <a-input
                  v-model="mfaCode"
                  :placeholder="lang.mfa_code"
                  size="large"
                  autofocus
                />
              </a-form-item>
            </a-form>
            <p class="mfa-desc">
              {{ lang.mfa_login_desc }}
            </p>
            <div class="submit-action">
              <a-button
                type="primary"
                size="large"
                long
                class="login-btn"
                :loading="mfaLoading"
                @click="onVerifyMfa"
              >
                {{ lang.mfa_verify }}
              </a-button>
            </div>
            <a-button
              type="text"
              long
              class="mfa-back"
              @click="resetMfa"
            >
              {{ lang.mfa_back }}
            </a-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { router } from '@/router'
import lang from '@/i18n/i18n'
import { http } from '@/utils/axios'
import { useGlobalStatusStore } from '@/stores/useGlobalStatusStore'
import type { LoginResult, UserInfo } from '@/types/api'

const globalStatus = useGlobalStatusStore()
const loading = ref(false)
const mfaRequired = ref(false)
const challengeToken = ref('')
const mfaCode = ref('')
const mfaLoading = ref(false)

const form = reactive({
  account: '',
  password: ''
})

const onSubmit = () => {
  if (!form.account || !form.password) {
    Message.warning(lang.login_fill_required)
    return
  }
  loading.value = true
  http
    .post<LoginResult>('/api/login', form)
    .then((res) => {
      loading.value = false
      if (res.errorNo !== 0) {
        Message.error(res.errorMsg)
        return
      }
      if (res.data.mfaRequired) {
        mfaRequired.value = true
        challengeToken.value = res.data.challengeToken ?? ''
        mfaCode.value = ''
        return
      }
      finishLogin(res.data)
    })
    .catch(() => {
      loading.value = false
    })
}

const onVerifyMfa = () => {
  if (!mfaCode.value.trim()) {
    Message.warning(lang.mfa_code)
    return
  }
  mfaLoading.value = true
  http
    .post<Partial<UserInfo>>('/api/login/mfa', {
      challengeToken: challengeToken.value,
      code: mfaCode.value.trim()
    })
    .then((res) => {
      mfaLoading.value = false
      if (res.errorNo !== 0) {
        Message.error(res.errorMsg || lang.mfa_verify_failed)
        return
      }
      finishLogin(res.data)
    })
    .catch(() => {
      mfaLoading.value = false
    })
}

const finishLogin = (user: Partial<UserInfo>) => {
  Object.assign(globalStatus.userInfos, user)
  router.replace({
    path: '/',
    query: {
      redirect: router.currentRoute.value.fullPath
    }
  })
}

const resetMfa = () => {
  mfaRequired.value = false
  challengeToken.value = ''
  mfaCode.value = ''
}
</script>

<style scoped>
.login-wrapper {
  width: 100vw;
  height: 100vh;
  background:
    radial-gradient(circle at 15% 20%, rgba(0, 113, 227, 0.1) 0%, transparent 36%),
    radial-gradient(circle at 85% 80%, rgba(0, 113, 227, 0.12) 0%, transparent 34%),
    var(--pm-bg-primary);
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 24px;
}

.login-container {
  width: 100%;
  max-width: 1080px;
  min-height: 640px;
  background: var(--pm-surface-glass);
  border-radius: var(--pm-radius-xl);
  box-shadow: var(--pm-shadow-lg);
  border: 1px solid var(--pm-glass-border);
  backdrop-filter: blur(18px);
  display: flex;
  overflow: hidden;
  animation: pm-rise-in 0.55s var(--pm-ease-out);
}

.login-left {
  flex: 1;
  background: linear-gradient(145deg, #0f6ddf 0%, #3f8ef7 55%, #9ec9ff 100%);
  padding: 72px 64px;
  color: #fff;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.brand-info h1 {
  font-size: 56px;
  font-weight: 600;
  margin-bottom: 14px;
  letter-spacing: -0.03em;
  animation: pm-rise-in 0.58s var(--pm-ease-out);
}

.login-right {
  flex: 1;
  padding: 64px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  background: linear-gradient(180deg, var(--pm-login-right-grad-start), var(--pm-login-right-grad-end));
}

.login-form-box {
  width: 100%;
  max-width: 400px;
  margin: 0 auto;
}

.login-form-box h2 {
  font-size: 34px;
  font-weight: 600;
  color: var(--pm-text-primary);
  margin-bottom: 8px;
  letter-spacing: -0.02em;
}

.subtitle {
  color: var(--pm-text-secondary);
  margin-bottom: 30px;
  font-size: 14px;
}

.submit-action {
  margin-top: 34px;
}

.mfa-desc {
  color: var(--pm-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.mfa-back {
  margin-top: 8px;
  color: var(--pm-text-secondary);
}

.login-btn {
  width: 100%;
  height: 50px;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 0.02em;
}

@media (max-width: 768px) {
  .login-container {
    flex-direction: column;
    height: auto;
    min-height: 0;
    border-radius: var(--pm-radius-lg);
  }
  .login-left {
    padding: 38px 28px;
    align-items: center;
    text-align: center;
  }
  .brand-info h1 {
    font-size: 38px;
  }
  .login-right {
    padding: 36px 20px;
  }
}
</style>
