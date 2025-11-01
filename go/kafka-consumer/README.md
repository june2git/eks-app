# Kafka Consumer (Go)

Kafka 메시지를 수신하고 저장하는 Go 기반 Consumer 애플리케이션


## 🚀 기능

- ✅ Kafka Consumer Group 기반 메시지 소비
- ✅ HTTP API를 통한 메시지 조회
- ✅ 메모리 내 메시지 저장 (최대 1000개)
- ✅ Health check 엔드포인트
- ✅ Graceful shutdown
- ✅ 자동 리밸런싱
- ✅ 오프셋 자동 커밋

## 📋 API 엔드포인트

### 1. Health Check
```bash
GET /health
```


**응답:**
```json
{
  "status": "healthy",
  "service": "kafka-consumer",
  "kafka": {
    "brokers": "kafka-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092",
    "topic": "demo-events",
    "groupID": "kafka-consumer-group"
  },
  "messages": {
    "count": 150,
    "max": 1000
  }
}
```

### 2. 메시지 조회
```bash
GET /messages?limit=100
```

**응답:**
```json
{
  "count": 100,
  "total": 150,
  "messages": [
    {
      "key": "user-123",
      "value": "{\"message\":\"Hello\"}",
      "timestamp": "2025-10-31T10:30:00Z",
      "partition": 0,
      "offset": 1234,
      "metadata": {
        "message": "Hello"
      }
    }
  ]
}
```

### 3. 통계 조회
```bash
GET /stats
```

**응답:**
```json
{
  "topic": "demo-events",
  "group_id": "kafka-consumer-group",
  "message_count": 150,
  "max_messages": 1000
}
```

### 4. 메시지 삭제
```bash
DELETE /messages
```

**응답:**
```json
{
  "status": "success",
  "message": "All messages cleared"
}
```

## 🔧 환경 변수

| 변수 | 기본값 | 설명 |
|------|--------|------|
| `KAFKA_BROKERS` | `kafka-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092` | Kafka 브로커 주소 |
| `KAFKA_TOPIC` | `demo-events` | 구독할 토픽 |
| `KAFKA_GROUP_ID` | `kafka-consumer-group` | Consumer Group ID |
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
KAFKA_BROKERS=localhost:9092 \
KAFKA_TOPIC=test-topic \
KAFKA_GROUP_ID=my-consumer-group \
go run main.go
```

### 3. 메시지 확인
```bash
# 최신 메시지 100개 조회
curl http://localhost:8080/messages?limit=100

# 통계 조회
curl http://localhost:8080/stats

# Health check
curl http://localhost:8080/health
```

## 🐳 Docker 빌드 & 실행

### 빌드
```bash
docker build -t kafka-consumer:latest .
```

### 실행
```bash
docker run -p 8080:8080 \
  -e KAFKA_BROKERS=kafka-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092 \
  -e KAFKA_TOPIC=demo-events \
  -e KAFKA_GROUP_ID=kafka-consumer-group \
  kafka-consumer:latest
```

## ☸️ Kubernetes 배포

GitOps 저장소의 Helm Chart를 통해 자동 배포됩니다.

```bash
# ArgoCD Application 생성
kubectl apply -f gitops/apps/kafka-consumer-app.yaml
```

## 📊 Consumer Group 동작

### Consumer Group 장점
- ✅ 파티션 간 메시지 병렬 처리
- ✅ 자동 리밸런싱 (Consumer 추가/제거 시)
- ✅ Offset 관리 (재시작 시 이어서 처리)
- ✅ 고가용성 (Consumer 장애 시 자동 복구)

### Offset 관리
- `OffsetNewest`: 최신 메시지부터 읽기 (기본값)
- `OffsetOldest`: 가장 오래된 메시지부터 읽기

```go
// main.go에서 변경 가능
config.Consumer.Offsets.Initial = sarama.OffsetOldest
```

## 🔐 보안 설정

- 비루트 사용자로 실행 (UID: 1000)
- 최소 권한 원칙
- Health check 포함

## 📈 메모리 관리

- 최대 1000개 메시지 메모리 저장
- 오래된 메시지 자동 제거 (FIFO)
- `/messages` DELETE로 수동 삭제 가능

## 📚 참고 자료

- [Sarama (Kafka Go Client)](https://github.com/IBM/sarama)
- [Gin Web Framework](https://github.com/gin-gonic/gin)
- [Kafka Consumer Groups](https://kafka.apache.org/documentation/#consumerapi)

