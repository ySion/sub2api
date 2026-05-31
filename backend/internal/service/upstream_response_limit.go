package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
)

var ErrUpstreamResponseBodyTooLarge = errors.New("upstream response body too large")

const (
	nonStreamKeepaliveActiveContextKey = "sub2api_nonstream_keepalive_active"
	nonStreamKeepaliveStopContextKey   = "sub2api_nonstream_keepalive_stop"
)

// defaultUpstreamResponseReadMaxBytes 源自 config.DefaultUpstreamResponseReadMaxBytes，
// 仅在 cfg 为 nil 时作为兜底（测试或极端场景）。
const defaultUpstreamResponseReadMaxBytes = config.DefaultUpstreamResponseReadMaxBytes

// DefaultNonStreamKeepaliveIntervalSeconds 是非流式 keepalive 的默认间隔。
// CPA 的同类实现使用“间隔秒数 <= 0 即关闭”；sub2api 额外拆成显式开关，
// 因此 interval <= 0 时回落到 30 秒，保证后台选项有稳定默认值。
const DefaultNonStreamKeepaliveIntervalSeconds = 30

// NonStreamKeepaliveSettings 描述非流式请求等待期间的下游连接保活策略。
//
// 这个功能用于非流式上游请求已经进入执行阶段、但下游暂时没有任何字节可写的窗口：
// - 不放在全局中间件中，避免提前 flush 导致账号 failover 和错误状态映射失效。
// - 每次心跳只写入一个空行，JSON 客户端通常会忽略响应体前的 JSON whitespace。
// - 第一次心跳 flush 后，HTTP 状态码和已设置的响应头会被提交；所以默认关闭。
type NonStreamKeepaliveSettings struct {
	Enabled         bool
	IntervalSeconds int
}

func normalizeNonStreamKeepaliveSettings(settings NonStreamKeepaliveSettings) NonStreamKeepaliveSettings {
	if settings.IntervalSeconds <= 0 {
		settings.IntervalSeconds = DefaultNonStreamKeepaliveIntervalSeconds
	}
	return settings
}

func nonStreamKeepaliveSettingsFromConfig(cfg *config.Config) NonStreamKeepaliveSettings {
	if cfg == nil {
		return NonStreamKeepaliveSettings{Enabled: false, IntervalSeconds: DefaultNonStreamKeepaliveIntervalSeconds}
	}
	return normalizeNonStreamKeepaliveSettings(NonStreamKeepaliveSettings{
		Enabled:         cfg.Gateway.NonStreamKeepaliveEnabled,
		IntervalSeconds: cfg.Gateway.NonStreamKeepaliveIntervalSeconds,
	})
}

func resolveNonStreamKeepaliveSettings(ctx context.Context, cfg *config.Config, settingService *SettingService) NonStreamKeepaliveSettings {
	if settingService != nil {
		return settingService.GetNonStreamKeepaliveSettings(ctx)
	}
	return nonStreamKeepaliveSettingsFromConfig(cfg)
}

func resolveUpstreamResponseReadLimit(cfg *config.Config) int64 {
	if cfg != nil && cfg.Gateway.UpstreamResponseReadMaxBytes > 0 {
		return cfg.Gateway.UpstreamResponseReadMaxBytes
	}
	return defaultUpstreamResponseReadMaxBytes
}

func readUpstreamResponseBodyLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	if reader == nil {
		return nil, errors.New("response body is nil")
	}
	if maxBytes <= 0 {
		maxBytes = defaultUpstreamResponseReadMaxBytes
	}

	body, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("%w: limit=%d", ErrUpstreamResponseBodyTooLarge, maxBytes)
	}
	return body, nil
}

func startNonStreamKeepalive(c *gin.Context, settings NonStreamKeepaliveSettings) func() {
	settings = normalizeNonStreamKeepaliveSettings(settings)
	if !settings.Enabled || c == nil || c.Writer == nil || c.Writer.Written() {
		return func() {}
	}
	if active, _ := c.Get(nonStreamKeepaliveActiveContextKey); active == true {
		if stopValue, ok := c.Get(nonStreamKeepaliveStopContextKey); ok {
			if stop, ok := stopValue.(func()); ok && stop != nil {
				return stop
			}
		}
		return func() {}
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return func() {}
	}

	// Keepalive 的第一个空行会提交当前响应头。多数非流式网关响应都是 JSON；
	// 若调用方尚未设置 Content-Type，这里先写入 application/json，避免 net/http
	// 根据空行嗅探成 text/plain。调用方可在启动 keepalive 前自行覆盖更精确的响应头。
	if c.Writer.Header().Get("Content-Type") == "" {
		c.Writer.Header().Set("Content-Type", "application/json")
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	var stopOnce sync.Once
	interval := time.Duration(settings.IntervalSeconds) * time.Second
	requestDone := context.Background().Done()
	if c.Request != nil {
		requestDone = c.Request.Context().Done()
	}
	stopFunc := func() {
		stopOnce.Do(func() {
			close(stopCh)
			<-doneCh
			c.Set(nonStreamKeepaliveActiveContextKey, false)
			c.Set(nonStreamKeepaliveStopContextKey, func() {})
		})
	}
	c.Set(nonStreamKeepaliveActiveContextKey, true)
	c.Set(nonStreamKeepaliveStopContextKey, stopFunc)

	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// 主 goroutine 此时只阻塞在上游 body 读取；stop 函数会等待这里退出，
				// 避免最终 JSON body 与 keepalive 空行并发写入 gin.ResponseWriter。
				if _, err := c.Writer.Write([]byte("\n")); err != nil {
					return
				}
				flusher.Flush()
			case <-requestDone:
				return
			case <-stopCh:
				return
			}
		}
	}()

	return stopFunc
}

