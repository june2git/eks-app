# Runtime stage - GitHub Actions에서 이미 빌드된 JAR 사용
FROM eclipse-temurin:17-jre-alpine

# Set working directory
WORKDIR /app

# Copy the pre-built jar from GitHub Actions
COPY build/libs/*.jar app.jar

# Expose port
EXPOSE 8080

# Run the application
ENTRYPOINT ["java", "-jar", "app.jar"]
