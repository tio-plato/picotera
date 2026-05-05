# 请求详情 Deeplink

请求页面需要 deeplink，在点击展开某个请求详情的时候，URL 应该被 replace 成 `/requests/:requestId` 这样的路径（但是界面不跳转，只弹窗），关闭之后还原。而如果用户直接访问 `/requests/:requestId` 的话，那就相当于它全屏地看请求弹窗（最好是复用相同组件）。
