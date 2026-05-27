FROM node:25.8.1-trixie AS frontend-builder
COPY . /app
WORKDIR /app
RUN npm install -g pnpm@10 && \
    pnpm install --frozen-lockfile && \
    pnpm --dir dashboard build

FROM golang:1.26.1-trixie AS backend-builder
COPY . /app
COPY --from=frontend-builder /app/dashboard/dist /app/dashboard/dist
WORKDIR /app
RUN find pkg/server/static/dist -mindepth 1 ! -name index.html ! -name .gitignore -delete && \
    cp -r dashboard/dist/. pkg/server/static/dist/ && \
    go build -o picotera ./cmd/picotera

FROM tinygo/tinygo:0.41.1 AS llmbridge-wasm-builder
USER root:root
COPY . /app
WORKDIR /app
RUN mkdir -p dist && \
    tinygo build -tags tinygo -target=wasi -scheduler=none -panic=print -opt=z -buildmode=c-shared -o dist/llmbridge.wasm ./cmd/llmbridge-wasm && \
    go build -o dist/picotera ./cmd/picotera && \
    PICOTERA_LLMBRIDGE_WASM_PATH=/app/dist/llmbridge.wasm /app/dist/picotera precompile-llmbridge-wasm

FROM gcr.io/distroless/base-debian13 AS runtime
COPY LICENSE /app/LICENSE
COPY --from=backend-builder /app/picotera /app/picotera
COPY --from=llmbridge-wasm-builder /app/dist/llmbridge.wasm /app/llmbridge.wasm
COPY --from=llmbridge-wasm-builder /app/dist/llmbridge.wasm.cache /app/llmbridge.wasm.cache
COPY THIRD_PARTY_NOTICES.md /app/THIRD_PARTY_NOTICES.md
ENV PICOTERA_LLMBRIDGE_WASM_PATH=/app/llmbridge.wasm
WORKDIR /app
ENTRYPOINT ["/app/picotera"]
