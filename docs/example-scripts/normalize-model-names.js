// 拉取模型时自动归一化模型名，移除 / 的前缀，移除常见的免费后缀，转为小写
picotera.hooks.rewriteProviderModels.tap('rewrite-name', function({}, models) {
  if (!models) return
  for (const model of models) {
    const localName = model.model ?? model.upstreamModelName
    const newName = localName.toLowerCase()
      .replace(/[-:]free/, '')
      .replace(/^.*\//, '')
    if (localName !== newName) {
      if (!model.upstreamModelName) {
        model.upstreamModelName = model.model
      }
      model.model = newName
    }
  }
  return models
})
