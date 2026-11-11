FROM docker.io/library/golang:alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /localmap ./cmd/localmap

# Alpine's chromium package can't do headless software WebGL (its bundled
# SwiftShader/ANGLE Vulkan backend fails to init without a real GPU), which
# maplibre-gl requires. Debian's chromium works with --enable-unsafe-swiftshader.
FROM docker.io/library/debian:12-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium fonts-liberation imagemagick ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /localmap /usr/local/bin/localmap
ENV LM_CACHE_DIR=/tmp \
    LM_RENDERER=browser \
    CHROME_PATH=/usr/bin/chromium
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/localmap"]
