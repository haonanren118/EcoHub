package model

import "gorm.io/gorm"


type StorageType string

const (
	StorageTypeWebDAV StorageType = "webdav"
	StorageTypeLocal  StorageType = "local"
)

// StorageSource WebDAV 或本地挂载存储源
type StorageSource struct {
	gorm.Model
	Name       string      `gorm:"size:64;not null" json:"name"`           // 挂载点名称 (如：Alist-115, NAS)
	Type       StorageType `gorm:"size:32;default:webdav" json:"type"`     // webdav | local
	Endpoint   string      `gorm:"size:256;not null" json:"endpoint"`      // WebDAV 服务地址 (如 http://127.0.0.1:5244/dav)
	RootPath   string      `gorm:"size:256;default:/" json:"rootPath"`     // 挂载扫描根目录
	Username   string      `gorm:"size:64" json:"username"`                // 认证用户名
	Password   string      `gorm:"size:128" json:"password"`               // 认证密码
	ScanStatus string      `gorm:"size:32;default:idle" json:"scanStatus"` // idle | scanning | failed | success
	LastScanAt int64       `json:"lastScanAt"`                             // 上次扫描完成时间戳
	FileCount  int64       `json:"fileCount"`                              // 扫描到的视频文件总数
	MovieCount int64       `json:"movieCount"`                             // 成功入库的影片数
	LastError  string      `gorm:"size:512" json:"lastError"`              // 最近一次错误信息
}

func (StorageSource) TableName() string {
	return TableStorageSource
}
