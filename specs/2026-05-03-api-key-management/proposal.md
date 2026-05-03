# API Key 管理

增加 API Key 管理。相关界面和接口参考现有，不赘述。需要有禁用的功能。加了之后所有网关的 API 就都需要用 API Key 鉴权了（原有的管理面板 API 不需要）。API Key 不用脱敏，可以在前端复制，甚至改成自定义的都行，默认格式是 sk_pt_xxxx 这样的。hook 也需要能读到 api key 的信息（名字、annotations 啥的）。
