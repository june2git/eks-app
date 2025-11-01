// ============================================================
// Package Declaration (패키지 선언)
// ============================================================
// main: 실행 가능한 프로그램의 진입점
// Go에서 main 패키지는 특별한 의미를 가지며, 프로그램 시작점인 main() 함수를 포함해야 함
package main

// ============================================================
// Import Statements (패키지 임포트)
// ============================================================
// 표준 라이브러리와 외부 패키지를 가져옴
import (
	"context"       // 컨텍스트: 작업 취소, 타임아웃 관리
	"encoding/json" // JSON 인코딩/디코딩
	"fmt"           // 포맷된 입출력 (문자열 포맷팅)
	"log"           // 로깅
	"net/http"      // HTTP 서버/클라이언트
	"os"            // 운영체제 기능 (환경변수, 시그널 등)
	"os/signal"     // OS 시그널 처리 (SIGINT, SIGTERM)
	"syscall"       // 시스템 콜 상수
	"time"          // 시간 관련 기능

	"github.com/IBM/sarama"    // Kafka 클라이언트 라이브러리
	"github.com/gin-gonic/gin" // Gin 웹 프레임워크 (빠르고 간결한 HTTP 라우터)
)

// ============================================================
// Struct Definitions (구조체 정의)
// ============================================================

// Message: Kafka로 전송할 메시지 구조체
// Go의 struct는 다른 언어의 class와 유사하지만 메서드만 가능 (상속 없음)
// JSON 태그: JSON 직렬화/역직렬화 시 사용할 필드 이름 정의
type Message struct {
	Key       string                 `json:"key"`                // 메시지 키 (같은 키는 같은 파티션으로)
	Value     string                 `json:"value"`              // 메시지 본문
	Timestamp time.Time              `json:"timestamp"`          // 타임스탬프
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // 추가 메타데이터 (선택적)
	// omitempty: 필드가 비어있으면 JSON에서 제외
}

// ProducerService: Kafka Producer 기능을 제공하는 서비스 구조체
// 구조체는 데이터와 그 데이터를 다루는 메서드를 그룹화
type ProducerService struct {
	producer sarama.SyncProducer // Kafka Producer 인스턴스 (동기식)
	topic    string              // 메시지를 보낼 Kafka 토픽 이름
}

// ============================================================
// Constructor Function (생성자 함수)
// ============================================================
// NewProducerService: ProducerService 인스턴스를 생성하는 생성자 함수
// Go에는 생성자가 없으므로 관례적으로 New로 시작하는 함수를 생성자로 사용
//
// 매개변수:
//   - brokers []string: Kafka 브로커 주소 목록 (예: ["localhost:9092"])
//   - topic string: 메시지를 보낼 토픽 이름
//
// 반환값:
//   - *ProducerService: 생성된 서비스의 포인터 (메모리 주소)
//   - error: 에러 발생 시 에러 객체, 없으면 nil
//
// Go의 에러 처리: 예외(exception) 대신 에러를 반환값으로 처리
func NewProducerService(brokers []string, topic string) (*ProducerService, error) {
	// Kafka Producer 설정 생성
	config := sarama.NewConfig()

	// RequiredAcks: 메시지 전송 확인 수준 설정
	// WaitForAll(-1): 모든 In-Sync Replica가 메시지를 받을 때까지 대기
	// 장점: 최고 수준의 데이터 안정성 (메시지 손실 최소화)
	// 단점: 전송 속도가 느림
	config.Producer.RequiredAcks = sarama.WaitForAll

	// 전송 실패 시 최대 재시도 횟수
	// 네트워크 일시 장애 등에 대비
	config.Producer.Retry.Max = 5

	// 성공 메시지를 채널로 반환
	// SyncProducer는 이 값이 true여야 함
	config.Producer.Return.Successes = true

	// 메시지 압축 방식: Snappy
	// Snappy: Google이 개발한 빠른 압축 알고리즘
	// 장점: 압축/해제 속도가 빠름, CPU 부하가 적음
	// 다른 옵션: Gzip(높은 압축률), LZ4, Zstd
	config.Producer.Compression = sarama.CompressionSnappy

	// Kafka Producer 생성 (동기식)
	// SyncProducer: 메시지 전송이 완료될 때까지 블로킹
	// 장점: 간단한 에러 처리, 전송 성공 보장
	// 단점: 비동기보다 느림
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		// 에러 발생 시 nil과 에러 반환
		// %w: 에러 래핑 (원본 에러 정보 보존)
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	// 성공 로그 출력
	// Printf: 포맷팅된 문자열 출력
	// %v: 값을 기본 형식으로 출력
	// %s: 문자열로 출력
	log.Printf("✅ Kafka Producer initialized: brokers=%v topic=%s", brokers, topic)

	// ProducerService 인스턴스 반환
	// &: 주소 연산자 (포인터 생성)
	// Go는 구조체를 값으로 복사하므로, 큰 구조체는 포인터로 전달하는 것이 효율적
	return &ProducerService{
		producer: producer,
		topic:    topic,
	}, nil // 에러 없음을 의미하는 nil 반환
}

