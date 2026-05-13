# Proposal: llmbridge WASM Isolation

给 llmbridge 这个包单独做成 wasm 编译，然后启动时加载进 golang 本体里面。因为 llmbridge 使用了 lgpl 代码，隔离一下。

Revision: the Go binary is built once and never embeds wasm. The LGPL Docker target reuses that same binary layer, adds `llmbridge.wasm` as a separate file, and sets the wasm path. Operators can also mount an external wasm file into the default image and enable bridge with the same path setting.
