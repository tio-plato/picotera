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

FROM gcr.io/distroless/base-debian13
COPY --from=backend-builder /app/picotera /app/picotera
WORKDIR /app
ENTRYPOINT ["/app/picotera"]
