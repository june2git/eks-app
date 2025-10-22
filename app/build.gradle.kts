plugins {
    id("org.springframework.boot") version "3.3.4"
    id("io.spring.dependency-management") version "1.1.6"
    java
}


group = "com.example"
version = System.getenv("APP_VERSION") ?: "0.0.1-SNAPSHOT"


java { toolchain { languageVersion.set(JavaLanguageVersion.of(17)) } }


dependencies {
    implementation("org.springframework.boot:spring-boot-starter-web")
    testImplementation("org.springframework.boot:spring-boot-starter-test")
}


tasks.test { useJUnitPlatform() }