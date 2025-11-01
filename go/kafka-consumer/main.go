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
	"sync"          // 동기화 프리미티브 (Mutex, WaitGroup 등)
	"syscall"       // 시스템 콜 상수
	"time"          // 시간 관련 기능

	"github.com/IBM/sarama"    // Kafka 클라이언트 라이브러리
	"github.com/gin-gonic/gin" // Gin 웹 프레임워크 (빠르고 간결한 HTTP 라우터)
)

// ============================================================
// Struct Definitions (구조체 정의)
// ============================================================

// Message: Kafka에서 수신한 메시지 구조체
// Producer의 Message와 유사하지만 Partition, Offset 정보가 추가됨
// JSON 태그: JSON 직렬화/역직렬화 시 사용할 필드 이름 정의
type Message struct {
	Key       string                 `json:"key"`                // 메시지 키
	Value     string                 `json:"value"`              // 메시지 본문 (JSON 문자열)
	Timestamp time.Time              `json:"timestamp"`          // 메시지 생성 시각
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // 파싱된 JSON 데이터
	Partition int32                  `json:"partition"`          // 메시지가 저장된 파티션 번호
	Offset    int64                  `json:"offset"`             // 파티션 내 메시지 위치
}

// ConsumerService: Kafka Consumer 기능을 제공하는 서비스 구조체
// Producer와 다르게 Consumer Group, 메시지 저장소, 동기화 기능 포함
type ConsumerService struct {
	consumer      sarama.ConsumerGroup // Kafka Consumer Group 인스턴스
	topic         string               // 구독할 토픽 이름
	groupID       string               // Consumer Group ID
	messages      []Message            // 메모리에 저장된 메시지 목록 (in-memory cache)
	messagesMutex sync.RWMutex         // 메시지 슬라이스 동시 접근 보호용 뮤텍스
	maxMessages   int                  // 최대 저장 메시지 수 (메모리 제한)
}

// sync.RWMutex: 읽기-쓰기 뮤텍스
// - 여러 고루틴이 동시에 읽기 가능 (RLock)
// - 쓰기는 독점적 (Lock) - 다른 읽기/쓰기 차단
// 사용 이유: HTTP API와 Consumer 고루틴이 동시에 messages 슬라이스에 접근

// ============================================================
// Constructor Function (생성자 함수)
// ============================================================
// NewConsumerService: ConsumerService 인스턴스를 생성하는 생성자 함수
// Go에는 생성자가 없으므로 관례적으로 New로 시작하는 함수를 생성자로 사용
//
// 매개변수:
//   - brokers []string: Kafka 브로커 주소 목록
//   - topic string: 구독할 토픽 이름
//   - groupID string: Consumer Group ID (같은 그룹의 Consumer들이 파티션을 분담)
//   - maxMessages int: 메모리에 저장할 최대 메시지 수
//
// 반환값:
//   - *ConsumerService: 생성된 서비스의 포인터
//   - error: 에러 (없으면 nil)
func NewConsumerService(brokers []string, topic, groupID string, maxMessages int) (*ConsumerService, error) {
	// Kafka Consumer 설정 생성
	config := sarama.NewConfig()

	// Consumer Group Rebalance 전략: Round Robin
	// Rebalance: Consumer가 추가/제거될 때 파티션을 재분배하는 과정
	// Round Robin: 파티션을 Consumer들에게 순환 방식으로 균등 분배
	// 다른 옵션: Range(연속된 파티션 할당), Sticky(기존 할당 유지)
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()

	// 초기 오프셋: OffsetNewest (최신 메시지부터 읽기)
	// OffsetNewest: Consumer Group이 처음 시작할 때 최신 메시지부터 읽음
	// OffsetOldest: 토픽의 가장 오래된 메시지부터 읽음
	// 주의: 이미 커밋된 오프셋이 있으면 그 위치부터 읽음
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	// 에러를 채널로 반환
	// true: Consumer 에러를 Errors() 채널로 전달 (별도 고루틴에서 처리 필요)
	config.Consumer.Return.Errors = true

	// Kafka Consumer Group 생성
	// Consumer Group: 같은 groupID를 가진 Consumer들이 파티션을 나눠서 처리
	// 장점: 수평 확장 가능, 고가용성 (Consumer 장애 시 자동 리밸런싱)
	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		// Consumer Group 생성 실패 시 에러 반환
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	// 성공 로그 출력
	log.Printf("✅ Kafka Consumer initialized: brokers=%v topic=%s groupID=%s", brokers, topic, groupID)

	// ConsumerService 인스턴스 반환
	return &ConsumerService{
		consumer: consumer,
		topic:    topic,
		groupID:  groupID,
		// make: 슬라이스 생성
		// 0: 초기 길이 (비어있음)
		// maxMessages: 용량(capacity) - 메모리 사전 할당으로 성능 최적화
		messages:    make([]Message, 0, maxMessages),
		maxMessages: maxMessages,
	}, nil
}

