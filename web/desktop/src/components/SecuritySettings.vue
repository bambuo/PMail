<template>
  <div class="settings-card">
    <div class="settings-header">
      <h3>{{ lang.modify_pwd }}</h3>
      <p class="settings-desc">
        {{ lang.modify_pwd_desc }}
      </p>
    </div>

    <div class="settings-body">
      <a-form
        ref="formRef"
        :model="ruleForm"
        :rules="rules"
        layout="vertical"
        class="settings-form"
      >
        <a-form-item
          :label="lang.modify_pwd"
          field="new_pwd"
        >
          <a-input-password
            v-model="ruleForm.new_pwd"
            :placeholder="lang.enter_new_pwd"
          />
        </a-form-item>

        <a-form-item
          :label="lang.enter_again"
          field="new_pwd2"
        >
          <a-input-password
            v-model="ruleForm.new_pwd2"
            :placeholder="lang.confirm_new_pwd"
          />
        </a-form-item>

        <div class="form-actions">
          <a-button
            type="primary"
            class="submit-btn"
            @click="submit"
          >
            {{ lang.submit }}
          </a-button>
        </div>
      </a-form>
    </div>

    <a-divider class="settings-divider" />

    <div class="settings-header">
      <h3 class="danger-text">
        {{ lang.logout }}
      </h3>
      <p class="settings-desc">
        {{ lang.logout_desc }}
      </p>
    </div>
    <div class="settings-body">
      <a-button
        status="danger"
        type="outline"
        class="logout-btn"
        @click="logout"
      >
        {{ lang.logout }}
      </a-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Notification, type FormInstance } from '@arco-design/web-vue'
import lang from '@/i18n/i18n'
import { http } from '@/utils/axios'

const formRef = ref<FormInstance>()

const ruleForm = reactive({
  new_pwd: '',
  new_pwd2: ''
})

const rules = reactive({
  new_pwd: [{ required: true, message: lang.err_required_pwd }],
  new_pwd2: [{ required: true, message: lang.err_required_pwd }]
})

const logout = () => {
  http.post('/api/logout', {}).then(() => {
    location.reload()
  })
}

const submit = async () => {
  const errors = await formRef.value?.validate()
  if (errors) return
  if (ruleForm.new_pwd !== ruleForm.new_pwd2) {
    Notification.error({
      title: 'Error',
      content: lang.err_pwd_diff
    })
    return
  }
  http.post<string>('/api/settings/modify_password', { password: ruleForm.new_pwd }).then((res) => {
    Notification[res.errorNo === 0 ? 'success' : 'error']({
      title: res.errorNo === 0 ? lang.succ : lang.fail,
      content: res.data
    })
  })
}
</script>

<style scoped>
.settings-card {
  padding: 0;
}

.settings-header {
  margin-bottom: 24px;
}

.settings-header h3 {
  font-size: 18px;
  font-weight: 600;
  color: var(--pm-text-primary);
  margin: 0 0 8px 0;
}

.danger-text {
  color: #ef4444 !important;
}

.settings-desc {
  font-size: 14px;
  color: var(--pm-text-secondary);
  margin: 0;
}

.settings-body {
  width: 100%;
}

.form-actions {
  margin-top: 24px;
}

.settings-divider {
  margin: 32px 0;
}
</style>
