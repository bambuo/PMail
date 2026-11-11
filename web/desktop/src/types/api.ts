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