// ============================================================
// Method Functions (메서드 함수)
// ============================================================

// AddMessage: 메모리에 메시지를 추가하는 메서드 (FIFO 큐 방식)
// Thread-safe: 뮤텍스로 동시성 제어
//
// 동작:
//  1. 새 메시지를 슬라이스 끝에 추가
//  2. 최대 개수 초과 시 가장 오래된 메시지(맨 앞) 삭제
//
// 매개변수:
//   - msg Message: 추가할 메시지
//
// 포인터 리시버(*ConsumerService): 원본 구조체를 수정하기 위함
func (cs *ConsumerService) AddMessage(msg Message) {
	// Lock: 쓰기 잠금 (다른 모든 접근 차단)
	// 동시에 하나의 고루틴만 messages 슬라이스에 쓰기 가능
	cs.messagesMutex.Lock()

	// defer: 함수 종료 시 자동으로 Unlock 실행
	// 장점: 에러가 발생해도 반드시 잠금 해제 (데드락 방지)
	defer cs.messagesMutex.Unlock()

	// append: 슬라이스에 요소 추가
	// Go의 슬라이스는 동적 배열로, 용량 초과 시 자동으로 재할당
	cs.messages = append(cs.messages, msg)

	// 최대 메시지 개수 초과 시 오래된 메시지 삭제 (FIFO)
	// len: 슬라이스 길이
	if len(cs.messages) > cs.maxMessages {
		// 슬라이스 재슬라이싱: [1:] = 첫 번째 요소 제거, 나머지 반환
		// 예: [A, B, C][1:] → [B, C]
		cs.messages = cs.messages[1:]
	}
}

// GetMessages: 저장된 메시지를 조회하는 메서드
// Thread-safe: 읽기 잠금으로 동시 읽기 허용
//
// 동작: 최신 메시지부터 limit 개수만큼 반환
//
// 매개변수:
//   - limit int: 조회할 최대 메시지 수
//
// 반환값:
//   - []Message: 메시지 슬라이스 (최신 메시지가 먼저)
func (cs *ConsumerService) GetMessages(limit int) []Message {
	// RLock: 읽기 잠금
	// 장점: 여러 고루틴이 동시에 읽기 가능 (쓰기만 차단)
	// 읽기가 많은 경우 Lock보다 성능이 좋음
	cs.messagesMutex.RLock()
	defer cs.messagesMutex.RUnlock()

	// limit 유효성 검사
	if limit <= 0 || limit > len(cs.messages) {
		limit = len(cs.messages) // 전체 메시지 반환
	}

	// 최신 메시지부터 limit 개수만큼 선택
	// 예: 10개 메시지 중 최신 3개 → start = 10 - 3 = 7
	start := len(cs.messages) - limit
	if start < 0 {
		start = 0 // 메시지가 limit보다 적은 경우
	}

	// 새 슬라이스 생성 및 복사
	// make: limit 크기의 슬라이스 생성
	result := make([]Message, limit)
	// copy: 슬라이스 복사 (원본 보호)
	// cs.messages[start:]: start부터 끝까지 슬라이싱
	copy(result, cs.messages[start:])

	// 역순으로 정렬 (최신 메시지가 먼저 오도록)
	// 알고리즘: 양 끝부터 중간까지 swap
	// 예: [A, B, C, D] → [D, C, B, A]
	for i := 0; i < len(result)/2; i++ {
		// Go의 다중 할당: 한 줄에 swap 가능
		result[i], result[len(result)-1-i] = result[len(result)-1-i], result[i]
	}

	return result
}

