package handler

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"server/internal/infra/db"
	"server/internal/model"

	"github.com/gin-gonic/gin"
)

type streamCacheItem struct {
	streamURL      string
	isDirectStream bool
	expireAt       time.Time
}

type StreamHandler struct {
	cache sync.Map // map[string]*streamCacheItem
}

var StreamHd = new(StreamHandler)

// detectMimeType 根据文件名后缀智能推断合规的视频 MIME 类型，替换网盘错误的 application/oct-stream
func detectMimeType(filePath string, upstreamType string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mkv":
		return "video/x-matroska"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".flv":
		return "video/x-flv"
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	case ".m4v":
		return "video/mp4"
	}

	if upstreamType != "" &&
		!strings.Contains(upstreamType, "application/oct") &&
		upstreamType != "application/octet-stream" {
		return upstreamType
	}

	if detected := mime.TypeByExtension(ext); detected != "" {
		return detected
	}
	return "video/mp4"
}

// StreamPlayRedirect 安全重定向或流代理 WebDAV / 网盘流媒体
func (h *StreamHandler) StreamPlayRedirect(c *gin.Context) {
	// 跨域头支持（确保支持全源行内播放与第三方本地播放器拉取）
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type, Accept, Authorization")
	c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges, Content-Type")

	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusNoContent)
		return
	}

	sourceIdStr := c.Query("source_id")
	rawPath := c.Query("path")

	sourceID, _ := strconv.ParseUint(sourceIdStr, 10, 64)
	if sourceID == 0 || rawPath == "" {
		c.String(http.StatusBadRequest, "Invalid source_id or path")
		return
	}

	decodedPath, err := url.QueryUnescape(rawPath)
	if err != nil {
		decodedPath = rawPath
	}

	cleanRelPath := strings.TrimPrefix(decodedPath, "/")
	cacheKey := fmt.Sprintf("%d:%s", sourceID, cleanRelPath)

	var streamURL string
	var isDirectStream bool

	// 1. 优先查阅解析缓存，避免浏览器发送多次 Range 请求时重复消耗 2~3s 向小雅/网盘发起探测
	if cachedVal, ok := h.cache.Load(cacheKey); ok {
		item := cachedVal.(*streamCacheItem)
		if time.Now().Before(item.expireAt) {
			streamURL = item.streamURL
			isDirectStream = item.isDirectStream
		} else {
			h.cache.Delete(cacheKey)
		}
	}

	// 2. 缓存未命中时，查询存储源信息并向远端探测直链
	var source model.StorageSource
	if streamURL == "" {
		if err := db.Mdb.First(&source, sourceID).Error; err != nil {
			c.String(http.StatusNotFound, "Storage source not found")
			return
		}

		parsedEndpoint, err := url.Parse(source.Endpoint)
		if err != nil {
			c.String(http.StatusInternalServerError, "Invalid storage endpoint")
			return
		}

		targetURL := *parsedEndpoint
		targetURL.User = nil
		targetURL.Path = path.Join(parsedEndpoint.Path, cleanRelPath)
		targetURLStr := targetURL.String()

		// 若目标为 .strm 文件，读取其内部包含的流媒体真实直链
		if strings.HasSuffix(strings.ToLower(cleanRelPath), ".strm") {
			req, rErr := http.NewRequestWithContext(c.Request.Context(), "GET", targetURLStr, nil)
			if rErr == nil {
				if source.Username != "" || source.Password != "" {
					req.SetBasicAuth(source.Username, source.Password)
				}
				client := &http.Client{Timeout: 8 * time.Second}
				resp, doErr := client.Do(req)
				if doErr == nil {
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						content, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
						for _, line := range strings.Split(string(content), "\n") {
							line = strings.TrimSpace(line)
							if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
								streamURL = line
								isDirectStream = true
								break
							}
						}
					}
				}
			}
		}

		// 若非 strm，探测远端 WebDAV 是否返回 302 重定向直链（例如 Alist / 小雅）
		if streamURL == "" {
			probeClient := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
				Timeout: 6 * time.Second,
			}

			probeReq, pErr := http.NewRequestWithContext(c.Request.Context(), "HEAD", targetURLStr, nil)
			var probeResp *http.Response
			if pErr == nil {
				if source.Username != "" || source.Password != "" {
					probeReq.SetBasicAuth(source.Username, source.Password)
				}
				probeResp, err = probeClient.Do(probeReq)
				if err == nil && probeResp.StatusCode == http.StatusMethodNotAllowed {
					probeResp.Body.Close()
					probeResp = nil
				}
			}

			if probeResp == nil {
				getProbeReq, gErr := http.NewRequestWithContext(c.Request.Context(), "GET", targetURLStr, nil)
				if gErr == nil {
					if source.Username != "" || source.Password != "" {
						getProbeReq.SetBasicAuth(source.Username, source.Password)
					}
					getProbeReq.Header.Set("Range", "bytes=0-0")
					probeResp, _ = probeClient.Do(getProbeReq)
				}
			}

			streamURL = targetURLStr
			isDirectStream = false

			if probeResp != nil {
				defer probeResp.Body.Close()
				if probeResp.StatusCode >= 300 && probeResp.StatusCode < 400 {
					location := probeResp.Header.Get("Location")
					if location != "" {
						if parsedLoc, lErr := url.Parse(location); lErr == nil {
							location = parsedEndpoint.ResolveReference(parsedLoc).String()
						}
						streamURL = location
						isDirectStream = true
					}
				}

				if probeResp.StatusCode == http.StatusNotFound {
					c.String(http.StatusNotFound, "视频文件不存在于存储源中: %s", cleanRelPath)
					return
				}
			}
		}

		// 存入缓存，有效期 15 分钟（小雅网盘签名 URL 通常在 1~4 小时内有效）
		h.cache.Store(cacheKey, &streamCacheItem{
			streamURL:      streamURL,
			isDirectStream: isDirectStream,
			expireAt:       time.Now().Add(15 * time.Minute),
		})
	}

	// 3. 若前端显式指定 redirect=1（如外部播放器 IINA/PotPlayer/VLC），直接 302 重定向
	if c.Query("redirect") == "1" && isDirectStream {
		c.Header("Referrer-Policy", "no-referrer")
		c.Redirect(http.StatusFound, streamURL)
		return
	}

	// 4. 服务端流代理中继传输：
	// - 网盘 OSS 签名直链（OSS Signature）仅签名了 GET 方法，对 HEAD 请求会报 403 签名不匹配；
	//   因此无论客户端是 GET 还是 HEAD，代理向上游发起请求时统一使用 GET，避免 403 阻断；
	// - 智能修正 Content-Type 为匹配视频 MIME；
	// - 强制改写 Content-Disposition: inline 确保行内秒播；
	// - 完美透传 Range、Content-Range、Accept-Ranges
	isHeadReq := c.Request.Method == http.MethodHead
	proxyReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, streamURL, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "创建流媒体代理失败")
		return
	}

	if !isDirectStream {
		if source.ID == 0 {
			_ = db.Mdb.First(&source, sourceID)
		}
		if source.Username != "" || source.Password != "" {
			proxyReq.SetBasicAuth(source.Username, source.Password)
		}
	}

	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		proxyReq.Header.Set("Range", rangeHeader)
	} else if isHeadReq {
		// 客户端发送 HEAD 未指定 Range 时，向上游请求 1 字节探针以获取视频总长度与 ETag
		proxyReq.Header.Set("Range", "bytes=0-0")
	}

	proxyClient := &http.Client{Timeout: 0}
	proxyResp, err := proxyClient.Do(proxyReq)
	if err != nil {
		c.String(http.StatusBadGateway, "流媒体代理连接失败: %v", err)
		return
	}
	defer proxyResp.Body.Close()

	// 透传标准流媒体响应头
	for _, headerKey := range []string{"Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
		if val := proxyResp.Header.Get(headerKey); val != "" {
			c.Writer.Header().Set(headerKey, val)
		}
	}

	statusCode := proxyResp.StatusCode
	// 若客户端发送的是无 Range 的 HEAD 请求，从 Content-Range（如 bytes 0-0/2790459476）中解析出文件总大小并以 200 OK 响应
	if isHeadReq && rangeHeader == "" {
		cr := proxyResp.Header.Get("Content-Range")
		if slashIdx := strings.LastIndex(cr, "/"); slashIdx != -1 {
			totalLen := cr[slashIdx+1:]
			c.Writer.Header().Set("Content-Length", totalLen)
			c.Writer.Header().Del("Content-Range")
			statusCode = http.StatusOK
		}
	}

	c.Writer.Header().Set("Accept-Ranges", "bytes")
	c.Writer.Header().Set("Content-Disposition", "inline")
	c.Writer.Header().Set("Content-Type", detectMimeType(cleanRelPath, proxyResp.Header.Get("Content-Type")))

	c.Status(statusCode)
	if !isHeadReq {
		_, _ = io.Copy(c.Writer, proxyResp.Body)
	}
}
