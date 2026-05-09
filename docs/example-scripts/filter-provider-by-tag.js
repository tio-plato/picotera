// 限制 API Key 只能访问打了标签的渠道
// 用法：
// 1. 在 API Key 上添加标注 `usage` = `foo,bar`
// 2. 在渠道或者模型上添加标注 `usage.foo` = `yes`

picotera.hooks.sortProviders.tap('filter-by-usage', function (ctx, input) {
  const { apiKey } = ctx

  if (!apiKey.annotations.usage) {
    throw new Error('API Key is not assigned any usage')
  }
  
  const apiKeyUsages = `${apiKey.annotations.usage}`.split(',')
  input.providers = input.providers.filter(p => {
    for (const apiKeyUsage of apiKeyUsages) {
      const providerUsageAnnotation = p.annotations[`usage.${apiKeyUsage}`]
      if (['yes', 'y', 'true', 'ok', '1'].includes(providerUsageAnnotation)) {
        return true
      }
    }
    console.log(`${p.provider.name}::${p.mpe.modelName} usage not matching api key`, p.annotations, apiKeyUsages)
    return false
  })

  return input
})
