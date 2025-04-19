package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// User 结构体定义
type User struct {
	Username string `json:"username"`
}

// validateParams 校验参数
func validateParams(username, sleepTimeStr string) (int, error) {
	if username == "" {
		return 0, gin.Error{
			Err:  nil,
			Type: gin.ErrorTypePublic,
			Meta: "Missing username",
		}
	}

	sleepTime, err := strconv.Atoi(sleepTimeStr)
	if err != nil || sleepTime < 0 {
		return 0, gin.Error{
			Err:  nil,
			Type: gin.ErrorTypePublic,
			Meta: "Invalid sleep time",
		}
	}

	return sleepTime, nil
}

// getUserHandler 处理 /blog/user 路由
func getUserHandler(c *gin.Context) {
	// 获取参数
	username := c.DefaultQuery("username", "")
	sleepTimeStr := c.DefaultQuery("sleep_time", "0")

	// 校验参数
	sleepTime, err := validateParams(username, sleepTimeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 记录日志
	log.Printf("Request from %s: username=%s, sleep_time=%d", c.ClientIP(), username, sleepTime)

	// 模拟 sleep 操作
	time.Sleep(time.Duration(sleepTime) * time.Second)

	// 返回 JSON 响应
	c.JSON(http.StatusOK, User{Username: username})
}

// 定义全局指标集合
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, path and status",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration distribution",
			Buckets: []float64{0.1, 0.5, 1, 2, 5}, // 自定义耗时分布桶
		},
		[]string{"method", "path"},
	)

	httpResponseStatus = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_response_status_total",
			Help: "Count of HTTP response status codes",
		},
		[]string{"status"},
	)
)

func init() {
	// 注册自定义指标（包含默认 Go 运行时指标）
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		httpResponseStatus,
	)
}

// Prometheus 监控中间件
func promMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath() // 获取定义的路由路径（非实际请求路径）

		// 处理未匹配路由的情况
		if path == "" {
			path = "unmatched_route"
		}

		// 执行后续处理链
		c.Next()

		// 计算耗时
		duration := time.Since(start).Seconds()
		status := http.StatusText(c.Writer.Status())

		// 记录所有维度指标
		labels := prometheus.Labels{
			"method": c.Request.Method,
			"path":   path,
			"status": status,
		}

		httpRequestsTotal.With(labels).Inc()
		httpRequestDuration.With(prometheus.Labels{
			"method": c.Request.Method,
			"path":   path,
		}).Observe(duration)
		httpResponseStatus.With(prometheus.Labels{
			"status": status,
		}).Inc()
	}
}

func main() {
	// 创建 Gin 路由
	r := gin.New()

	// 中间件执行顺序建议
	r.Use(
		gin.Recovery(),   // 优先处理 panic
		promMiddleware(), // 监控埋点
		gin.Logger(),     // 访问日志
	)

	// 示例路由
	r.GET("/blog/metrics", gin.WrapH(promhttp.Handler()))
	// 注册路由
	r.GET("/blog/user", getUserHandler)
	r.GET("/blog/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// 启动服务器
	log.Println("Server starting on port 8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