// GetMessageCount: 저장된 메시지 총 개수를 반환하는 메서드
// Thread-safe: 읽기 잠금
//
// 반환값:
//   - int: 메시지 개수
func (cs *ConsumerService) GetMessageCount() int {
	cs.messagesMutex.RLock()
	defer cs.messagesMutex.RUnlock()
	return len(cs.messages)
}

// ClearMessages: 모든 메시지를 삭제하는 메서드
// Thread-safe: 쓰기 잠금
//
// 용도: 메모리 정리, 테스트 초기화 등
func (cs *ConsumerService) ClearMessages() {
	cs.messagesMutex.Lock()
	defer cs.messagesMutex.Unlock()
	// 새로운 빈 슬라이스 생성 (기존 메시지는 GC에 의해 정리됨)
	cs.messages = make([]Message, 0, cs.maxMessages)
}

// Start: Consumer를 시작하는 메서드 (비동기)
// 고루틴을 사용하여 백그라운드에서 메시지 수신
//
// 매개변수:
//   - ctx context.Context: 컨텍스트 (취소 신호 전달용)
//
// 반환값:
//   - error: 초기화 에러 (Consumer 시작 후 에러는 별도 채널로 처리)
//
// Context 패턴:
//   - ctx.Done(): 취소/타임아웃 신호를 채널로 받음
//   - 부모가 cancel() 호출 시 모든 자식 고루틴에 종료 신호 전달
func (cs *ConsumerService) Start(ctx context.Context) error {
	// Consumer Group Handler 생성
	// &: 구조체 포인터 생성
	// handler는 Kafka 메시지를 처리하는 콜백 인터페이스 구현
	handler := &consumerGroupHandler{service: cs}

	// 첫 번째 고루틴: 메시지 수신 루프
	go func() {
		// 무한 루프: Consumer는 계속 실행되어야 함
		for {
			// Consume: 토픽에서 메시지를 수신하고 handler에 전달
			// Rebalance 발생 시 이 함수가 반환되고 다시 호출됨
			// handler.ConsumeClaim()이 실제 메시지 처리 담당
			if err := cs.consumer.Consume(ctx, []string{cs.topic}, handler); err != nil {
				log.Printf("❌ Consumer error: %v", err)
			}

			// Context가 취소되면 종료
			// ctx.Err(): 컨텍스트 에러 확인 (취소/타임아웃 시 nil 아님)
			if ctx.Err() != nil {
				return // 고루틴 종료
			}
		}
	}()

	// 두 번째 고루틴: 에러 채널 모니터링
	// Consumer.Errors() 채널에서 에러를 받아 로깅
	go func() {
		// range: 채널이 닫힐 때까지 값을 계속 받음
		// 채널이 닫히면 루프 종료
		for err := range cs.consumer.Errors() {
			log.Printf("⚠️ Consumer error: %v", err)
		}
	}()

	return nil // 시작 성공 (실제 수신은 고루틴에서 비동기 처리)
}

// Close: Consumer 리소스 정리 및 연결 종료 메서드
//
// 동작:
//   - Consumer Group 탈퇴
//   - Kafka 연결 종료
//   - 에러 채널 닫기
//
// 반환값:
//   - error: 종료 중 발생한 에러
func (cs *ConsumerService) Close() error {
	// 내부 Consumer 종료
	// Consumer Group에서 탈퇴하고 리소스 정리
	return cs.consumer.Close()
}

