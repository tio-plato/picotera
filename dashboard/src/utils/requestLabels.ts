export function finishReasonLabel(reason: number | undefined | null): string {
  switch (reason) {
    case 1:
      return '内部错误'
    case 2:
      return '已取消'
    case 3:
      return '正常结束'
    case 4:
      return '请求头超时'
    case 5:
      return '读取超时'
    case 6:
      return '流式错误'
    case 7:
      return '控制台打断'
    default:
      return reason === undefined || reason === null ? '—' : String(reason)
  }
}
