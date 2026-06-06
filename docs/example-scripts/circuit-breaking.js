// 模拟 axonhub 的渠道熔断功能
// 可能缺少一个 error 的 hook，用 beforeRequest 也能实现，就是逻辑稍微有点绕了
picotera.hooks.beforeRequest.tap('fuse', function ({
  provider: {
    id: providerId,
    name: providerName,
  },
  attempt: {
    currentRetryCount,
  },
}, input) {
  let errCount = picotera.kv.get(`fail:${providerId}`) ?? 0
  if (errCount >= 10) {
    // 60s 内失败 >10 次
    console.log(`渠道 ${providerName} 错误次数太多，熔断生效中`)
    return { next: true, delay: 0 } // 尝试下一个
  }
  if (currentRetryCount > 0) {
    errCount += 1
    console.log(`provider ${providerName} error count ${errCount}`)
    picotera.kv.setex(`fail:${providerId}`, 60, errCount)
  }
  return input
})
