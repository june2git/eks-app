# Kafka Producer (Go)

Kafka 메시지를 전송하는 Go 기반 Producer 애플리케이션

## 🚀 기능

- ✅ HTTP API를 통한 메시지 전송
- ✅ 단일 메시지 & 배치 메시지 지원
- ✅ Health check 엔드포인트
- ✅ Graceful shutdown
- ✅ Snappy 압축
- ✅ 자동 재시도 (최대 5회)
- ✅ 모든 ISR 확인 (데이터 안정성)

## 📋 API 엔드포인트

### 1. Health Check
```bash
GET /health
```

### 2. 메시지 전송
```bash
POST /send
Content-Type: application/json

{
  "key": "user-123",
  "value": "Hello Kafka!",
  "metadata": {
    "source": "api",
    "version": "1.0"
  }
}
```

### 3. 배치 메시지 전송
```bash
POST /send/batch
Content-Type: application/json

[
  {
    "key": "user-1",
    "value": "Message 1"
  },
  {
    "key": "user-2",
    "value": "Message 2"
  }
]
```

## 🔧 환경 변수

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `KAFKA_BROKERS` | `kafka-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092` | Kafka 브로커 주소 |
| `KAFKA_TOPIC` | `demo-events` | 메시지를 전송할 토픽 |
| `PORT` | `8080` | HTTP 서버 포트 |

## 🏃 로컬 실행

### 1. 의존성 설치
```bash
go mod download
```

### 2. 애플리케이션 실행
```bash
# 기본 설정으로 실행
go run main.go

# 환경 변수 설정하여 실행
KAFKA_BROKERS=localhost:9092 KAFKA_TOPIC=test-topic go run main.go
```

### 3. 메시지 전송 테스트
```bash
# 단일 메시지
curl -X POST http://localhost:8080/send \
  -H "Content-Type: application/json" \
  -d '{"key":"test-1","value":"Hello from Producer!"}'

# 배치 메시지
curl -X POST http://localhost:8080/send/batch \
  -H "Content-Type: application/json" \
  -d '[
    {"key":"batch-1","value":"Message 1"},
    {"key":"batch-2","value":"Message 2"}
  ]'
```

## 🐳 Docker 빌드 & 실행

### 빌드
```bash
docker build -t kafka-producer:latest .
```

### 실행
```bash
docker run -p 8080:8080 \
  -e KAFKA_BROKERS=kafka-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092 \
  -e KAFKA_TOPIC=demo-events \
  kafka-producer:latest
```

## ☸️ Kubernetes 배포

GitOps 저장소의 Helm Chart를 통해 자동 배포됩니다.

```bash
# ArgoCD Application 생성
kubectl apply -f gitops/apps/kafka-producer-app.yaml
```

## 📊 메트릭 & 모니터링

- Health check: `GET /health`
- 로그: JSON 형식으로 구조화된 로그 출력

## 🔐 보안 설정

- 비루트 사용자로 실행 (UID: 1000)
- 최소 권한 원칙
- Health check 포함

## 📚 참고 자료

- [Sarama (Kafka Go Client)](https://github.com/IBM/sarama)
- [Gin Web Framework](https://github.com/gin-gonic/gin)


