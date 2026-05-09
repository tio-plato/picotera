// 拉取 zenmux 模型的时候，根据供应商自动限定模型的 endpoint
picotera.hooks.rewriteProviderModels.tap('zenmux', function({ provider: { annotations }}, models) {
  if (!models) return
  const isZenMux = annotations['rewrite-models'] === 'zenmux'
  if (!isZenMux) return
  for (const model of models) {
    const name = model.upstreamModelName ?? model.model
    if (name.startsWith('anthropic/')) {
      model.endpoints = [
        '/v1/messages',
      ]
      model.upstreamModelName = name
      model.model = name.replace(/\./g, '-')
    } else if (name.startsWith('google/')) {
      model.endpoints = [
        '/v1beta/models/{model}:generateContent',
        '/v1beta/models/{model}:streamGenerateContent'
      ]
    } else {
      model.endpoints = [
        '/v1/chat/completions'
      ]
    }
  }
  return models
}, 100)
