// 重试规则
picotera.hooks.beforeRequest.tap("retry", function (ctx, input) {
  return {
    next: !(ctx.attempt.currentRetryCount < 2 && ctx.attempt.totalAttemptCount < 5),
    delay: ctx.attempt.currentRetryCount * 500,
  };
}, 0);
