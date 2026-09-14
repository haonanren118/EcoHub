"use client";

import React, { useCallback } from "react";
import { Button, Space, Tooltip } from "antd";
import {
  CopyOutlined,
  ExportOutlined,
  PlaySquareOutlined,
  DesktopOutlined,
} from "@ant-design/icons";
import { useAppMessage } from "@/lib/useAppMessage";
import styles from "./index.module.less";

interface ExternalPlayerBarProps {
  streamPath?: string;
  filmName?: string;
  episodeName?: string;
}

const ExternalPlayerBar: React.FC<ExternalPlayerBarProps> = ({
  streamPath,
  filmName,
  episodeName,
}) => {
  const { message } = useAppMessage();

  const getFullStreamUrl = useCallback(
    (withRedirect = false) => {
      if (!streamPath) return "";
      let path = streamPath;
      if (withRedirect && !path.includes("redirect=1")) {
        path += (path.includes("?") ? "&" : "?") + "redirect=1";
      }
      if (typeof window !== "undefined") {
        return window.location.origin + path;
      }
      return path;
    },
    [streamPath],
  );

  const handleCopyLink = () => {
    const url = getFullStreamUrl(false);
    if (!url) {
      message.error("播放链接不可用");
      return;
    }
    if (navigator.clipboard) {
      navigator.clipboard
        .writeText(url)
        .then(() => message.success("流媒体播放直链已复制到剪贴板！"))
        .catch(() => message.error("复制失败，请手动复制"));
    } else {
      message.info("当前环境不支持自动复制");
    }
  };

  const handleOpenIINA = () => {
    const url = getFullStreamUrl(true);
    if (!url) return;
    window.location.href = `iina://weblink?url=${encodeURIComponent(url)}`;
  };

  const handleOpenPotPlayer = () => {
    const url = getFullStreamUrl(true);
    if (!url) return;
    window.location.href = `potplayer://${url}`;
  };

  const handleOpenVLC = () => {
    const url = getFullStreamUrl(true);
    if (!url) return;
    window.location.href = `vlc://${url}`;
  };

  const handleOpenInNewTab = () => {
    const url = getFullStreamUrl(true);
    if (!url) return;
    window.open(url, "_blank", "noopener,noreferrer");
  };

  if (!streamPath) return null;

  return (
    <div className={styles.barContainer}>
      <div className={styles.leftGroup}>
        <Tooltip title="复制完整的视频代理流直链，可直接在第三方播放器或下载器中播放">
          <Button
            size="small"
            icon={<CopyOutlined />}
            className={styles.actionBtn}
            onClick={handleCopyLink}
          >
            复制流直链
          </Button>
        </Tooltip>

        <Tooltip title="一键使用 macOS 原生播放器 IINA 打开（完美支持高码率 MKV / 杜比全景声 / 外挂字幕）">
          <Button
            size="small"
            icon={<PlaySquareOutlined />}
            className={styles.actionBtn}
            onClick={handleOpenIINA}
          >
            IINA 播放
          </Button>
        </Tooltip>

        <Tooltip title="一键使用 Windows 播放器 PotPlayer 打开">
          <Button
            size="small"
            icon={<DesktopOutlined />}
            className={styles.actionBtn}
            onClick={handleOpenPotPlayer}
          >
            PotPlayer
          </Button>
        </Tooltip>

        <Tooltip title="一键调用 VLC 播放器">
          <Button
            size="small"
            className={styles.actionBtn}
            onClick={handleOpenVLC}
          >
            VLC
          </Button>
        </Tooltip>

        <Tooltip title="直接在新标签页打开或下载原始视频流">
          <Button
            size="small"
            icon={<ExportOutlined />}
            className={styles.actionBtn}
            onClick={handleOpenInNewTab}
          >
            原画新标签
          </Button>
        </Tooltip>
      </div>

      <div className={styles.hintText}>
        <span>如遇杜比多声道音频静音或格式限制，推荐使用外部播放器秒播</span>
      </div>
    </div>
  );
};

export default ExternalPlayerBar;
