export interface ApiResponse<T = unknown> {
  errorNo: number
  errorMsg: string
  data: T
}

export interface UserInfo {
  account: string
  name: string
  is_admin: boolean
  domains: string[]
}

export interface LoginResult extends Partial<UserInfo> {
  mfaRequired?: boolean
  challengeToken?: string
}

export interface MFASetupData {
  secret: string
  url: string
  qrDataUrl: string
}
