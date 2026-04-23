import { reactive } from 'vue'

interface ConfirmOptions {
  message: string
  accept: () => Promise<void> | void
}

interface ConfirmState {
  visible: boolean
  message: string
  onAccept: (() => Promise<void> | void) | null
  accepting: boolean
}

export const confirmState = reactive<ConfirmState>({
  visible: false,
  message: '',
  onAccept: null,
  accepting: false,
})

export function useConfirm() {
  function require(opts: ConfirmOptions) {
    confirmState.visible = true
    confirmState.message = opts.message
    confirmState.onAccept = opts.accept
    confirmState.accepting = false
  }

  return { require }
}
