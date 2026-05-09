// 特定 api key cc 改写为国模
picotera.hooks.rewriteModel.tap("cc-china", function ({ apiKey }, input) {
  const useChineseModels = apiKey.annotations['cn-models']
  if (useChineseModels) {
    if (input.startsWith('claude-haiku-')) {
      return useChineseModels === 'dpsk' ? 'deepseek-v4-flash' : 'minimax-m2.7'
    } else {
      return useChineseModels === 'dpsk' ? 'deepseek-v4-pro' : 'glm-5.1'
    }
  }
  return input
})
