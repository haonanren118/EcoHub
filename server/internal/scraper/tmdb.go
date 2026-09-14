package scraper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TmdbItem 统一的 TMDB 影视元数据条目
type TmdbItem struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`         // 电影名
	Name         string  `json:"name"`          // 电视剧名
	OriginalName string  `json:"original_name"` // 原始标题
	PosterPath   string  `json:"poster_path"`   // 竖版海报路径 (/xxx.jpg)
	BackdropPath string  `json:"backdrop_path"` // 横版背景图路径 (/xxx.jpg)
	Overview     string  `json:"overview"`      // 中文简介
	VoteAverage  float64 `json:"vote_average"`  // 评分 (0-10)
	ReleaseDate  string  `json:"release_date"`  // 上映日期 (YYYY-MM-DD)
	FirstAirDate string  `json:"first_air_date"`// 电视剧首播日期
	GenreIDs     []int   `json:"genre_ids"`     // 分类 ID
}

// DisplayTitle 获取有效标题（中文优先）
func (t *TmdbItem) DisplayTitle() string {
	if t.Title != "" {
		return t.Title
	}
	if t.Name != "" {
		return t.Name
	}
	return t.OriginalName
}

// ReleaseYear 获取 4 位年份
func (t *TmdbItem) ReleaseYear() string {
	date := t.ReleaseDate
	if date == "" {
		date = t.FirstAirDate
	}
	if len(date) >= 4 {
		return date[:4]
	}
	return ""
}

// TmdbClient TMDB 官方接口封装客户端
type TmdbClient struct {
	ApiKey       string
	BaseURL      string
	ImageBaseURL string
	httpClient   *http.Client
}

// NewTmdbClient 创建客户端实例
func NewTmdbClient(apiKey, baseURL string) *TmdbClient {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = "https://api.themoviedb.org"
	}
	return &TmdbClient{
		ApiKey:       apiKey,
		BaseURL:      base,
		ImageBaseURL: "https://image.tmdb.org/t/p",
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// FullPosterURL 拼装完整海报图地址 (w500)
func (c *TmdbClient) FullPosterURL(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return fmt.Sprintf("%s/w500%s", c.ImageBaseURL, path)
}

// FullBackdropURL 拼装完整背景图地址 (original)
func (c *TmdbClient) FullBackdropURL(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return fmt.Sprintf("%s/original%s", c.ImageBaseURL, path)
}

// SearchMovie 搜索电影
func (c *TmdbClient) SearchMovie(query, year string) (*TmdbItem, error) {
	return c.search("/3/search/movie", query, year)
}

// SearchTV 搜索电视剧
func (c *TmdbClient) SearchTV(query, year string) (*TmdbItem, error) {
	return c.search("/3/search/tv", query, year)
}

// SearchAuto 智能搜索：先按猜测类型搜索，未命中自动反向重试
func (c *TmdbClient) SearchAuto(meta ParsedMedia) (*TmdbItem, error) {
	if meta.IsMovie {
		item, err := c.SearchMovie(meta.Title, meta.Year)
		if err == nil && item != nil {
			return item, nil
		}
		// 容错重试：电视剧
		return c.SearchTV(meta.Title, meta.Year)
	}

	item, err := c.SearchTV(meta.Title, meta.Year)
	if err == nil && item != nil {
		return item, nil
	}
	// 容错重试：电影
	return c.SearchMovie(meta.Title, meta.Year)
}

func (c *TmdbClient) search(endpoint, query, year string) (*TmdbItem, error) {
	if c.ApiKey == "" {
		return nil, fmt.Errorf("tmdb api key is empty")
	}

	params := url.Values{}
	params.Set("api_key", c.ApiKey)
	params.Set("query", query)
	params.Set("language", "zh-CN")
	if year != "" {
		params.Set("year", year)
		params.Set("first_air_date_year", year)
	}

	reqURL := fmt.Sprintf("%s%s?%s", c.BaseURL, endpoint, params.Encode())
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb api returned status: %d", resp.StatusCode)
	}

	var data struct {
		Results []TmdbItem `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if len(data.Results) == 0 {
		return nil, fmt.Errorf("no results found for: %s", query)
	}

	return &data.Results[0], nil
}

// TmdbDetail 完整详情信息（包含演职员与类型标签）
type TmdbDetail struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Name        string  `json:"name"`
	Overview    string  `json:"overview"`
	VoteAverage float64 `json:"vote_average"`
	Genres      []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	Credits struct {
		Cast []struct {
			Name string `json:"name"`
		} `json:"cast"`
		Crew []struct {
			Name string `json:"name"`
			Job  string `json:"job"`
		} `json:"crew"`
	} `json:"credits"`
}

// GetCreditsFormatted 格式化主演与导演
func (d *TmdbDetail) GetCreditsFormatted() (actors, directors string) {
	actList := make([]string, 0, 5)
	for i, c := range d.Credits.Cast {
		if i >= 5 {
			break
		}
		actList = append(actList, c.Name)
	}
	actors = strings.Join(actList, ", ")

	dirList := make([]string, 0, 2)
	for _, cr := range d.Credits.Crew {
		if cr.Job == "Director" {
			dirList = append(dirList, cr.Name)
		}
	}
	directors = strings.Join(dirList, ", ")
	return
}
