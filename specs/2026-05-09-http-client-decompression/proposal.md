# HTTP Client 自动解压

给项目中的 http Client 增加自动解压功能：当识别到 Content-Encoding 的时候，自动检测 gzip/br/zstd 并自动解压。注意：返回给客户端的不需要解压，拷贝一份解压出来自己用，包括 response artifacts、llmbridge 聚合、记录 usage、检测 TTFT 等等内部用的所有响应体都是用解压后的部分，只有返回给客户端的不要解压。
