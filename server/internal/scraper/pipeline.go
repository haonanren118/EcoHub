package scraper

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"net/url"
	"sort"
	"time"

	"server/internal/infra/db"
	"server/internal/model"
	"server/internal/repository"
	"server/internal/repository/film"
)

func generateMediaID(title, year string) int64 {
	h := fnv.New64a()
	h.Write([]byte(fmt.Sprintf("%s|%s", title, year)))
	val := int64(h.Sum64() & 0x7FFFFFFFFFFFFFFF)
	return (val % 8000000000) + 1000000000
}

// Pipeline 媒体扫描与刮削流水线
type Pipeline struct {
	storage *model.StorageSource
	tmdb    *TmdbClient
	webdav  *WebdavClient
}

// NewPipeline 创建流水线
func NewPipeline(storage *model.StorageSource) *Pipeline {
	basicConfig := repository.GetSiteBasic()
	var tmdbClient *TmdbClient
	if basicConfig.TmdbApiKey != "" {
		tmdbClient = NewTmdbClient(basicConfig.TmdbApiKey, basicConfig.TmdbProxyUrl)
	}

	return &Pipeline{
		storage: storage,
		tmdb:    tmdbClient,
		webdav:  NewWebdavClient(storage.Endpoint, storage.Username, storage.Password),
	}
}

