import { defineStore } from 'pinia'
import { http } from '@/utils/axios'
import type { UserInfo } from '@/types/api'

interface GlobalStatusState {
  userInfos: Partial<UserInfo>
  mobileDrawerVisible: boolean
  settingsDrawerVisible: boolean
}

export const useGlobalStatusStore = defineStore('useGlobalStatusStore', {
  state: (): GlobalStatusState => ({
    userInfos: {},
    mobileDrawerVisible: false,
    settingsDrawerVisible: false
  }),
  getters: {
    isLogin(state): boolean {
      return Object.keys(state.userInfos).length !== 0
    }
  },
  actions: {
    init(callback: () => void) {
      http.post<Partial<UserInfo>>('/api/user/info', {}).then((res) => {
        if (res.errorNo === 0) {
          Object.assign(this.userInfos, res.data)
          callback()
        }
      })
    }
  }
})
