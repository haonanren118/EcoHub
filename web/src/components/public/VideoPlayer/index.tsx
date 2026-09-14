"use client";

import React, { useEffect, useRef, useState } from "react";
import Artplayer from "artplayer";
import Hls from "hls.js";
import { Button, Result } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import styles from "./index.module.less";

interface VideoPlayerProps {
  src: string;
  poster?: string;
  autoplay?: boolean;
  initialTime?: number;
  onEnded?: () => void;
  onTimeUpdate?: (currentTime: number, duration: number) => void;
  onError?: (error: any) => void;
}

/**
 * 极简播放器组件：
 * 采用 "src as key" 策略。
 * 核心原则：加载失败即展示 UI 报错与重试按钮，不进行静默处理。
 */
const VideoPlayer: React.FC<VideoPlayerProps> = ({
  src,
  poster,
  autoplay = true,
  initialTime = 0,
  onEnded,
  onTimeUpdate,
  onError,
}) => {
  const artRef = useRef<HTMLDivElement>(null);
  const playerRef = useRef<Artplayer | null>(null);
  const hlsRef = useRef<Hls | null>(null);
  const [hasError, setHasError] = useState(false);
  const [retryCount, setRetryCount] = useState(0);

  // 回调保护
  const callbacks = useRef({ onEnded, onTimeUpdate, onError });
  useEffect(() => {
    callbacks.current = { onEnded, onTimeUpdate, onError };
  }, [onEnded, onTimeUpdate, onError]);

  // 核心初始化 Effect
  useEffect(() => {
    const isMobile = /iPhone|iPad|iPod|Android/i.test(navigator.userAgent) || window.innerWidth <= 768;
    Artplayer.PLAYBACK_RATE = [0.5, 0.75, 1, 1.25, 1.5, 2, 2.5, 3];
    const art = new Artplayer({
      container: artRef.current,
      url: src,
      poster: poster || "",
      autoplay,
      theme: "#fa8c16", // 直接使用 Hex 色值，Artplayer 内部无法解析 CSS 变量
      volume: 0.7,
      pip: false,
      autoMini: false,
      setting: true,
      playbackRate: true,
      aspectRatio: true,
      fullscreen: true,
      fullscreenWeb: !isMobile,
      mutex: true,
      backdrop: false, // 禁用全屏 dim 遮罩，避免移动端（华为/手机浏览器）全屏变暗及拦截 Header 点击
      playsInline: true,
      // initialTime > 0 时业务层已指定跳转位置，关闭 autoPlayback 避免两套机制冲突；
      // 否则开启，让 Artplayer 以视频 URL 为 key 自动恢复每集各自的播放进度
      autoPlayback: initialTime <= 0,
      airplay: true,
      useSSR: typeof window === "undefined",
      customType: {
        m3u8: function (video: HTMLMediaElement, url: string) {
          if (Hls.isSupported()) {
            if (hlsRef.current) hlsRef.current.destroy();
            const hls = new Hls({
              enableWorker: true,
              maxBufferSize: 100 * 1024 * 1024, // 100MB 缓冲区
              maxBufferLength: 60, // 增加预加载时长到 60s
              fragLoadingMaxRetry: 10, // 增加切片下载重载次数
              levelLoadingMaxRetry: 10,
              manifestLoadingMaxRetry: 10,
              startLevel: -1, // 自动选择最佳清晰度
              abrEwmaDefaultEstimate: 5000000, // 初始带宽预估 (5Mbps)
              testBandwidth: true, // 加强带宽测试
            });
            hlsRef.current = hls;
            hls.loadSource(url);
            hls.attachMedia(video);

            // 监听 HLS 致命错误，只要报错立刻展示 UI
            hls.on(Hls.Events.ERROR, (_, data) => {
              if (data.fatal) {
                console.error("HLS Fatal Error:", data);
                // 忽略非关键错误，尝试自动恢复
                if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
                  hls.startLoad();
                } else if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
                  hls.recoverMediaError();
                } else {
                  setHasError(true);
                  callbacks.current.onError?.(data);
                }
              }
            });
          } else if (video.canPlayType("application/vnd.apple.mpegurl")) {
            video.src = url;
          }
          // 对移动端进行标准行内播放标签补全
          video.setAttribute("playsinline", "true");
          video.setAttribute("webkit-playsinline", "true");
          if (/MicroMessenger/i.test(navigator.userAgent)) {
            video.setAttribute("x5-video-player-type", "h5");
            video.setAttribute("x5-video-player-fullscreen", "true");
          }
        },
      },
    });

    if (art.template?.$video) {
      const v = art.template.$video;
      v.setAttribute("playsinline", "true");
      v.setAttribute("webkit-playsinline", "true");
      v.setAttribute("x5-playsinline", "true");
      v.setAttribute("x5-video-player-type", "h5");
      v.setAttribute("x5-video-player-fullscreen", "false");
    }

    playerRef.current = art;

    // 小窗触发：用 IntersectionObserver + 滚动监听，避免 rAF 常驻循环占 CPU
    let miniRafId = 0;
    let onScrollOrResize: (() => void) | null = null;
    let miniObserver: IntersectionObserver | null = null;
    if (!isMobile && artRef.current) {
      const updateMiniState = () => {
        if (!artRef.current || !art) return;
        if (art.fullscreen || art.fullscreenWeb) {
          if (art.mini) art.mini = false;
          return;
        }
        const rect = artRef.current.getBoundingClientRect();
        if (rect.bottom < 100) {
          if (art.playing && !art.mini) art.mini = true;
        } else if (rect.top > -50) {
          if (art.mini) art.mini = false;
        }
      };
      const scheduleMiniCheck = () => {
        if (miniRafId) return;
        miniRafId = requestAnimationFrame(() => {
          miniRafId = 0;
          updateMiniState();
        });
      };
      onScrollOrResize = scheduleMiniCheck;
      window.addEventListener("scroll", scheduleMiniCheck, { passive: true });
      window.addEventListener("resize", scheduleMiniCheck);
      if (typeof IntersectionObserver !== "undefined") {
        miniObserver = new IntersectionObserver(scheduleMiniCheck, {
          root: null,
          rootMargin: "100px 0px 100px 0px",
          threshold: [0, 0.01, 1],
        });
        miniObserver.observe(artRef.current);
      }
      scheduleMiniCheck();
    }

    art.on("ready", () => {
      if (art.template?.$video) {
        const v = art.template.$video;
        v.setAttribute("playsinline", "true");
        v.setAttribute("webkit-playsinline", "true");
        v.setAttribute("x5-playsinline", "true");
      }
      if (initialTime > 0) {
        art.currentTime = initialTime;
      }
    });

    art.on("video:ended", () => callbacks.current.onEnded?.());
    art.on("video:timeupdate", () =>
      callbacks.current.onTimeUpdate?.(art.currentTime, art.duration),
    );

    // 监听播放器通用错误
    art.on("error", (err) => {
      console.error("Artplayer Error:", err);
      setHasError(true);
      callbacks.current.onError?.(err);
    });

    art.on("video:playing", () => setHasError(false));

    // 移动端横屏接管：进入全屏时强制横屏，退出时解锁
    const isIOS =
      /iPhone|iPad|iPod/i.test(navigator.userAgent) ||
      // iPadOS 13+ 伪装成 Mac，用触摸点判断
      (navigator.platform === "MacIntel" && (navigator as any).maxTouchPoints > 1);

    const lockLandscape = () => {
      try {
        const so = (screen as any).orientation;
        if (so && typeof so.lock === "function") {
          // 仅在真全屏上下文中可用（Android Chrome 生效；iOS/非安全上下文静默失败）
          so.lock("landscape").catch(() => {});
        }
      } catch {
        /* 不支持则忽略，依赖设备物理旋转 */
      }
    };

    const unlockOrientation = () => {
      try {
        const so = (screen as any).orientation;
        if (so && typeof so.unlock === "function") so.unlock();
      } catch {
        /* ignore */
      }
    };

    art.on("fullscreen", (state: boolean) => {
      if (!isMobile) return;
      const video = art.template?.$video as any;
      if (state) {
        // iOS：标准 requestFullscreen 不会旋转到横屏，必须用原生 webkit 全屏才能横屏播放
        if (isIOS && video?.webkitEnterFullscreen) {
          video.webkitEnterFullscreen();
        } else {
          lockLandscape();
        }
      } else {
        unlockOrientation();
      }
    });

    return () => {
      if (miniRafId) cancelAnimationFrame(miniRafId);
      if (onScrollOrResize) {
        window.removeEventListener("scroll", onScrollOrResize);
        window.removeEventListener("resize", onScrollOrResize);
      }
      miniObserver?.disconnect();
      if (hlsRef.current) {
        hlsRef.current.destroy();
        hlsRef.current = null;
      }
      if (art) {
        if (art.mini) art.mini = false;
        art.destroy(true);
      }
      playerRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [retryCount, src]);

  useEffect(() => {
    if (playerRef.current && poster) playerRef.current.poster = poster;
  }, [poster]);

  return (
    <div className={styles.playerWrapper}>
      <div
        ref={artRef}
        style={{
          width: "100%",
          height: "100%",
          display: hasError ? "none" : "block",
        }}
      />
      {hasError && (
        <div className={styles.errorOverlay}>
          <Result
            status="error"
            title="视频加载失败"
            subTitle="该视频源可能已失效或受到环境策略限制，请尝试切换播放源或重新加载。"
            extra={[
              <Button
                type="primary"
                key="retry"
                icon={<ReloadOutlined />}
                className={styles.retryBtn}
                onClick={() => {
                  setHasError(false);
                  setRetryCount((p) => p + 1);
                }}
              >
                立即重试
              </Button>,
              <Button
                key="back"
                ghost
                className={styles.backBtn}
                onClick={() => window.location.reload()}
              >
                刷新页面
              </Button>,
            ]}
          />
        </div>
      )}
    </div>
  );
};

export default VideoPlayer;