// Run 执行扫描与刮削
func (p *Pipeline) Run(ctx context.Context) error {
	p.updateStatus("scanning", "正在扫描远端目录与媒体文件...")

	// 1. 扫描 WebDAV 视频文件
	files, err := p.webdav.ScanVideoFiles(ctx, p.storage.RootPath, 8)
	if err != nil {
		p.updateStatus("failed", fmt.Sprintf("WebDAV 扫描失败: %v", err))
		return err
	}

	p.storage.FileCount = int64(len(files))
	_ = db.Mdb.Model(&model.StorageSource{}).Where("id = ?", p.storage.ID).Update("file_count", p.storage.FileCount).Error

	if len(files) == 0 {
		p.updateStatus("success", "")
		return nil
	}

	// 2. 解析文件名并按影视剧聚合分组
	type mediaGroup struct {
		meta     ParsedMedia
		episodes []model.MovieUrlInfo
	}
	groupMap := make(map[string]*mediaGroup)

	for _, f := range files {
		parsed := ParseFilename(f.Path)
		if parsed.Title == "" {
			continue
		}

		groupKey := parsed.Title
		if parsed.IsMovie && parsed.Year != "" {
			groupKey = fmt.Sprintf("%s|%s", parsed.Title, parsed.Year)
		}

		streamLink := fmt.Sprintf("/api/stream/play?source_id=%d&path=%s", p.storage.ID, url.QueryEscape(f.Path))
		epLabel := "全一集"
		if !parsed.IsMovie {
			if parsed.Season > 1 {
				epLabel = fmt.Sprintf("S%02dE%02d", parsed.Season, parsed.Episode)
			} else {
				epLabel = fmt.Sprintf("第%02d集", parsed.Episode)
			}
		}

		grp, exists := groupMap[groupKey]
		if !exists {
			grp = &mediaGroup{
				meta:     parsed,
				episodes: make([]model.MovieUrlInfo, 0, 1),
			}
			groupMap[groupKey] = grp
		}
		grp.episodes = append(grp.episodes, model.MovieUrlInfo{
			Episode: epLabel,
			Link:    streamLink,
		})
	}

	// 3. 构建 MovieDetail 并通过 TMDB 补充元数据（流式分批入库 + 断点续扫）
	sourceID := fmt.Sprintf("storage_%d", p.storage.ID)
	details := make([]model.MovieDetail, 0, 10)
	nowTime := time.Now().Format("2006-01-02 15:04:05")

	// 断点续扫：预先加载该存储源已成功入库的影片 mid 集合，避免重复耗时刮削
	existingMids := make(map[int64]bool)
	var existingMidsList []int64
	_ = db.Mdb.Model(&model.FilmIndex{}).
		Where("source_id = ?", sourceID).
		Pluck("mid", &existingMidsList).Error
	for _, mid := range existingMidsList {
		existingMids[mid] = true
	}

	totalGroups := len(groupMap)
	currentIdx := 0
	savedTotal := int64(len(existingMids))

	// flushBatch 将内存已刮削好的批次安全入库并刷新前台快照
	flushBatch := func() error {
		if len(details) == 0 {
			return nil
		}
		if err := film.SaveDetails(sourceID, details); err != nil {
			log.Printf("[Pipeline] 分批入库失败: %v", err)
			return err
		}
		savedTotal += int64(len(details))
		p.storage.MovieCount = savedTotal
		_ = db.Mdb.Model(&model.StorageSource{}).Where("id = ?", p.storage.ID).Update("movie_count", savedTotal).Error
		details = details[:0]

		// 异步轻量更新前台快照，让用户即时在前后台看到最新入库影片
		go func() {
			newVer := film.NewSnapshotVersion()
			if err := film.RebuildFilmListSnapshot(newVer); err == nil {
				_ = film.ActivateRebuiltFilmListSnapshot(newVer)
			}
		}()
		return nil
	}

	for _, grp := range groupMap {
		currentIdx++
		targetMid := generateMediaID(grp.meta.Title, grp.meta.Year)
		if existingMids[targetMid] {
			// 已经入库过，直接跳过 TMDB 刮削，实现断点续扫
			continue
		}

		select {
		case <-ctx.Done():
			// 扫描被手动停止或服务退出时，先将已刮削好的剩余影片落盘入库，杜绝数据白费
			_ = flushBatch()
			p.updateStatus("idle", fmt.Sprintf("扫描已停止，已成功入库 %d 部影片", savedTotal))
			return ctx.Err()
		default:
		}

		if p.tmdb != nil {
			p.updateProgress(fmt.Sprintf("正在断点续扫海报与详情: %d/%d (%s)", currentIdx, totalGroups, grp.meta.Title), int64(len(files)), savedTotal+int64(len(details)))
		} else {
			p.updateProgress(fmt.Sprintf("正在断点整理入库: %d/%d (%s)", currentIdx, totalGroups, grp.meta.Title), int64(len(files)), savedTotal+int64(len(details)))
		}

		// 选集排序
		sort.Slice(grp.episodes, func(i, j int) bool {
			return grp.episodes[i].Episode < grp.episodes[j].Episode
		})

		detail := model.MovieDetail{
			Id:       generateMediaID(grp.meta.Title, grp.meta.Year),
			Name:     grp.meta.Title,
			PlayFrom: []string{p.storage.Name},
			PlayList: [][]model.MovieUrlInfo{grp.episodes},
			MovieDescriptor: model.MovieDescriptor{
				Year:       grp.meta.Year,
				AddTime:    time.Now().Unix(),
				UpdateTime: nowTime,
			},
		}

		if grp.meta.IsMovie {
			detail.CName = "电影"
			detail.Pid = 1
			detail.Remarks = "高清HD"
			if grp.meta.Resolution != "" {
				detail.Remarks = grp.meta.Resolution
			}
		} else {
			detail.CName = "电视剧"
			detail.Pid = 2
			detail.Remarks = fmt.Sprintf("更新至%d集", len(grp.episodes))
		}

		// TMDB 刮削增强
		if p.tmdb != nil {
			if item, err := p.tmdb.SearchAuto(grp.meta); err == nil && item != nil {
				detail.Name = item.DisplayTitle()
				if item.ReleaseYear() != "" {
					detail.MovieDescriptor.Year = item.ReleaseYear()
				}
				if item.PosterPath != "" {
					detail.Picture = p.tmdb.FullPosterURL(item.PosterPath)
				}
				if item.BackdropPath != "" {
					detail.PictureSlide = p.tmdb.FullBackdropURL(item.BackdropPath)
				}
				if item.Overview != "" {
					detail.MovieDescriptor.Content = item.Overview
					detail.MovieDescriptor.Blurb = item.Overview
				}
				if item.VoteAverage > 0 {
					detail.MovieDescriptor.DbScore = fmt.Sprintf("%.1f", item.VoteAverage)
				}
			} else {
				log.Printf("[Pipeline] TMDB 未命中: %s (%v)", grp.meta.Title, err)
			}
		}

		details = append(details, detail)

		// 每满 10 部立即流式落盘一次，避免因长耗时中断导致数据全部丢失
		if len(details) >= 10 {
			if err := flushBatch(); err != nil {
				p.updateStatus("failed", fmt.Sprintf("分批入库失败: %v", err))
				return err
			}
		}
	}

	// 将收尾剩余不足 10 部的影片落盘入库
	if len(details) > 0 {
		if err := flushBatch(); err != nil {
			p.updateStatus("failed", fmt.Sprintf("入库失败: %v", err))
			return err
		}
	}

	p.storage.MovieCount = savedTotal
	p.storage.LastScanAt = time.Now().Unix()
	p.updateStatus("success", "")
	return nil
}

func (p *Pipeline) updateProgress(msg string, fileCount, movieCount int64) {
	p.storage.LastError = msg
	p.storage.FileCount = fileCount
	p.storage.MovieCount = movieCount
	_ = db.Mdb.Model(&model.StorageSource{}).Where("id = ?", p.storage.ID).Updates(map[string]any{
		"last_error":  msg,
		"file_count":  fileCount,
		"movie_count": movieCount,
	}).Error
}

func (p *Pipeline) updateStatus(status, lastErr string) {
	p.storage.ScanStatus = status
	p.storage.LastError = lastErr
	updates := map[string]any{
		"scan_status": status,
		"last_error":  lastErr,
	}
	if status == "success" {
		updates["last_scan_at"] = p.storage.LastScanAt
		updates["movie_count"] = p.storage.MovieCount
	}
	_ = db.Mdb.Model(&model.StorageSource{}).Where("id = ?", p.storage.ID).Updates(updates).Error
}
