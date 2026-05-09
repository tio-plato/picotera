// 重试规则
picotera.hooks.beforeRequest.tap("retry", function (ctx, input) {
  return {
    next: !(ctx.currentRetryCount < 2 && ctx.totalAttemptCount < 5),
    delay: ctx.currentRetryCount * 500,
  };
}, 0);
