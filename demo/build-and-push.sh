#!/bin/bash

# ACR 정보
ACR_NAME="acrjune2souldev"
ACR_LOGIN_SERVER="${ACR_NAME}.azurecr.io"
IMAGE_NAME="demo"
IMAGE_TAG="${1:-latest}"

# 색상 출력
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Demo 프로젝트 빌드 및 ACR Push ===${NC}"

# 1. Gradle 빌드
echo -e "${YELLOW}[1/5] Gradle 빌드 중...${NC}"
./gradlew clean build -x test

if [ $? -ne 0 ]; then
    echo "❌ Gradle 빌드 실패"
    exit 1
fi

# 2. JAR 파일 확인
JAR_FILE=$(find build/libs -name "*.jar" ! -name "*-plain.jar" | head -n 1)
if [ -z "$JAR_FILE" ]; then
    echo "❌ JAR 파일을 찾을 수 없습니다"
    exit 1
fi

echo -e "${GREEN}✓ JAR 파일: $JAR_FILE${NC}"

# 3. Azure 로그인 확인
echo -e "${YELLOW}[2/5] Azure 로그인 확인 중...${NC}"
az account show > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "❌ Azure에 로그인되지 않았습니다. 'az login'을 실행하세요"
    exit 1
fi

# 4. ACR 로그인
echo -e "${YELLOW}[3/5] ACR 로그인 중...${NC}"
az acr login --name $ACR_NAME

if [ $? -ne 0 ]; then
    echo "❌ ACR 로그인 실패"
    exit 1
fi

# 5. Docker 이미지 빌드
echo -e "${YELLOW}[4/5] Docker 이미지 빌드 중...${NC}"
docker build --platform linux/amd64 -t ${ACR_LOGIN_SERVER}/${IMAGE_NAME}:${IMAGE_TAG} .

if [ $? -ne 0 ]; then
    echo "❌ Docker 이미지 빌드 실패"
    exit 1
fi

# 6. ACR에 Push
echo -e "${YELLOW}[5/5] ACR에 이미지 Push 중...${NC}"
docker push ${ACR_LOGIN_SERVER}/${IMAGE_NAME}:${IMAGE_TAG}

if [ $? -ne 0 ]; then
    echo "❌ 이미지 Push 실패"
    exit 1
fi

echo -e "${GREEN}✓ 완료!${NC}"
echo -e "${GREEN}이미지: ${ACR_LOGIN_SERVER}/${IMAGE_NAME}:${IMAGE_TAG}${NC}"

