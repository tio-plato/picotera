# JSON 查看器与 SSE Events 视图

前端查看 json 的地方，比如请求-原始请求-body，原始响应-聚合，这些地方增加 json editor 查看器，用 https://github.com/josdejong/svelte-jsoneditor 这个库的 readonly mode ，显示为 tree 就行了。前端采用 content-type 探测 json 类型并渲染成查看器，同时也增加选项切换原始 or JSON。需要注意的是 SSE 的 body，在原本的聚合/渲染/Raw 之外增加一个 Events 渲染，点击之后可以按 event 查看渲染好的内容，其中内容如果是 json 则渲染成 json editor，不是则继续用代码块。
