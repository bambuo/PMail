export type ArcoTheme = 'light' | 'dark'

export function applyArcoTheme(theme: ArcoTheme): void {
  document.body.setAttribute('arco-theme', theme)
}

export function useSystemArcoTheme(): () => void {
  const media = window.matchMedia('(prefers-color-scheme: dark)')
  const update = () => applyArcoTheme(media.matches ? 'dark' : 'light')
  update()
  media.addEventListener('change', update)
  return () => media.removeEventListener('change', update)
}
