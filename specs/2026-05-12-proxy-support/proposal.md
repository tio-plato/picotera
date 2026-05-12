# Proxy Support for Gateway Outbound Requests

给这个项目增加走代理的功能，主要是网关向外请求，需要遵守 http_proxy / https_proxy / all_proxy / no_proxy 这样的（只是举例）标准环境变量。然后再给上游增加一个代理 URL 字段，如果有这个代理字段的话，可以覆盖默认的环境变量，也可以禁止特定上游走代理。
