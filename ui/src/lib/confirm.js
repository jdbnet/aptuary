import { ref } from 'vue'

const open = ref(false)
const title = ref('')
const message = ref('')
let resolveFn = null

export function confirm({ title: t, message: m }) {
  title.value = t
  message.value = m
  open.value = true
  return new Promise((resolve) => {
    resolveFn = resolve
  })
}

export function useConfirm() {
  return { open, title, message, accept: () => { open.value = false; resolveFn?.(true) }, cancel: () => { open.value = false; resolveFn?.(false) } }
}