// ============================================================
// Consumer Group Handler (Consumer Group 핸들러)
// ============================================================
// consumerGroupHandler: sarama.ConsumerGroupHandler 인터페이스 구현
//
// 인터페이스 메서드:
//   - Setup: 리밸런싱 전 호출
//   - Cleanup: 리밸런싱 후 호출
//   - ConsumeClaim: 실제 메시지 처리
//
// Consumer Group 라이프사이클:
//  1. Setup() 호출
//  2. ConsumeClaim() 실행 (할당된 파티션별로)
//  3. Rebalance 발생 시 Cleanup() 호출
//  4. 다시 1번부터 반복
type consumerGroupHandler struct {
	service *ConsumerService // ConsumerService 참조 (메시지 저장용)
}

// Setup: Consumer Group 리밸런싱 전 호출되는 콜백 메서드
//
// 용도:
//   - 초기화 작업
//   - 리밸런싱 로그 출력
//   - 상태 초기화 등
//
// 매개변수:
//   - sarama.ConsumerGroupSession: Consumer Group 세션 (사용 안 함)
//
// 반환값:
//   - error: 에러 발생 시 Consumer 종료
//
// Rebalance: Consumer 추가/제거/장애 시 파티션 재분배
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	log.Println("🔄 Consumer group rebalancing...")
	return nil // 에러 없음
}

// Cleanup: Consumer Group 리밸런싱 후 호출되는 콜백 메서드
//
// 용도:
//   - 정리 작업
//   - 리밸런싱 완료 로그
//   - 임시 리소스 해제 등
//
// 매개변수:
//   - sarama.ConsumerGroupSession: Consumer Group 세션 (사용 안 함)
//
// 반환값:
//   - error: 에러 (보통 nil)
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	log.Println("✅ Consumer group rebalance complete")
	return nil
}

