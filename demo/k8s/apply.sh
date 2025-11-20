#!/bin/bash

# 색상 출력
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Demo 애플리케이션 배포 ===${NC}"

# kubeconfig 파일 경로 확인
KUBECONFIG_FILE="${1:-/Users/june2soul/.kube/config}"

if [ ! -f "$KUBECONFIG_FILE" ]; then
    echo "❌ kubeconfig 파일을 찾을 수 없습니다: $KUBECONFIG_FILE"
    echo "사용법: ./apply.sh [kubeconfig 경로]"
    exit 1
fi

export KUBECONFIG="$KUBECONFIG_FILE"

# 1. Namespace 생성
echo -e "${YELLOW}[1/4] Namespace 생성 중...${NC}"
kubectl apply -f namespace.yaml

# 2. Deployment 생성
echo -e "${YELLOW}[2/4] Deployment 생성 중...${NC}"
kubectl apply -f deployment.yaml

# 3. Service 생성
echo -e "${YELLOW}[3/4] Service 생성 중...${NC}"
kubectl apply -f service.yaml

# 4. Ingress 생성
echo -e "${YELLOW}[4/4] Ingress 생성 중...${NC}"
kubectl apply -f ingress.yaml

echo -e "${GREEN}✓ 배포 완료!${NC}"
echo ""
echo "배포 상태 확인:"
kubectl get all -n demo
echo ""
echo "Ingress 상태 확인:"
kubectl get ingress -n demo

