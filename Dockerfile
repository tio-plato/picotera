FROM node:25.8.1-trixie AS frontend-builder
COPY . /app
WORKDIR /app
RUN npm install -g pnpm && \
    pnpm install --frozen-lockfile && \
    pnpm --dir dashboard build

FROM golang:1.26.1-trixie AS backend-builder
COPY . /app
COPY --from=frontend-builder /app/dashboard/dist /app/dashboard/dist
WORKDIR /app
RUN find pkg/server/static/dist -mindepth 1 ! -name index.html ! -name .gitignore -delete && \
    cp -r dashboard/dist/. pkg/server/static/dist/ && \
    go build -o picotera ./cmd/picotera

FROM golang:1.26.1-trixie AS llmbridge-wasm-builder
COPY . /app
WORKDIR /app
RUN mkdir -p dist && \
    GOOS=wasip1 GOARCH=wasm go build -trimpath -ldflags=-buildid= -buildmode=c-shared -o dist/llmbridge.wasm ./cmd/llmbridge-wasm

FROM gcr.io/distroless/base-debian13 AS runtime
COPY --from=backend-builder /app/picotera /app/picotera
WORKDIR /app
ENTRYPOINT ["/app/picotera"]

FROM runtime AS runtime-lgpl
COPY --from=llmbridge-wasm-builder /app/dist/llmbridge.wasm /app/llmbridge.wasm
ENV PICOTERA_LLMBRIDGE_WASM_PATH=/app/llmbridge.wasm
