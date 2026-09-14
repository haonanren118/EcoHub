package scraper

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// SupportedVideoExts 支持的视频格式后缀
var SupportedVideoExts = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".ts":   true,
	".mov":  true,
	".avi":  true,
	".flv":  true,
	".wmv":  true,
	".iso":  true,
	".m4v":  true,
	".webm": true,
	".strm": true,
}

// IsVideoFile 判断是否为支持的视频文件
func IsVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return SupportedVideoExts[ext]
}

// ParsedMedia 文件名解析结果
type ParsedMedia struct {
	RawFilename string // 原始文件名
	Title       string // 解析出的标准片名（中文或英文主标题）
	Year        string // 上映年份（若有）
	Season      int    // 季号（电影默认为 0，剧集默认为 1+）
	Episode     int    // 集号（电影默认为 0，剧集默认为 1+）
	IsMovie     bool   // 是否为单片电影
	Resolution  string // 分辨率 (4K, 1080p 等)
}

var (
	// 匹配季集：S01E02, s1e2, EP03, ep4, E05, 第06集, 第7话
	seasonEpRegex = regexp.MustCompile(`(?i)[sS](\d{1,2})[eE](\d{1,3})|(?i)(?:EP|E)(\d{1,3})|第\s*(\d{1,3})\s*[集话期]`)
	// 匹配单独的季：Season 01, S02
	seasonOnlyRegex = regexp.MustCompile(`(?i)(?:Season|S)\s*(\d{1,2})`)
	// 匹配年份：(2023) 或 .2023. 或 [2023]
	yearRegex = regexp.MustCompile(`[\. \(\[\-_](19\d\d|20\d\d)[\. \)\]\-_]?`)
	// 匹配分辨率与规格
	resolutionRegex = regexp.MustCompile(`(?i)(4K|2160[pP]|1080[pP]|720[pP]|Remux|WEB-DL|BluRay|HDR|DoVi|H\.?26[45]|x26[45]|AAC|DTS)`)
	// 常见发布组与压制噪音词
	noiseWordsRegex = regexp.MustCompile(`(?i)(2160[pP]|1080[pP]|720[pP]|4[kK]|Remux|WEB-DL|BluRay|BD|HDTV|HDR10\+?|HDR|DoVi|DV|H\.?26[45]|x26[45]|HEVC|AVC|DDP\d?\.\d?|Atmos|AAC|FLAC|TrueHD|DTS-HD|MA|DTS|Repack|PROPER|HQ|Official).*`)
)

// ParseFilename 解析文件路径或文件名
func ParseFilename(rawPath string) ParsedMedia {
	filename := filepath.Base(rawPath)
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	res := ParsedMedia{
		RawFilename: filename,
		IsMovie:     true,
	}

	// 1. 匹配分辨率
	if rMatch := resolutionRegex.FindString(base); rMatch != "" {
		res.Resolution = strings.ToUpper(rMatch)
	}

	// 2. 匹配季集 (S01E02 / EP01 / 第01集)
	if m := seasonEpRegex.FindStringSubmatch(base); len(m) > 0 {
		res.IsMovie = false
		if m[1] != "" && m[2] != "" {
			res.Season, _ = strconv.Atoi(m[1])
			res.Episode, _ = strconv.Atoi(m[2])
		} else if m[3] != "" {
			res.Season = 1
			res.Episode, _ = strconv.Atoi(m[3])
		} else if m[4] != "" {
			res.Season = 1
			res.Episode, _ = strconv.Atoi(m[4])
		}
	} else {
		// 检查父级目录中是否包含季信息（如 /电视剧/繁花/Season 1/繁花.01.mkv）
		dir := filepath.Dir(rawPath)
		if dir != "" && dir != "." {
			parentName := filepath.Base(dir)
			if sm := seasonOnlyRegex.FindStringSubmatch(parentName); len(sm) > 1 {
				res.Season, _ = strconv.Atoi(sm[1])
				res.IsMovie = false
			}
		}
	}

	// 3. 提取年份
	if ym := yearRegex.FindStringSubmatch(base); len(ym) > 1 {
		res.Year = ym[1]
	}

	// 4. 清理标题（剔除季集、年份及后续规格噪音）
	cleanTitle := base

	// 剔除规格与发布组噪音
	cleanTitle = noiseWordsRegex.ReplaceAllString(cleanTitle, "")

	// 剔除季集信息
	cleanTitle = seasonEpRegex.ReplaceAllString(cleanTitle, "")

	// 剔除年份
	if res.Year != "" {
		cleanTitle = strings.ReplaceAll(cleanTitle, res.Year, "")
	}

	// 替换常见分隔符为规范空格
	cleanTitle = strings.ReplaceAll(cleanTitle, ".", " ")
	cleanTitle = strings.ReplaceAll(cleanTitle, "_", " ")
	cleanTitle = strings.ReplaceAll(cleanTitle, "[", " ")
	cleanTitle = strings.ReplaceAll(cleanTitle, "]", " ")
	cleanTitle = strings.ReplaceAll(cleanTitle, "(", " ")
	cleanTitle = strings.ReplaceAll(cleanTitle, ")", " ")
	cleanTitle = strings.ReplaceAll(cleanTitle, "-", " ")

	// 压缩连续空格
	spaceRegex := regexp.MustCompile(`\s+`)
	cleanTitle = spaceRegex.ReplaceAllString(cleanTitle, " ")
	cleanTitle = strings.TrimSpace(cleanTitle)

	if cleanTitle == "" {
		cleanTitle = strings.TrimSpace(strings.TrimSuffix(filename, ext))
	}
	res.Title = cleanTitle

	return res
}