// ConsumeClaim: 실제 메시지를 소비하는 메서드 (핵심 로직)
// Kafka에서 메시지를 받아 처리하는 메인 루프
//
// 매개변수:
//   - session sarama.ConsumerGroupSession: Consumer Group 세션 (오프셋 커밋용)
//   - claim sarama.ConsumerGroupClaim: 파티션 클레임 (메시지 채널 제공)
//
// 반환값:
//   - error: 처리 중 에러
//
// 동작:
//  1. claim.Messages() 채널에서 메시지 수신
//  2. JSON 파싱 및 Message 구조체 생성
//  3. 메모리에 저장 (AddMessage)
//  4. 오프셋 커밋 (MarkMessage)
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// 무한 루프: 메시지가 올 때까지 대기
	for {
		// select: 여러 채널 중 준비된 것 하나 선택 (non-blocking)
		select {
		// case 1: 메시지 채널에서 메시지 수신
		// <-: 채널에서 값 받기 (블로킹)
		case message := <-claim.Messages():
			// nil 체크: 채널이 닫힌 경우
			if message == nil {
				return nil // 정상 종료
			}

			// JSON 파싱 (역직렬화)
			// var: map[string]interface{} 타입 변수 선언
			// interface{}: 모든 타입을 담을 수 있는 빈 인터페이스
			var msgData map[string]interface{}

			// Unmarshal: JSON 바이트 → Go 데이터 구조
			if err := json.Unmarshal(message.Value, &msgData); err != nil {
				// JSON 파싱 실패 시 원본 메시지를 "raw" 필드에 저장
				log.Printf("⚠️ Failed to unmarshal message: %v", err)
				msgData = map[string]interface{}{
					"raw": string(message.Value),
				}
			}

			// Message 구조체 생성
			// Kafka 메시지 → 애플리케이션 Message 타입 변환
			msg := Message{
				Key:       string(message.Key),   // 메시지 키 (바이트 → 문자열)
				Value:     string(message.Value), // 메시지 값 (JSON 문자열)
				Timestamp: message.Timestamp,     // 메시지 타임스탬프
				Partition: message.Partition,     // 파티션 번호
				Offset:    message.Offset,        // 오프셋 (파티션 내 위치)
				Metadata:  msgData,               // 파싱된 JSON 데이터
			}

			// 메시지를 메모리에 저장
			// Thread-safe: AddMessage 내부에서 뮤텍스 처리
			h.service.AddMessage(msg)

			// 로그 출력: 메시지 수신 정보
			log.Printf("📥 Message received: topic=%s partition=%d offset=%d key=%s",
				message.Topic, message.Partition, message.Offset, string(message.Key))

			// 오프셋 커밋 (마킹)
			// MarkMessage: 이 메시지까지 처리 완료했음을 표시
			// 실제 커밋은 주기적으로 일괄 처리됨
			// 장점: 재시작 시 이 오프셋부터 읽음 (중복 처리 최소화)
			session.MarkMessage(message, "") // 두 번째 인자: 메타데이터 (사용 안 함)

		// case 2: Context 종료 신호
		// session.Context().Done(): Context 취소/타임아웃 채널
		case <-session.Context().Done():
			return nil // Consumer 종료
		}
	}
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
	brokers := getEnv("KAFKA_BROKERS", "kafka-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092")
	topic := getEnv("KAFKA_TOPIC", "demo-events")
	groupID := getEnv("KAFKA_GROUP_ID", "kafka-consumer-group")
	port := getEnv("PORT", "8080")
	maxMessages := 1000 // 메모리에 저장할 최대 메시지 수 (FIFO 큐)

	// ========================================
	// 2. Kafka Consumer 초기화
	// ========================================
	consumerService, err := NewConsumerService([]string{brokers}, topic, groupID, maxMessages)
	if err != nil {
		log.Fatalf("❌ Failed to create consumer: %v", err)
	}
	// defer: main 함수 종료 시 Consumer 연결 종료
	defer consumerService.Close()

	// ========================================
	// 3. Consumer 시작 (백그라운드)
	// ========================================
	// context.WithCancel: 취소 가능한 컨텍스트 생성
	// ctx: 자식 컨텍스트 (고루틴에 전달)
	// cancel: 취소 함수 (호출 시 모든 자식 고루틴에 종료 신호)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // main 함수 종료 시 모든 고루틴에 취소 신호 전달

	// Consumer 시작 (비동기 - 고루틴에서 실행)
	if err := consumerService.Start(ctx); err != nil {
		log.Fatalf("❌ Failed to start consumer: %v", err)
	}

	// ========================================
	// 4. Gin 웹 프레임워크 설정
	// ========================================
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// ========================================
	// 5. HTTP API 엔드포인트 정의
	// ========================================

	// GET /health: 헬스체크 엔드포인트
	// Kubernetes liveness/readiness probe에서 사용
	// Consumer 상태 및 메시지 수 확인
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "kafka-consumer",
			"kafka": gin.H{
				"brokers": brokers,
				"topic":   topic,
				"groupID": groupID, // Consumer Group ID
			},
			"messages": gin.H{
				"count": consumerService.GetMessageCount(), // 현재 저장된 메시지 수
				"max":   maxMessages,                       // 최대 저장 가능 메시지 수
			},
		})
	})

	// GET /messages: 저장된 메시지 조회 API
	// Query Parameter: limit (조회할 메시지 수, 기본값 100)
	// 최신 메시지부터 반환
	router.GET("/messages", func(c *gin.Context) {
		limit := 100 // 기본값

		// c.Query: URL 쿼리 파라미터 읽기
		// 예: /messages?limit=50
		if l := c.Query("limit"); l != "" {
			// fmt.Sscanf: 문자열을 포맷에 맞게 파싱
			// %d: 정수
			// &limit: limit 변수의 주소 (값을 수정하기 위함)
			fmt.Sscanf(l, "%d", &limit)
		}

		// 메시지 조회
		messages := consumerService.GetMessages(limit)

		// 응답
		c.JSON(http.StatusOK, gin.H{
			"count":    len(messages),                     // 반환된 메시지 수
			"total":    consumerService.GetMessageCount(), // 전체 저장된 메시지 수
			"messages": messages,                          // 메시지 배열
		})
	})

	// GET /stats: Consumer 통계 정보 API
	// 토픽, 그룹 ID, 메시지 수 등 메타 정보 제공
	router.GET("/stats", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"topic":         topic,                             // 구독 중인 토픽
			"group_id":      groupID,                           // Consumer Group ID
			"message_count": consumerService.GetMessageCount(), // 현재 메시지 수
			"max_messages":  maxMessages,                       // 최대 저장 가능 수
		})
	})

	// DELETE /messages: 저장된 모든 메시지 삭제 API
	// 메모리 정리 또는 테스트 초기화 용도
	router.DELETE("/messages", func(c *gin.Context) {
		consumerService.ClearMessages()
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "All messages cleared",
		})
	})

	// GET /: 루트 엔드포인트 (API 정보 제공)
	// 사용 가능한 API 엔드포인트 목록 반환
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "kafka-consumer",
			"version": "1.0.0",
			"endpoints": gin.H{
				"health":   "GET /health",             // 헬스체크
				"messages": "GET /messages?limit=100", // 메시지 조회
				"stats":    "GET /stats",              // 통계 정보
				"clear":    "DELETE /messages",        // 메시지 삭제
			},
		})
	})

	// ========================================
	// 6. HTTP 서버 설정 및 시작
	// ========================================
	srv := &http.Server{
		Addr:    ":" + port, // 리슨 주소
		Handler: router,     // Gin 라우터를 HTTP 핸들러로 사용
	}

	// ========================================
	// 7. Graceful Shutdown 구현
	// ========================================
	// Graceful Shutdown: 진행 중인 요청을 완료한 후 서버 종료
	// 중요: Consumer와 HTTP 서버를 모두 안전하게 종료해야 함

	// 고루틴: HTTP 서버 시작 (메인 고루틴을 블로킹하지 않기 위함)
	go func() {
		log.Printf("🚀 Kafka Consumer server starting on port %s", port)

		// ListenAndServe: HTTP 서버 시작 (블로킹)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed: %v", err)
		}
	}()

	// 종료 시그널 대기
	// make(chan os.Signal, 1): 버퍼 크기 1인 시그널 채널 생성
	quit := make(chan os.Signal, 1)

	// signal.Notify: OS 시그널을 채널로 받도록 설정
	// SIGINT: Ctrl+C (2)
	// SIGTERM: kill 명령 (15) - Kubernetes Pod 종료 시 사용
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// <-quit: 시그널이 올 때까지 블로킹 (대기)
	// 시그널 수신 → 이 라인 이후 코드 실행 (종료 프로세스 시작)
	<-quit

	log.Println("🛑 Shutting down server...")

	// ========================================
	// 8. 리소스 정리 (순서 중요)
	// ========================================

	// 1단계: Consumer 종료 (먼저 종료)
	// cancel(): Context 취소 → Consumer 고루틴에 종료 신호 전달
	// 이유: 새로운 메시지 수신 중단, 진행 중인 메시지 처리 완료
	cancel()

	// 2단계: HTTP 서버 종료
	// context.WithTimeout: 5초 타임아웃 컨텍스트 생성
	// 진행 중인 HTTP 요청 완료를 기다림 (최대 5초)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel() // 함수 종료 시 컨텍스트 리소스 정리

	// srv.Shutdown: 서버 안전 종료
	// 새 연결 거부, 기존 연결은 완료될 때까지 대기
	// 타임아웃 초과 시 강제 종료
	if err := srv.Shutdown(shutdownCtx); err != nil {
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
//   - 환경별 설정 분리 (개발/스테이징/프로덕션)
//   - 12-Factor App 원칙 준수
//   - Docker/Kubernetes 환경에서 설정 주입 용이
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value // 환경 변수가 있으면 그 값 반환
	}
	return defaultValue // 없으면 기본값 반환
}
