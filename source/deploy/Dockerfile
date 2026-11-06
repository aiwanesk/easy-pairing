# pairing_easy — online platform image.
# Build from the source/ folder:  docker build -f deploy/Dockerfile -t pairing_easy .
FROM node:22-alpine AS front
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine AS back
WORKDIR /src/backend
COPY backend/ ./
COPY --from=front /src/frontend/dist ./internal/web/dist
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags "-s -w -X pairingeasy/internal/api.Version=${VERSION}" -o /pairing_easy ./cmd/pairing_easy

# bbpPairings (Linux), compiled from the official sources.
FROM alpine:3.22 AS bbp
RUN apk add --no-cache git g++ make
RUN git clone --depth 1 https://github.com/BieremaBoyzProgramming/bbpPairings.git /bbp && make -C /bbp -j"$(nproc)" && cp /bbp/bbpPairings.exe /bbpPairings

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata libstdc++ && adduser -D -H -u 10001 pe && mkdir /data && chown pe /data
COPY --from=back /pairing_easy /usr/local/bin/pairing_easy
COPY --from=bbp /bbpPairings /usr/local/bin/bbpPairings
USER pe
ENV PAIRING_EASY_LISTEN=:8080 PAIRING_EASY_DATA=/data TZ=Europe/Paris
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["pairing_easy", "-bbp", "/usr/local/bin/bbpPairings"]
