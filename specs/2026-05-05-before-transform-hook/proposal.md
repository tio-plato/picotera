原本在 unified 网关里面有个根据 ah.outbound.xx annotations 干活的一段逻辑，这个好像不是很科学，应该新增一个 hook 叫做 `beforeTransform`，由这个 hook 返回或者改写 outbound 的 type 和 config。