// ============================================================
// Method Functions (메서드 함수)
// ============================================================
// SendMessage: Kafka에 메시지를 전송하는 메서드
// (ps *ProducerService): 리시버(receiver) - 이 메서드가 속한 구조체
// *ProducerService: 포인터 리시버 (원본 구조체를 참조, 복사하지 않음)
// 포인터 리시버를 사용하는 이유: 구조체 복사 비용 절약, 원본 수정 가능
//
// 매개변수:
//   - msg Message: 전송할 메시지 (구조체 값)
//
// 반환값:
//   - partition int32: 메시지가 저장된 파티션 번호
//   - offset int64: 파티션 내에서 메시지의 위치
//   - err error: 에러 (없으면 nil)
//
// Named return values: 반환값에 이름을 붙일 수 있음
func (ps *ProducerService) SendMessage(msg Message) (partition int32, offset int64, err error) {
	// 메시지를 JSON으로 직렬화 (Marshaling)
	// Go 구조체 → JSON 바이트 배열 변환
	// JSON 태그(`json:"key"`)를 사용하여 필드 이름 매핑
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		// 직렬화 실패 시 기본값(0, 0)과 에러 반환
		return 0, 0, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Kafka ProducerMessage 생성
	// &: 구조체 포인터 생성 및 초기화
	kafkaMsg := &sarama.ProducerMessage{
		Topic: ps.topic,                      // 토픽: 메시지가 저장될 카테고리
		Key:   sarama.StringEncoder(msg.Key), // 키: 파티션 결정에 사용 (같은 키 = 같은 파티션)
		Value: sarama.ByteEncoder(msgBytes),  // 값: 실제 메시지 내용 (JSON 바이트)
	}
	// StringEncoder/ByteEncoder: Sarama가 제공하는 인코더 인터페이스 구현

	// Kafka에 메시지 전송 (동기식)
	// SendMessage: 전송 완료까지 블로킹 (대기)
	// 반환값: 파티션 번호, 오프셋, 에러
	partition, offset, err = ps.producer.SendMessage(kafkaMsg)
	if err != nil {
		// 전송 실패 시 에러 반환
		return 0, 0, fmt.Errorf("failed to send message: %w", err)
	}

	// 전송 성공 로그 출력
	// %s: 문자열, %d: 정수(decimal)
	log.Printf("📤 Message sent: topic=%s partition=%d offset=%d key=%s",
		ps.topic, partition, offset, msg.Key)

	// 성공: 파티션, 오프셋 반환, 에러는 nil
	return partition, offset, nil
}

// Close: Producer 리소스 정리 및 연결 종료 메서드
// Go의 리소스 관리: defer 키워드와 함께 사용하여 자동 정리
// 사용 예: defer producerService.Close()
//
// 반환값:
//   - error: 종료 중 발생한 에러 (없으면 nil)
func (ps *ProducerService) Close() error {
	// 내부 Producer 종료
	// 연결 해제, 버퍼 플러시, 고루틴 정리 등 수행
	return ps.producer.Close()
}

