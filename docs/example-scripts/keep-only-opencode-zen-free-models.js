// 给渠道打上 `rewrite-models` = `opencode-free` 的标注
// 这样获取到的模型就只有免费的
picotera.hooks.rewriteProviderModels.tap('opencode-zen-free', function({ provider: { annotations } }, models) {
  if (annotations['rewrite-models'] !== 'opencode-free') {
    return
  }
  const newModels = []
  for (const model of models) {
    const name = model.upstreamModelName ?? model.model
    if (name === 'big-pickle' || name.endsWith('-free')) {
      newModels.push(model)
    }
  }
  return newModels
})
