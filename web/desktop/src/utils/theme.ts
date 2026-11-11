export type ArcoTheme = 'light' | 'dark'

const THEME_STORAGE_KEY = 'pmail-theme'

export function applyArcoTheme(theme: ArcoTheme): void {
  document.body.setAttribute('arco-theme', theme)
  document.documentElement.setAttribute('data-theme', theme)
}

export function getStoredTheme(): ArcoTheme | null {
  const value = localStorage.getItem(THEME_STORAGE_KEY)
  return value === 'light' || value === 'dark' ? value : null
}

export function setTheme(theme: ArcoTheme): void {
  localStorage.setItem(THEME_STORAGE_KEY, theme)
  applyArcoTheme(theme)
}

export function useSystemArcoTheme(): () => void {
  const media = window.matchMedia('(prefers-color-scheme: dark)')
  // 已手动选择过主题时以存储值为准，否则跟随系统
  const update = () => applyArcoTheme(getStoredTheme() ?? (media.matches ? 'dark' : 'light'))
  update()
  media.addEventListener('change', update)
  return () => media.removeEventListener('change', update)
}
