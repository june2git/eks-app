# Demo 애플리케이션 Kubernetes 배포

이 디렉토리에는 demo 애플리케이션을 AKS 클러스터에 배포하기 위한 Kubernetes 매니페스트 파일들이 있습니다.

## 파일 구조

- `namespace.yaml`: demo 네임스페이스 생성
- `deployment.yaml`: demo 애플리케이션 Deployment (2개 Pod)
- `service.yaml`: demo Service (80 포트 → 8080 포트)
- `ingress.yaml`: Application Gateway Ingress (외부 접근)
- `apply.sh`: 모든 리소스를 한 번에 배포하는 스크립트

## 사용 방법

### 방법 1: 스크립트 사용 (권장)

```bash
cd k8s
./apply.sh [kubeconfig 경로]
```

기본 kubeconfig 경로: `../infra-azure/kubeconfig`

### 방법 2: 수동 배포

```bash
# kubeconfig 설정
export KUBECONFIG=../infra-azure/kubeconfig

# 순서대로 배포
kubectl apply -f namespace.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml
```

## 배포 확인

```bash
# 모든 리소스 확인
kubectl get all -n demo

# Pod 상태 확인
kubectl get pods -n demo

# Service 확인
kubectl get svc -n demo

# Ingress 확인
kubectl get ingress -n demo

# Pod 로그 확인
kubectl logs -n demo -l app=demo
```

## 접근 방법

### Application Gateway Public IP 사용

```bash
# Application Gateway Public IP 확인
kubectl get ingress -n demo -o jsonpath='{.items[0].status.loadBalancer.ingress[0].ip}'

# 또는 Terraform output 사용
cd ../../infra-azure
terraform output appgw_public_ip
```

### DNS 도메인 사용 (DNS Zone 설정된 경우)

- URL: `http://demo.june2soul.store`

## 이미지 업데이트

새로운 이미지를 배포하려면:

1. 이미지 빌드 및 push:
   ```bash
   cd ..
   ./build-and-push.sh [태그]
   ```

2. Deployment 이미지 태그 업데이트:
   ```bash
   kubectl set image deployment/demo demo=acrjune2souldev.azurecr.io/demo:[새태그] -n demo
   ```

3. 또는 `deployment.yaml`의 이미지 태그를 수정 후 재배포:
   ```bash
   kubectl apply -f deployment.yaml
   ```

## 삭제

```bash
kubectl delete -f .
```

또는

```bash
kubectl delete namespace demo
```

