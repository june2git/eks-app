# ============================================================
# Stage 1: Build Stage (빌드 단계)
# ============================================================
# 목적: Go 애플리케이션을 컴파일하여 실행 가능한 바이너리 생성
# 베이스 이미지: golang:1.23-alpine (Go 1.23 + Alpine Linux)
# - Alpine Linux: 경량 Linux 배포판 (~5MB)
# - AS builder: 이 단계를 'builder'로 명명 (나중에 참조용)
FROM golang:1.23-alpine AS builder

# 빌드에 필요한 패키지 설치
# - git: Go 모듈 의존성 다운로드 시 필요
# - ca-certificates: HTTPS 통신을 위한 SSL 인증서
# - tzdata: 타임존 데이터 (시간대 설정용)
# - --no-cache: 패키지 인덱스를 저장하지 않아 이미지 크기 감소
RUN apk add --no-cache git ca-certificates tzdata

# 작업 디렉토리 설정
# 이후 모든 명령은 /app 디렉토리에서 실행됨
WORKDIR /app

# Go 모듈 의존성 파일 복사 및 다운로드
# go.mod, go.sum만 먼저 복사하는 이유:
# - Docker 레이어 캐싱 최적화
# - 소스 코드 변경 시 의존성 재다운로드 방지
# - 의존성이 변경되지 않으면 이 레이어는 캐시 사용
COPY go.mod go.sum ./
RUN go mod download

# 애플리케이션 소스 코드 전체 복사
# . (현재 디렉토리) → /app (컨테이너 내부)
COPY . .

# Go 애플리케이션 바이너리 빌드
# CGO_ENABLED=0: C 라이브러리 의존성 제거 (완전한 정적 바이너리)
# GOOS=linux: 리눅스용 바이너리 생성
# GOARCH=amd64: x86_64 아키텍처용 바이너리 생성
# -ldflags="-w -s": 디버그 정보 제거로 바이너리 크기 감소
#   -w: DWARF 디버깅 정보 제거
#   -s: 심볼 테이블 제거
# -o: 출력 파일 경로 지정
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/kafka-consumer \
    main.go

# ============================================================
# Stage 2: Runtime Stage (실행 단계)
# ============================================================
# 목적: 최소한의 런타임 환경으로 최종 이미지 크기 최소화
# 베이스 이미지: alpine:3.19 (경량 Linux, 빌드 도구 제외)
# Multi-stage build 장점:
# - 빌드 도구(Go 컴파일러 등)는 최종 이미지에 포함되지 않음
# - 최종 이미지 크기: ~20MB (vs 빌드 이미지 ~300MB)
FROM alpine:3.19

# 런타임에 필요한 최소한의 패키지 설치
# - ca-certificates: HTTPS 통신용 SSL 인증서 (Kafka 브로커 TLS 연결)
# - tzdata: 타임존 데이터 (로그 시간 표시용)
# - --no-cache: 패키지 캐시 저장 안 함 (이미지 크기 절약)
RUN apk add --no-cache ca-certificates tzdata

# 타임존 환경 변수 설정
# Asia/Seoul: 한국 표준시 (KST, UTC+9)
# 로그 타임스탬프가 한국 시간으로 표시됨
ENV TZ=Asia/Seoul

# 작업 디렉토리 설정
# 애플리케이션 바이너리가 위치할 디렉토리
WORKDIR /app

# Build Stage에서 빌드된 바이너리만 복사
# --from=builder: 첫 번째 스테이지에서 파일 가져오기
# 소스 코드, Go 컴파일러 등은 복사되지 않음 (이미지 크기 최소화)
COPY --from=builder /app/kafka-consumer /app/kafka-consumer

# 보안: 비루트 사용자 생성 및 권한 설정
# 1. addgroup: appuser 그룹 생성 (GID: 1000)
# 2. adduser: appuser 사용자 생성 (UID: 1000)
#    -D: 패스워드 없이 생성
#    -u 1000: UID 지정
#    -G appuser: appuser 그룹에 소속
# 3. chown: /app 디렉토리 소유권을 appuser로 변경
# 이유: root로 실행 시 보안 취약점 발생 가능
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

# 이후 모든 명령은 appuser 권한으로 실행
# Kubernetes securityContext와 일치 (runAsUser: 1000)
USER appuser

# 컨테이너가 사용할 포트 문서화
# 8080: HTTP API 서버 포트 (실제로 포트를 열지는 않음, 문서화 목적)
# Kubernetes Service가 이 포트로 트래픽 전달
EXPOSE 8080

# 헬스체크 설정 (컨테이너 상태 모니터링)
# --interval=30s: 30초마다 헬스체크 실행
# --timeout=3s: 3초 내에 응답 없으면 실패
# --start-period=5s: 컨테이너 시작 후 5초간 실패 무시 (워밍업 시간)
# --retries=3: 3번 연속 실패 시 unhealthy 상태로 전환
# CMD: /health 엔드포인트 호출하여 200 OK 응답 확인
# wget 사용 이유: curl이 Alpine에 기본 포함되지 않음 (크기 절약)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 컨테이너 시작 시 실행할 명령
# JSON 배열 형식: exec form (권장)
# - PID 1로 직접 실행 (셸을 거치지 않음)
# - 시그널(SIGTERM) 처리가 정상적으로 동작
# - Graceful shutdown 지원
CMD ["/app/kafka-consumer"]