// StartGatewayNonStreamKeepalive starts the same blank-line keepalive used by
// ReadUpstreamResponseBodyWithGatewayNonStreamKeepalive, but at the beginning of
// a non-stream upstream request. This matches CPA's effective coverage for
// clients/proxies such as Cloudflare that time out while no response headers or
// body bytes have been sent yet.
func StartGatewayNonStreamKeepalive(ctx context.Context, c *gin.Context, cfg *config.Config, settingService *SettingService) func() {
	settings := resolveNonStreamKeepaliveSettings(ctx, cfg, settingService)
	return startNonStreamKeepalive(c, settings)
}

// TooLargeWriter 在响应超限时向客户端写格式化的错误响应。
type TooLargeWriter func(c *gin.Context)

// ReadUpstreamResponseBody 读取上游非流式响应体。
// 超限时自动记录 ops error 并调用 onTooLarge 向客户端写错误。
func ReadUpstreamResponseBody(reader io.Reader, cfg *config.Config, c *gin.Context, onTooLarge TooLargeWriter) ([]byte, error) {
	maxBytes := resolveUpstreamResponseReadLimit(cfg)
	body, err := readUpstreamResponseBodyLimited(reader, maxBytes)
	if err != nil {
		if errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			setOpsUpstreamError(c, http.StatusBadGateway, "upstream response too large", "")
			if onTooLarge != nil {
				onTooLarge(c)
			}
		}
		return nil, err
	}
	return body, nil
}

// ReadUpstreamResponseBodyWithNonStreamKeepalive 读取上游非流式响应体，并按需发送空行保活。
//
// 若同一 gin.Context 已经通过 StartGatewayNonStreamKeepalive 启动了请求级保活，
// 这里会复用并在 body 读完后停止它，确保最终 JSON 写回前没有保活 goroutine
// 继续写空行。不要在账号选择或全局中间件中调用：一旦空行被 flush，HTTP
// 状态码和响应头就已经提交，后续错误只能作为 body 写回。
func ReadUpstreamResponseBodyWithNonStreamKeepalive(
	reader io.Reader,
	cfg *config.Config,
	c *gin.Context,
	settings NonStreamKeepaliveSettings,
	onTooLarge TooLargeWriter,
) ([]byte, error) {
	stopKeepalive := startNonStreamKeepalive(c, settings)
	defer stopKeepalive()
	return ReadUpstreamResponseBody(reader, cfg, c, onTooLarge)
}

// ReadUpstreamResponseBodyWithGatewayNonStreamKeepalive 从后台“网关服务”设置解析 keepalive。
// 若后台 SettingService 不可用，则回落到 config.yaml 的 gateway.nonstream_keepalive_*。
func ReadUpstreamResponseBodyWithGatewayNonStreamKeepalive(
	ctx context.Context,
	reader io.Reader,
	cfg *config.Config,
	c *gin.Context,
	settingService *SettingService,
	onTooLarge TooLargeWriter,
) ([]byte, error) {
	settings := resolveNonStreamKeepaliveSettings(ctx, cfg, settingService)
	return ReadUpstreamResponseBodyWithNonStreamKeepalive(reader, cfg, c, settings, onTooLarge)
}

// anthropicTooLargeError 以 Anthropic Messages API 格式写入超限错误。
func anthropicTooLargeError(c *gin.Context) {
	c.JSON(http.StatusBadGateway, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    "upstream_error",
			"message": "Upstream response too large",
		},
	})
}

// openAITooLargeError 以 OpenAI / Gemini 格式写入超限错误。
func openAITooLargeError(c *gin.Context) {
	c.JSON(http.StatusBadGateway, gin.H{
		"error": gin.H{
			"type":    "upstream_error",
			"message": "Upstream response too large",
		},
	})
}
