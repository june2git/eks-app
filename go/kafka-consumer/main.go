package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
)

// Message Kafka 메시지 구조체
type Message struct {
	Key       string                 `json:"key"`
	Value     string                 `json:"value"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Partition int32                  `json:"partition"`
	Offset    int64                  `json:"offset"`
}

// ConsumerService Kafka Consumer 서비스
type ConsumerService struct {
	consumer      sarama.ConsumerGroup
	topic         string
	groupID       string
	messages      []Message
	messagesMutex sync.RWMutex
	maxMessages   int
}

// NewConsumerService Consumer 서비스 생성
func NewConsumerService(brokers []string, topic, groupID string, maxMessages int) (*ConsumerService, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest // 최신 메시지부터 읽기
	config.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	log.Printf("✅ Kafka Consumer initialized: brokers=%v topic=%s groupID=%s", brokers, topic, groupID)
	return &ConsumerService{
		consumer:    consumer,
		topic:       topic,
		groupID:     groupID,
		messages:    make([]Message, 0, maxMessages),
		maxMessages: maxMessages,
	}, nil
}

// AddMessage 메시지 추가 (최대 개수 유지)
func (cs *ConsumerService) AddMessage(msg Message) {
	cs.messagesMutex.Lock()
	defer cs.messagesMutex.Unlock()

	cs.messages = append(cs.messages, msg)

	// 최대 메시지 개수 초과 시 오래된 메시지 삭제
	if len(cs.messages) > cs.maxMessages {
		cs.messages = cs.messages[1:]
	}
}

// GetMessages 저장된 메시지 조회
func (cs *ConsumerService) GetMessages(limit int) []Message {
	cs.messagesMutex.RLock()
	defer cs.messagesMutex.RUnlock()

	if limit <= 0 || limit > len(cs.messages) {
		limit = len(cs.messages)
	}

	// 최신 메시지부터 반환
	start := len(cs.messages) - limit
	if start < 0 {
		start = 0
	}

	result := make([]Message, limit)
	copy(result, cs.messages[start:])

	// 역순으로 정렬 (최신 메시지가 먼저)
	for i := 0; i < len(result)/2; i++ {
		result[i], result[len(result)-1-i] = result[len(result)-1-i], result[i]
	}

	return result
}

// GetMessageCount 메시지 개수 반환
func (cs *ConsumerService) GetMessageCount() int {
	cs.messagesMutex.RLock()
	defer cs.messagesMutex.RUnlock()
	return len(cs.messages)
}

// ClearMessages 모든 메시지 삭제
func (cs *ConsumerService) ClearMessages() {
	cs.messagesMutex.Lock()
	defer cs.messagesMutex.Unlock()
	cs.messages = make([]Message, 0, cs.maxMessages)
}

// Start Consumer 시작
func (cs *ConsumerService) Start(ctx context.Context) error {
	handler := &consumerGroupHandler{service: cs}

	go func() {
		for {
			// 컨슈머 그룹 실행 (리밸런싱 발생 시 자동으로 재시작됨)
			if err := cs.consumer.Consume(ctx, []string{cs.topic}, handler); err != nil {
				log.Printf("❌ Consumer error: %v", err)
			}

			// Context가 취소되면 종료
			if ctx.Err() != nil {
				return
			}
		}
	}()

	// 에러 처리
	go func() {
		for err := range cs.consumer.Errors() {
			log.Printf("⚠️ Consumer error: %v", err)
		}
	}()

	return nil
}

// Close Consumer 종료
func (cs *ConsumerService) Close() error {
	return cs.consumer.Close()
}

// consumerGroupHandler Kafka Consumer Group 핸들러
type consumerGroupHandler struct {
	service *ConsumerService
}

// Setup 컨슈머 그룹 설정 (리밸런싱 전)
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	log.Println("🔄 Consumer group rebalancing...")
	return nil
}

// Cleanup 컨슈머 그룹 정리 (리밸런싱 후)
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	log.Println("✅ Consumer group rebalance complete")
	return nil
}

// ConsumeClaim 메시지 소비
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			// JSON 파싱
			var msgData map[string]interface{}
			if err := json.Unmarshal(message.Value, &msgData); err != nil {
				log.Printf("⚠️ Failed to unmarshal message: %v", err)
				msgData = map[string]interface{}{
					"raw": string(message.Value),
				}
			}

			// Message 구조체 생성
			msg := Message{
				Key:       string(message.Key),
				Value:     string(message.Value),
				Timestamp: message.Timestamp,
				Partition: message.Partition,
				Offset:    message.Offset,
				Metadata:  msgData,
			}

			// 메시지 저장
			h.service.AddMessage(msg)

			log.Printf("📥 Message received: topic=%s partition=%d offset=%d key=%s",
				message.Topic, message.Partition, message.Offset, string(message.Key))

			// 오프셋 커밋
			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
}

func main() {
	// 환경 변수 로드
	brokers := getEnv("KAFKA_BROKERS", "kafka-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092")
	topic := getEnv("KAFKA_TOPIC", "demo-events")
	groupID := getEnv("KAFKA_GROUP_ID", "kafka-consumer-group")
	port := getEnv("PORT", "8080")
	maxMessages := 1000 // 메모리에 저장할 최대 메시지 수

	// Consumer 초기화
	consumerService, err := NewConsumerService([]string{brokers}, topic, groupID, maxMessages)
	if err != nil {
		log.Fatalf("❌ Failed to create consumer: %v", err)
	}
	defer consumerService.Close()

	// Consumer 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := consumerService.Start(ctx); err != nil {
		log.Fatalf("❌ Failed to start consumer: %v", err)
	}

	// Gin 라우터 설정
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Health check 엔드포인트
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "kafka-consumer",
			"kafka": gin.H{
				"brokers": brokers,
				"topic":   topic,
				"groupID": groupID,
			},
			"messages": gin.H{
				"count": consumerService.GetMessageCount(),
				"max":   maxMessages,
			},
		})
	})

	// 메시지 조회 API
	router.GET("/messages", func(c *gin.Context) {
		limit := 100
		if l := c.Query("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}

		messages := consumerService.GetMessages(limit)
		c.JSON(http.StatusOK, gin.H{
			"count":    len(messages),
			"total":    consumerService.GetMessageCount(),
			"messages": messages,
		})
	})

	// 메시지 통계 API
	router.GET("/stats", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"topic":         topic,
			"group_id":      groupID,
			"message_count": consumerService.GetMessageCount(),
			"max_messages":  maxMessages,
		})
	})

	// 메시지 삭제 API
	router.DELETE("/messages", func(c *gin.Context) {
		consumerService.ClearMessages()
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "All messages cleared",
		})
	})

	// 루트 엔드포인트
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "kafka-consumer",
			"version": "1.0.0",
			"endpoints": gin.H{
				"health":   "GET /health",
				"messages": "GET /messages?limit=100",
				"stats":    "GET /stats",
				"clear":    "DELETE /messages",
			},
		})
	})

	// HTTP 서버 시작
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Graceful shutdown 처리
	go func() {
		log.Printf("🚀 Kafka Consumer server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed: %v", err)
		}
	}()

	// 종료 시그널 대기
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Consumer 종료
	cancel()

	// HTTP 서버 종료
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("❌ Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server exited gracefully")
}

// getEnv 환경 변수 또는 기본값 반환
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
