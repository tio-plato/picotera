// 对所有 deepseek-v4 开头的模型，请求 /api/picotera/v1/chat/completions 时，使用 axonhub deepseek 驱动重写
// beforeTransform 仅在 /api/picotera/v1/ 下生效
picotera.hooks.beforeTransform.tap('deepseek', function (ctx, input) {
  const modelName = ctx.routedModel?.name ?? ''
  const isDeepseekV4Model = modelName.startsWith('deepseek-v4')
  if (ctx.upstreamFormat === 'openaiChatCompletions' && isDeepseekV4Model) {
    input.type = 'deepseek'
  }
  return input
})
