# Kafka Applications (Go)

Go 언어로 작성된 Kafka Producer와 Consumer 애플리케이션

## 📋 프로젝트 구조

```
eks-app/go/
├── kafka-producer/           # Kafka Producer 애플리케이션
│   ├── main.go              # Producer 소스 코드
│   ├── go.mod               # Go 모듈 정의
│   ├── Dockerfile           # Docker 이미지 빌드
│   └── README.md            # Producer 문서
│
├── kafka-consumer/           # Kafka Consumer 애플리케이션
│   ├── main.go              # Consumer 소스 코드
│   ├── go.mod               # Go 모듈 정의
│   ├── Dockerfile           # Docker 이미지 빌드
│   └── README.md            # Consumer 문서
│
├── .github/workflows/        # CI/CD 파이프라인
│   ├── producer-deploy.yaml # Producer 배포 워크플로우
│   └── consumer-deploy.yaml # Consumer 배포 워크플로우
│
└── README.md                # 이 파일
```


## 🎯 애플리케이션 기능

### Kafka Producer
- ✅ HTTP API를 통한 메시지 전송
- ✅ 단일 메시지 & 배치 메시지 지원
- ✅ Snappy 압축
- ✅ 자동 재시도 (최대 5회)
- ✅ 모든 ISR 확인 (데이터 안정성)

**API 엔드포인트:**
- `POST /send` - 단일 메시지 전송
- `POST /send/batch` - 배치 메시지 전송
- `GET /health` - Health check

### Kafka Consumer
- ✅ Consumer Group 기반 메시지 소비
- ✅ HTTP API를 통한 메시지 조회
- ✅ 메모리 내 메시지 저장 (최대 1000개)
- ✅ 자동 리밸런싱
- ✅ 오프셋 자동 커밋

**API 엔드포인트:**
- `GET /messages?limit=100` - 메시지 조회
- `GET /stats` - 통계 조회
- `DELETE /messages` - 메시지 삭제
- `GET /health` - Health check

## 🚀 빠른 시작

### 사전 요구사항
- Go 1.22+
- Docker
- Kubernetes (EKS) 클러스터
- Kafka 클러스터 (Strimzi)

### 로컬 개발

#### Producer 실행
```bash
cd kafka-producer
go mod download
go run main.go
```

#### Consumer 실행
```bash
cd kafka-consumer
go mod download
go run main.go
```

### Docker 빌드

#### Producer
```bash
cd kafka-producer
docker build -t kafka-producer:latest .
docker run -p 8080:8080 kafka-producer:latest
```

#### Consumer
```bash
cd kafka-consumer
docker build -t kafka-consumer:latest .
docker run -p 8080:8080 kafka-consumer:latest
```

## ☸️ Kubernetes 배포

### 1. ECR 저장소 생성

```bash
# infra/ecr.tf에 추가 필요
resource "aws_ecr_repository" "kafka_producer" {
  name = "kafka-producer"
  
  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_repository" "kafka_consumer" {
  name = "kafka-consumer"
  
  image_scanning_configuration {
    scan_on_push = true
  }
}
```

### 2. ArgoCD Application 배포

```bash
# GitOps 저장소에서
kubectl apply -f gitops/apps/kafka-apps.yaml

# 배포 상태 확인
kubectl get application kafka-apps -n argocd
kubectl get pods -n default -l component=producer
kubectl get pods -n default -l component=consumer
```

### 3. Ingress 확인

```bash
# Producer Ingress
kubectl get ingress kafka-producer -n default

# Consumer Ingress
kubectl get ingress kafka-consumer -n default
```

## 🔄 CI/CD 파이프라인

### GitHub Actions 워크플로우

> **재사용 가능한 워크플로우 사용**
> 
> Producer와 Consumer 모두 `devops-templates` 저장소의 `build_and_push_template.yaml`을 재사용합니다.
> 이를 통해 일관된 CI/CD 프로세스를 유지하고 중복 코드를 제거합니다.

#### Producer 배포 (`.github/workflows/producer-deploy.yaml`)
```yaml
# 트리거: kafka-producer/ 디렉토리 변경 시
uses: june2git/devops-templates/.github/workflows/build_and_push_template.yaml@main
with:
  app_name: kafka-producer
  build_system: go
  ecr_repo: kafka-producer
  values_file: charts/kafka-apps-values.yaml
  values_prefix: producer  # producer.image.* 경로 업데이트

# 동작:
# 1. Go 환경 설정 (v1.22)
# 2. Docker Multi-stage 빌드로 바이너리 생성
# 3. ECR에 이미지 푸시 (producer-main-<RUN_NUMBER>)
# 4. GitOps 저장소의 producer.image.tag 자동 업데이트
# 5. ArgoCD가 변경 감지하여 자동 배포
```

#### Consumer 배포 (`.github/workflows/consumer-deploy.yaml`)
```yaml
# 트리거: kafka-consumer/ 디렉토리 변경 시
uses: june2git/devops-templates/.github/workflows/build_and_push_template.yaml@main
with:
  app_name: kafka-consumer
  build_system: go
  ecr_repo: kafka-consumer
  values_file: charts/kafka-apps-values.yaml
  values_prefix: consumer  # consumer.image.* 경로 업데이트

# 동작: Producer와 동일한 프로세스
```

