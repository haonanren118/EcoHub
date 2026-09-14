package scraper

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// WebdavFileInfo WebDAV 远端文件信息
type WebdavFileInfo struct {
	Path    string    `json:"path"`    // 相对于 WebDAV root 的完整路径
	Name    string    `json:"name"`    // 文件名
	Size    int64     `json:"size"`    // 文件大小（字节）
	IsDir   bool      `json:"isDir"`   // 是否为目录
	ModTime time.Time `json:"modTime"` // 最后修改时间
}

// WebdavClient 轻量级 WebDAV 客户端
type WebdavClient struct {
	Endpoint   string // 如 http://127.0.0.1:5244/dav
	Username   string
	Password   string
	httpClient *http.Client
}

// NewWebdavClient 初始化 WebDAV 客户端
func NewWebdavClient(endpoint, username, password string) *WebdavClient {
	ep := strings.TrimRight(endpoint, "/")
	return &WebdavClient{
		Endpoint: ep,
		Username: username,
		Password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Ping 测试 WebDAV 连通性
func (c *WebdavClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", c.Endpoint, bytes.NewBufferString(propfindXML))
	if err != nil {
		return err
	}
	c.setAuth(req)
	req.Header.Set("Depth", "0")
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("WebDAV 连接失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("WebDAV 认证或服务状态异常 (HTTP %d)", resp.StatusCode)
	}
	return nil
}

// ScanVideoFiles 递归扫描指定目录下的全部支持的视频文件 (深度优先，限制最大递归深度以防死循环)
func (c *WebdavClient) ScanVideoFiles(ctx context.Context, startPath string, maxDepth int) ([]WebdavFileInfo, error) {
	var results []WebdavFileInfo
	err := c.traverse(ctx, startPath, 0, maxDepth, &results)
	return results, err
}

func (c *WebdavClient) traverse(ctx context.Context, currentPath string, depth, maxDepth int, results *[]WebdavFileInfo) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if depth > maxDepth {
		return nil
	}

	items, err := c.listDir(ctx, currentPath)
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.IsDir {
			// 跳过常见的系统隐藏目录与非媒体目录
			if strings.HasPrefix(item.Name, ".") || item.Name == "$RECYCLE.BIN" {
				continue
			}
			if err := c.traverse(ctx, item.Path, depth+1, maxDepth, results); err != nil {
				return err
			}
		} else if IsVideoFile(item.Name) {
			*results = append(*results, item)
		}
	}
	return nil
}

// XML 结构体映射 WebDAV PROPFIND 返回
type multistatusXML struct {
	XMLName   xml.Name      `xml:"multistatus"`
	Responses []responseXML `xml:"response"`
}

type responseXML struct {
	Href     string      `xml:"href"`
	Propstat propstatXML `xml:"propstat"`
}

type propstatXML struct {
	Status string  `xml:"status"`
	Prop   propXML `xml:"prop"`
}

type propXML struct {
	ResourceType  resourcetypeXML `xml:"resourcetype"`
	ContentLength int64           `xml:"getcontentlength"`
	LastModified  string          `xml:"getlastmodified"`
	DisplayName   string          `xml:"displayname"`
}

type resourcetypeXML struct {
	Collection *struct{} `xml:"collection"`
}

const propfindXML = `<?xml version="1.0" encoding="utf-8" ?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:resourcetype/>
    <D:getcontentlength/>
    <D:getlastmodified/>
    <D:displayname/>
  </D:prop>
</D:propfind>`

func escapeWebdavPath(rawPath string) string {
	rawPath = strings.Trim(rawPath, "/")
	if rawPath == "" {
		return ""
	}
	parts := strings.Split(rawPath, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func (c *WebdavClient) listDir(ctx context.Context, dirPath string) ([]WebdavFileInfo, error) {
	relPath := strings.TrimPrefix(dirPath, "/")
	reqURL := c.Endpoint
	if escaped := escapeWebdavPath(relPath); escaped != "" {
		reqURL = c.Endpoint + "/" + escaped
	}

	req, err := http.NewRequestWithContext(ctx, "PROPFIND", reqURL, bytes.NewBufferString(propfindXML))
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PROPFIND %s 失败 (HTTP %d): %s", dirPath, resp.StatusCode, string(body))
	}

	var multi multistatusXML
	if err := xml.NewDecoder(resp.Body).Decode(&multi); err != nil {
		return nil, fmt.Errorf("解析 WebDAV 响应 XML 失败: %w", err)
	}

	parsedEndpoint, _ := url.Parse(c.Endpoint)
	endpointPath := ""
	if parsedEndpoint != nil {
		endpointPath = strings.TrimRight(parsedEndpoint.Path, "/")
	}

	var items []WebdavFileInfo
	for _, r := range multi.Responses {
		decodedHref, err := url.PathUnescape(r.Href)
		if err != nil {
			decodedHref = r.Href
		}
		cleanHref := strings.TrimRight(decodedHref, "/")

		// 去除 endpoint 的前缀路径
		itemPath := cleanHref
		if endpointPath != "" && strings.HasPrefix(itemPath, endpointPath) {
			itemPath = strings.TrimPrefix(itemPath, endpointPath)
		}
		itemPath = strings.TrimPrefix(itemPath, "/")

		// 排除自身目录本身 (Depth:1 返回第一项是父级目录)
		cleanDir := strings.Trim(relPath, "/")
		if itemPath == cleanDir || itemPath == "" {
			continue
		}

		isDir := r.Propstat.Prop.ResourceType.Collection != nil
		name := r.Propstat.Prop.DisplayName
		if name == "" {
			name = path.Base(itemPath)
		}

		var modTime time.Time
		if r.Propstat.Prop.LastModified != "" {
			modTime, _ = http.ParseTime(r.Propstat.Prop.LastModified)
		}

		items = append(items, WebdavFileInfo{
			Path:    "/" + itemPath,
			Name:    name,
			Size:    r.Propstat.Prop.ContentLength,
			IsDir:   isDir,
			ModTime: modTime,
		})
	}

	return items, nil
}

func (c *WebdavClient) setAuth(req *http.Request) {
	if c.Username != "" || c.Password != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
}
