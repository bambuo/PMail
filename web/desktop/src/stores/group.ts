import { ref, type Ref } from 'vue'
import { defineStore } from 'pinia'
import lang from '@/i18n/i18n'

const useGroupStore = defineStore('group', () => {
  const tag: Ref<string> = ref('')
  const name: Ref<string> = ref(lang.inbox)
  return { tag, name }
})

export default useGroupStore
