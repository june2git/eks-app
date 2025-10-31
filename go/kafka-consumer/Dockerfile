# Build stage
FROM golang:1.22-alpine AS builder

# 빌드에 필요한 패키지 설치
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Go 모듈 의존성 복사 및 다운로드
COPY go.mod go.sum ./
RUN go mod download

# 소스 코드 복사
COPY . .

# 바이너리 빌드
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/kafka-consumer \
    main.go

# Runtime stage
FROM alpine:3.19

# 런타임에 필요한 패키지 설치
RUN apk add --no-cache ca-certificates tzdata

# 타임존 설정
ENV TZ=Asia/Seoul

# 작업 디렉토리 설정
WORKDIR /app

# 빌드된 바이너리 복사
COPY --from=builder /app/kafka-consumer /app/kafka-consumer

# 비루트 사용자 생성 및 권한 설정
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# 포트 노출
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 애플리케이션 실행
CMD ["/app/kafka-consumer"]

