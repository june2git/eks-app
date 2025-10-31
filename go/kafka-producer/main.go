package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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
}

// ProducerService Kafka Producer 서비스
type ProducerService struct {
	producer sarama.SyncProducer
	topic    string
}

// NewProducerService Producer 서비스 생성
func NewProducerService(brokers []string, topic string) (*ProducerService, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll       // 모든 ISR 확인 (데이터 안정성)
	config.Producer.Retry.Max = 5                          // 최대 5번 재시도
	config.Producer.Return.Successes = true                // 성공 메시지 반환
	config.Producer.Compression = sarama.CompressionSnappy // Snappy 압축

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	log.Printf("✅ Kafka Producer initialized: brokers=%v topic=%s", brokers, topic)
	return &ProducerService{
		producer: producer,
		topic:    topic,
	}, nil
}

// SendMessage 메시지 전송
func (ps *ProducerService) SendMessage(msg Message) (partition int32, offset int64, err error) {
	// 메시지를 JSON으로 직렬화
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Kafka 메시지 생성
	kafkaMsg := &sarama.ProducerMessage{
		Topic: ps.topic,
		Key:   sarama.StringEncoder(msg.Key),
		Value: sarama.ByteEncoder(msgBytes),
	}

	// 메시지 전송 (동기)
	partition, offset, err = ps.producer.SendMessage(kafkaMsg)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to send message: %w", err)
	}

	log.Printf("📤 Message sent: topic=%s partition=%d offset=%d key=%s",
		ps.topic, partition, offset, msg.Key)
	return partition, offset, nil
}

// Close Producer 종료
func (ps *ProducerService) Close() error {
	return ps.producer.Close()
}

func main() {
	// 환경 변수 로드
	brokers := getEnv("KAFKA_BROKERS", "kafka-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092")
	topic := getEnv("KAFKA_TOPIC", "demo-events")
	port := getEnv("PORT", "8080")

	// Producer 초기화
	producerService, err := NewProducerService([]string{brokers}, topic)
	if err != nil {
		log.Fatalf("❌ Failed to create producer: %v", err)
	}
	defer producerService.Close()

	// Gin 라우터 설정
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Health check 엔드포인트
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "kafka-producer",
			"kafka": gin.H{
				"brokers": brokers,
				"topic":   topic,
			},
		})
	})

	// 메시지 전송 API
	router.POST("/send", func(c *gin.Context) {
		var msg Message
		if err := c.ShouldBindJSON(&msg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request body",
				"details": err.Error(),
			})
			return
		}

		// Timestamp 자동 설정
		if msg.Timestamp.IsZero() {
			msg.Timestamp = time.Now()
		}

		// 메시지 전송
		partition, offset, err := producerService.SendMessage(msg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to send message",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "success",
			"partition": partition,
			"offset":    offset,
			"message":   msg,
		})
	})

	// 배치 메시지 전송 API
	router.POST("/send/batch", func(c *gin.Context) {
		var messages []Message
		if err := c.ShouldBindJSON(&messages); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request body",
				"details": err.Error(),
			})
			return
		}

		results := make([]gin.H, 0, len(messages))
		successCount := 0
		failCount := 0

		for i, msg := range messages {
			if msg.Timestamp.IsZero() {
				msg.Timestamp = time.Now()
			}

			partition, offset, err := producerService.SendMessage(msg)
			if err != nil {
				failCount++
				results = append(results, gin.H{
					"index":  i,
					"status": "failed",
					"error":  err.Error(),
				})
			} else {
				successCount++
				results = append(results, gin.H{
					"index":     i,
					"status":    "success",
					"partition": partition,
					"offset":    offset,
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"total":   len(messages),
			"success": successCount,
			"failed":  failCount,
			"results": results,
		})
	})

	// 루트 엔드포인트
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

	// HTTP 서버 시작
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Graceful shutdown 처리
	go func() {
		log.Printf("🚀 Kafka Producer server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed: %v", err)
		}
	}()

	// 종료 시그널 대기
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
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