// ============================================================
// Main Function (메인 함수)
// ============================================================
// main: 프로그램의 진입점 (Entry Point)
// Go 프로그램은 main 패키지의 main() 함수부터 실행 시작
func main() {
	// ========================================
	// 1. 환경 변수 로드 및 초기 설정
	// ========================================
	// getEnv: 환경 변수를 읽고, 없으면 기본값 반환하는 헬퍼 함수
	// := (short variable declaration): 타입을 자동으로 추론하여 변수 선언 및 초기화
	brokers := getEnv("KAFKA_BROKERS", "kafka-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092")
	topic := getEnv("KAFKA_TOPIC", "demo-events")
	port := getEnv("PORT", "8080")

	// ========================================
	// 2. Kafka Producer 초기화
	// ========================================
	// []string{brokers}: 슬라이스 리터럴 (동적 배열과 유사)
	producerService, err := NewProducerService([]string{brokers}, topic)
	if err != nil {
		// Fatalf: 에러 로그를 출력하고 프로그램 종료 (os.Exit(1))
		// %v: 값을 기본 형식으로 출력
		log.Fatalf("❌ Failed to create producer: %v", err)
	}

	// defer: 함수가 종료될 때 실행 (리소스 정리용)
	// LIFO(Last In First Out) 순서로 실행
	// 여기서는 main 함수 종료 시 Producer 연결 종료
	defer producerService.Close()

	// ========================================
	// 3. Gin 웹 프레임워크 설정
	// ========================================
	// Gin: 빠르고 간결한 HTTP 웹 프레임워크
	// ReleaseMode: 프로덕션 모드 (디버그 로그 비활성화)
	gin.SetMode(gin.ReleaseMode)

	// Default 라우터: 로거, 리커버리 미들웨어가 기본 포함
	// 미들웨어: HTTP 요청/응답을 가로채서 처리하는 함수
	router := gin.Default()

	// ========================================
	// 4. HTTP API 엔드포인트 정의
	// ========================================

	// GET /health: 헬스체크 엔드포인트
	// Kubernetes liveness/readiness probe에서 사용
	// router.GET: HTTP GET 요청 핸들러 등록
	// func(c *gin.Context): 익명 함수 (람다와 유사)
	router.GET("/health", func(c *gin.Context) {
		// c.JSON: JSON 응답 전송
		// http.StatusOK: HTTP 200 상태 코드
		// gin.H: map[string]interface{}의 별칭 (JSON 객체 생성용)
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "kafka-producer",
			"kafka": gin.H{
				"brokers": brokers,
				"topic":   topic,
			},
		})
	})

	// POST /send: 단일 메시지 전송 엔드포인트
	// 클라이언트가 JSON 형식으로 메시지를 보내면 Kafka에 전송
	// 요청 예시: {"key":"user-1", "value":"Hello Kafka"}
	router.POST("/send", func(c *gin.Context) {
		// var: 변수 선언 (타입 명시)
		// Message 구조체 인스턴스 생성 (제로값으로 초기화)
		var msg Message

		// ShouldBindJSON: HTTP 요청 본문을 JSON으로 파싱하여 구조체에 바인딩
		// &msg: msg의 주소를 전달 (포인터) - 함수가 원본을 수정할 수 있도록
		// if err := ...: 단축 변수 선언 + 조건문 (Go의 관용적 패턴)
		if err := c.ShouldBindJSON(&msg); err != nil {
			// JSON 파싱 실패 시 400 Bad Request 응답
			// c.JSON: JSON 응답 전송
			// err.Error(): 에러를 문자열로 변환
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request body",
				"details": err.Error(),
			})
			return // 함수 종료 (early return 패턴)
		}

		// Timestamp 자동 설정
		// IsZero(): time.Time의 제로값 확인 (클라이언트가 타임스탬프를 보내지 않은 경우)
		// time.Now(): 현재 시각
		if msg.Timestamp.IsZero() {
			msg.Timestamp = time.Now()
		}

		// SendMessage 호출하여 Kafka에 메시지 전송
		// 반환값: partition(파티션 번호), offset(오프셋), err(에러)
		partition, offset, err := producerService.SendMessage(msg)
		if err != nil {
			// 전송 실패 시 500 Internal Server Error 응답
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to send message",
				"details": err.Error(),
			})
			return
		}

		// 전송 성공 시 200 OK 응답
		// 클라이언트에게 파티션, 오프셋 정보 반환
		c.JSON(http.StatusOK, gin.H{
			"status":    "success",
			"partition": partition,
			"offset":    offset,
			"message":   msg,
		})
	})

	// POST /send/batch: 여러 메시지를 한 번에 전송하는 배치 엔드포인트
	// 클라이언트가 메시지 배열을 보내면 순차적으로 Kafka에 전송
	// 요청 예시: [{"key":"user-1","value":"msg1"}, {"key":"user-2","value":"msg2"}]
	router.POST("/send/batch", func(c *gin.Context) {
		// []Message: Message 슬라이스 (동적 배열)
		var messages []Message

		// JSON 배열을 슬라이스에 바인딩
		if err := c.ShouldBindJSON(&messages); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request body",
				"details": err.Error(),
			})
			return
		}

		// make: 슬라이스 생성 함수
		// []gin.H: gin.H 타입의 슬라이스
		// 0: 초기 길이 (비어있음)
		// len(messages): 용량(capacity) - 미리 메모리 할당하여 성능 최적화
		results := make([]gin.H, 0, len(messages))
		successCount := 0
		failCount := 0

		// for 루프: 슬라이스를 순회
		// i: 인덱스, msg: 현재 요소 값
		// range: 슬라이스/맵/채널 등을 순회하는 키워드
		for i, msg := range messages {
			// 타임스탬프가 없으면 현재 시각으로 설정
			if msg.Timestamp.IsZero() {
				msg.Timestamp = time.Now()
			}

			// 메시지 전송 시도
			partition, offset, err := producerService.SendMessage(msg)
			if err != nil {
				// 실패한 경우
				failCount++
				// append: 슬라이스에 요소 추가
				results = append(results, gin.H{
					"index":  i, // 몇 번째 메시지인지
					"status": "failed",
					"error":  err.Error(),
				})
			} else {
				// 성공한 경우
				successCount++
				results = append(results, gin.H{
					"index":     i,
					"status":    "success",
					"partition": partition,
					"offset":    offset,
				})
			}
		}

		// 배치 전송 결과 반환
		// 일부 실패해도 200 OK 반환 (전체 결과 포함)
		c.JSON(http.StatusOK, gin.H{
			"total":   len(messages), // 전체 메시지 수
			"success": successCount,  // 성공 개수
			"failed":  failCount,     // 실패 개수
			"results": results,       // 각 메시지별 상세 결과
		})
	})

	// GET /: 루트 엔드포인트 (API 정보 제공)
	// 사용 가능한 API 엔드포인트 목록을 반환
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "kafka-producer",
			"version": "1.0.0",
			"endpoints": gin.H{
				"health":     "GET /health",
				"send":       "POST /send",
				"send_batch": "POST /send/batch",
			},
		})
	})

	// ========================================
	// 5. HTTP 서버 설정 및 시작
	// ========================================
	// http.Server: Go의 표준 HTTP 서버 구조체
	// &: 주소 연산자 (구조체 리터럴의 포인터 생성)
	srv := &http.Server{
		Addr:    ":" + port, // 리슨 주소 (예: ":8080")
		Handler: router,     // Gin 라우터를 HTTP 핸들러로 사용
	}

	// ========================================
	// 6. Graceful Shutdown 구현
	// ========================================
	// Graceful Shutdown: 진행 중인 요청을 완료한 후 서버 종료
	// 장점: 데이터 손실 방지, 안정적인 종료

	// go: 고루틴(goroutine) 실행 - Go의 경량 스레드
	// 고루틴: 함수를 비동기적으로 실행 (동시성 프로그래밍)
	go func() {
		log.Printf("🚀 Kafka Producer server starting on port %s", port)

		// ListenAndServe: HTTP 서버 시작 (블로킹)
		// 에러 발생 시 반환 (정상 종료 시에도 http.ErrServerClosed 반환)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// 예상치 못한 에러 발생 시 프로그램 종료
			log.Fatalf("❌ Server failed: %v", err)
		}
	}()

	// 채널(Channel): 고루틴 간 통신을 위한 파이프
	// make(chan os.Signal, 1): 버퍼 크기 1인 시그널 채널 생성
	// 버퍼: 시그널을 놓치지 않기 위해
	quit := make(chan os.Signal, 1)

	// signal.Notify: 특정 OS 시그널을 채널로 받도록 설정
	// SIGINT: Ctrl+C (2)
	// SIGTERM: kill 명령 (15) - Kubernetes Pod 종료 시 사용
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// <-quit: 채널에서 값을 받을 때까지 블로킹 (대기)
	// 시그널이 올 때까지 메인 고루틴이 대기
	<-quit

	log.Println("🛑 Shutting down server...")

	// context.WithTimeout: 타임아웃이 있는 컨텍스트 생성
	// context.Background(): 최상위 컨텍스트 (부모 없음)
	// 5*time.Second: 5초 타임아웃
	// cancel: 컨텍스트 취소 함수
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // 함수 종료 시 컨텍스트 취소 (리소스 정리)

	// srv.Shutdown: 서버를 안전하게 종료
	// 새 연결 거부, 기존 연결은 완료될 때까지 대기 (최대 5초)
	// 컨텍스트 타임아웃 초과 시 강제 종료
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("❌ Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server exited gracefully")
}

// ============================================================
// Helper Functions (헬퍼 함수)
// ============================================================
// getEnv: 환경 변수를 읽고, 없으면 기본값을 반환하는 유틸리티 함수
//
// 매개변수:
//   - key string: 환경 변수 이름
//   - defaultValue string: 기본값
//
// 반환값:
//   - string: 환경 변수 값 또는 기본값
//
// 사용 이유:
// - 환경별 설정 분리 (개발/스테이징/프로덕션)
// - 12-Factor App 원칙 준수
// - Docker/Kubernetes 환경에서 설정 주입 용이
func getEnv(key, defaultValue string) string {
	// os.Getenv: 환경 변수 값 읽기
	// 환경 변수가 없으면 빈 문자열("") 반환
	if value := os.Getenv(key); value != "" {
		return value // 환경 변수가 있으면 그 값 반환
	}
	return defaultValue // 없으면 기본값 반환
}
