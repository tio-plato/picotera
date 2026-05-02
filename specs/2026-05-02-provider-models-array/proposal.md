# Proposal

重构 providers 的 model 字段。

现在是一个 `Record<string, {}>` 这样的结构，key 是 model name ，但是，这样对同一个内部模型 id 而言，没办法有两个模型了，所以要改成数组。数组内字段还和以前一样，只是多一个 `model` 作为 key 。其它引用的地方顺便改了。

如果能写个 SQL 迁移兼容迁移旧数据就最好，迁移不了的话清空就行。