#### 워크플로우 장점
- ✅ **일관성**: Java/Go/Node.js 모든 앱이 동일한 프로세스 사용
- ✅ **간결함**: 각 앱의 워크플로우가 30줄 이하로 단순화
- ✅ **유지보수성**: 템플릿만 수정하면 모든 앱에 적용
- ✅ **보안**: OIDC 기반 AWS 인증으로 자격 증명 불필요

### 배포 프로세스

```
1. 개발자 코드 푸시
   └─ github.com/june2git/eks-app/go/

2. GitHub Actions 트리거
   ├─ Docker 이미지 빌드
   ├─ ECR 푸시 (producer-main-123)
   └─ GitOps 업데이트

3. ArgoCD 자동 동기화
   └─ github.com/june2git/gitops/

4. Kubernetes 배포
   ├─ Rolling Update
   └─ ALB 업데이트

5. 서비스 가용
   ├─ producer.june2soul.store
   └─ consumer.june2soul.store
```

## 🧪 테스트

### Producer 테스트

```bash
# Health check
curl https://producer.june2soul.store/health

# 단일 메시지 전송
curl -X POST https://producer.june2soul.store/send \
  -H "Content-Type: application/json" \
  -d '{
    "key": "user-123",
    "value": "Hello from Go Producer!",
    "metadata": {
      "source": "api-test",
      "version": "1.0"
    }
  }'

# 배치 메시지 전송
curl -X POST https://producer.june2soul.store/send/batch \
  -H "Content-Type: application/json" \
  -d '[
    {"key": "batch-1", "value": "Message 1"},
    {"key": "batch-2", "value": "Message 2"},
    {"key": "batch-3", "value": "Message 3"}
  ]'
```

### Consumer 테스트

```bash
# Health check
curl https://consumer.june2soul.store/health

# 최신 메시지 100개 조회
curl https://consumer.june2soul.store/messages?limit=100

# 통계 조회
curl https://consumer.june2soul.store/stats

# 메시지 삭제
curl -X DELETE https://consumer.june2soul.store/messages
```

### 통합 테스트

```bash
# Producer에서 메시지 전송
for i in {1..10}; do
  curl -X POST https://producer.june2soul.store/send \
    -H "Content-Type: application/json" \
    -d "{\"key\":\"test-$i\",\"value\":\"Test message $i\"}"
  echo ""
done

# Consumer에서 메시지 확인
curl https://consumer.june2soul.store/messages?limit=10 | jq
```

## 📊 모니터링

### Health Check
```bash
# Producer
kubectl run test-pod --rm -i --tty --image=curlimages/curl -- \
  curl http://kafka-producer.default.svc.cluster.local/health

# Consumer
kubectl run test-pod --rm -i --tty --image=curlimages/curl -- \
  curl http://kafka-consumer.default.svc.cluster.local/health
```

### 로그 확인
```bash
# Producer 로그
kubectl logs -f -l app=kafka-producer -n default

# Consumer 로그
kubectl logs -f -l app=kafka-consumer -n default
```

### Kafka 메트릭
```bash
# Kafka 클러스터 상태
kubectl get kafka kafka-cluster -n kafka

# 토픽 상태
kubectl get kafkatopics -n kafka

# Consumer Group 확인
kubectl run kafka-consumer-groups -ti \
  --image=quay.io/strimzi/kafka:0.48.0-kafka-4.0.0 \
  --rm=true --restart=Never \
  -n kafka \
  -- bin/kafka-consumer-groups.sh \
  --bootstrap-server kafka-cluster-kafka-bootstrap:9092 \
  --describe \
  --group kafka-consumer-group
```

## 🔧 트러블슈팅

### Producer가 메시지를 전송하지 못하는 경우

```bash
# Kafka 클러스터 확인
kubectl get pods -n kafka -l strimzi.io/cluster=kafka-cluster

# Producer 로그 확인
kubectl logs -f -l app=kafka-producer -n default

# Kafka 연결 테스트
kubectl run kafka-test -ti \
  --image=quay.io/strimzi/kafka:0.48.0-kafka-4.0.0 \
  --rm=true --restart=Never \
  -n kafka \
  -- bin/kafka-broker-api-versions.sh \
  --bootstrap-server kafka-cluster-kafka-bootstrap:9092
```

### Consumer가 메시지를 수신하지 못하는 경우

```bash
# Consumer 로그 확인
kubectl logs -f -l app=kafka-consumer -n default

# 토픽에 메시지가 있는지 확인
kubectl run kafka-consumer-check -ti \
  --image=quay.io/strimzi/kafka:0.48.0-kafka-4.0.0 \
  --rm=true --restart=Never \
  -n kafka \
  -- bin/kafka-console-consumer.sh \
  --bootstrap-server kafka-cluster-kafka-bootstrap:9092 \
  --topic demo-events \
  --from-beginning \
  --max-messages 10
```

## 📚 참고 자료

### Go 애플리케이션
- [kafka-producer/README.md](./kafka-producer/README.md)
- [kafka-consumer/README.md](./kafka-consumer/README.md)

### Kafka & Kubernetes
- [Strimzi Documentation](https://strimzi.io/docs/)
- [Sarama (Kafka Go Client)](https://github.com/IBM/sarama)
- [Kafka Documentation](https://kafka.apache.org/documentation/)

### GitOps & CI/CD
- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
- [GitHub Actions OIDC](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services)

## 🤝 기여

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## 📄 라이선스

MIT License

