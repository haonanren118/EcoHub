package handler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"server/internal/infra/db"
	"server/internal/model"
	"server/internal/model/dto"
	"server/internal/scraper"

	"github.com/gin-gonic/gin"
)

var (
	scanTaskCancels sync.Map // map[uint]context.CancelFunc
)

type StorageHandler struct{}

var StorageHd = new(StorageHandler)

// List 查询存储源列表
func (h *StorageHandler) List(c *gin.Context) {
	var list []model.StorageSource
	if err := db.Mdb.Order("id DESC").Find(&list).Error; err != nil {
		dto.Failed("获取存储源失败: "+err.Error(), c)
		return
	}
	dto.Success(list, "获取存储源列表成功", c)
}

type SaveStorageRequest struct {
	ID          uint              `json:"id"`
	Name        string            `json:"name" binding:"required"`
	Type        model.StorageType `json:"type"`
	Endpoint    string            `json:"endpoint" binding:"required"`
	RootPath    string            `json:"rootPath"`
	Username    string            `json:"username"`
	Password    string            `json:"password"`
	TestConnect bool              `json:"testConnect"`
}

// Save 新增或更新存储源
func (h *StorageHandler) Save(c *gin.Context) {
	var req SaveStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Failed("参数错误: "+err.Error(), c)
		return
	}

	req.Endpoint = strings.TrimRight(req.Endpoint, "/")
	if req.RootPath == "" {
		req.RootPath = "/"
	}
	if req.Type == "" {
		req.Type = model.StorageTypeWebDAV
	}

	// 若需测试连接
	if req.TestConnect && req.Type == model.StorageTypeWebDAV {
		client := scraper.NewWebdavClient(req.Endpoint, req.Username, req.Password)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		if err := client.Ping(ctx); err != nil {
			dto.Failed("WebDAV 连接测试失败: "+err.Error(), c)
			return
		}
	}

	var source model.StorageSource
	if req.ID > 0 {
		if err := db.Mdb.First(&source, req.ID).Error; err != nil {
			dto.Failed("存储源不存在", c)
			return
		}
		source.Name = req.Name
		source.Type = req.Type
		source.Endpoint = req.Endpoint
		source.RootPath = req.RootPath
		source.Username = req.Username
		if req.Password != "" {
			source.Password = req.Password
		}
		if err := db.Mdb.Save(&source).Error; err != nil {
			dto.Failed("更新存储源失败: "+err.Error(), c)
			return
		}
	} else {
		source = model.StorageSource{
			Name:       req.Name,
			Type:       req.Type,
			Endpoint:   req.Endpoint,
			RootPath:   req.RootPath,
			Username:   req.Username,
			Password:   req.Password,
			ScanStatus: "idle",
		}
		if err := db.Mdb.Create(&source).Error; err != nil {
			dto.Failed("创建存储源失败: "+err.Error(), c)
			return
		}
	}

	dto.Success(source, "保存存储源成功", c)
}

// TestConnection 独立测试连接接口
func (h *StorageHandler) TestConnection(c *gin.Context) {
	var req SaveStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Failed("参数错误: "+err.Error(), c)
		return
	}

	req.Endpoint = strings.TrimRight(req.Endpoint, "/")
	if req.Password == "" && req.ID > 0 {
		var existing model.StorageSource
		if err := db.Mdb.First(&existing, req.ID).Error; err == nil {
			req.Password = existing.Password
		}
	}

	client := scraper.NewWebdavClient(req.Endpoint, req.Username, req.Password)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		dto.Failed("连接失败: "+err.Error(), c)
		return
	}
	dto.Success(nil, "WebDAV 连接成功", c)
}

// Delete 删除存储源
func (h *StorageHandler) Delete(c *gin.Context) {
	idStr := c.Query("id")
	id, _ := strconv.ParseUint(idStr, 10, 64)
	if id == 0 {
		dto.Failed("无效的 ID", c)
		return
	}

	if err := db.Mdb.Delete(&model.StorageSource{}, id).Error; err != nil {
		dto.Failed("删除存储源失败: "+err.Error(), c)
		return
	}

	// 异步清理关联影片
	sourceID := fmt.Sprintf("storage_%d", id)
	go func() {
		_ = db.Mdb.Where("source_id = ?", sourceID).Delete(&model.FilmIndex{}).Error
		_ = db.Mdb.Where("source_id = ?", sourceID).Delete(&model.MovieDetailInfo{}).Error
	}()

	dto.Success(nil, "删除存储源成功", c)
}

// Scan 触发异步扫描与刮削
func (h *StorageHandler) Scan(c *gin.Context) {
	idStr := c.Query("id")
	id, _ := strconv.ParseUint(idStr, 10, 64)
	if id == 0 {
		dto.Failed("无效的 ID", c)
		return
	}

	var source model.StorageSource
	if err := db.Mdb.First(&source, id).Error; err != nil {
		dto.Failed("存储源不存在", c)
		return
	}

	if source.ScanStatus == "scanning" {
		dto.Failed("该存储源正在扫描中，请勿重复触发", c)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	scanTaskCancels.Store(source.ID, cancel)

	go func(s model.StorageSource) {
		defer scanTaskCancels.Delete(s.ID)
		pipeline := scraper.NewPipeline(&s)
		if err := pipeline.Run(ctx); err != nil {
			log.Printf("[StorageHandler] 存储源 %s (ID=%d) 扫描任务退出: %v", s.Name, s.ID, err)
		}
	}(source)

	dto.Success(nil, "扫描任务已在后台启动", c)
}

// StopScan 停止正在运行的扫描任务
func (h *StorageHandler) StopScan(c *gin.Context) {
	idStr := c.Query("id")
	id, _ := strconv.ParseUint(idStr, 10, 64)
	if id == 0 {
		dto.Failed("无效的 ID", c)
		return
	}

	uintID := uint(id)
	// 1. 若内存中有 cancel 函数，调用取消通知协程退出
	if cancelVal, ok := scanTaskCancels.Load(uintID); ok {
		if cancel, ok := cancelVal.(context.CancelFunc); ok {
			cancel()
		}
		scanTaskCancels.Delete(uintID)
	}

	// 2. 无论内存中任务是否还在（如服务重启后的孤儿状态），强制将数据库中的状态重置为 idle
	updates := map[string]any{
		"scan_status": "idle",
		"last_error":  "已手动停止扫描",
	}
	_ = db.Mdb.Model(&model.StorageSource{}).Where("id = ?", uintID).Updates(updates).Error

	dto.Success(nil, "已成功停止扫描任务", c)
}
