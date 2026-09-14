import React from "react";
import type { MenuProps } from "antd";
import {
  HomeOutlined,
  ThunderboltOutlined,
  VideoCameraOutlined,
  FolderOpenOutlined,
  TeamOutlined,
  GlobalOutlined,
  LineChartOutlined,
  SettingOutlined,
  HddOutlined,
} from "@ant-design/icons";
import type { ThemeMode } from "@/components/theme/ThemeDock";

export type MenuItem = Required<MenuProps>["items"][number];

export const themeModeLabels: Record<ThemeMode, string> = {
  light: "浅色",
  dark: "深色",
  system: "跟随系统",
};

export const menuItems: MenuItem[] = [
  {
    key: "/manage",
    icon: <HomeOutlined />,
    label: "工作台",
  },
  {
    key: "sub-collect",
    icon: <ThunderboltOutlined />,
    label: "采集管理",
    children: [
      { key: "/manage/collect", label: <span data-tour="menu-collect">采集中心</span> },
      { key: "/manage/collect/record", label: <span data-tour="menu-record">失败记录</span> },
      { key: "/manage/cron", label: <span data-tour="menu-cron">计划任务</span> },
    ],
  },
  {
    key: "sub-film",
    icon: <VideoCameraOutlined />,
    label: "内容管理",
    children: [
      { key: "/manage/film", label: <span data-tour="menu-film">影片列表</span> },
      { key: "/manage/banners", label: "首页轮播" },
      { key: "/manage/collect/category", label: <span data-tour="menu-category">分类管理</span> },
      { key: "/manage/collect/category/rules", label: <span data-tour="menu-rules">分类规则</span> },
    ],
  },
  {
    key: "/manage/file",
    icon: <FolderOpenOutlined />,
    label: "素材中心",
  },
  {
    key: "/manage/system/users",
    icon: <TeamOutlined />,
    label: "账号管理",
  },
  {
    key: "/manage/system/website",
    icon: <GlobalOutlined />,
    label: "网站配置",
  },
  {
    key: "/manage/access",
    icon: <LineChartOutlined />,
    label: "数据分析",
  },
  {
    key: "/manage/system",
    icon: <SettingOutlined />,
    label: "系统设置",
  },
];

export function resolveMenuKey(pathname: string): string {
  if (pathname.startsWith("/manage/banners")) return "/manage/banners";
  if (pathname.startsWith("/manage/film/add")) return "/manage/film";
  if (pathname.startsWith("/manage/collect/category/rules")) return "/manage/collect/category/rules";
  if (pathname.startsWith("/manage/collect/category")) return "/manage/collect/category";
  if (pathname.startsWith("/manage/film")) return "/manage/film";
  if (pathname.startsWith("/manage/collect/record")) return "/manage/collect/record";
  if (pathname.startsWith("/manage/collect")) return "/manage/collect";
  if (pathname.startsWith("/manage/cron")) return "/manage/cron";
  if (pathname.startsWith("/manage/system/users")) return "/manage/system/users";
  if (pathname.startsWith("/manage/system/website")) return "/manage/system/website";
  if (pathname.startsWith("/manage/access")) return "/manage/access";
  if (pathname.startsWith("/manage/system")) return "/manage/system";
  if (pathname.startsWith("/manage/storage")) return "/manage/storage";
  if (pathname.startsWith("/manage/file")) return "/manage/file";
  return "/manage";
}

export function collectAllOpenKeys(items: MenuItem[]): string[] {
  const openKeys: string[] = [];
  for (const item of items) {
    if (
      !item ||
      typeof item !== "object" ||
      !("children" in item) ||
      !item.children ||
      !("key" in item) ||
      typeof item.key !== "string"
    ) {
      continue;
    }
    openKeys.push(item.key);
  }
  return openKeys;
}

export function computeVisibleMenuItems(
  systemMode: string = "collect",
  isAdmin: boolean = false,
  accessVisible: boolean = false,
): MenuItem[] {
  const items: MenuItem[] = [];

  for (const item of menuItems) {
    if (!item) continue;
    // 纯私有模式下彻底隐藏“采集管理”
    if (item.key === "sub-collect" && systemMode === "private") {
      continue;
    }
    if (item.key === "/manage/access" && !(isAdmin && accessVisible)) {
      continue;
    }
    if (item.key === "/manage/system" && !isAdmin) {
      continue;
    }

    items.push(item);

    // 在内容管理后插入媒体存储（私有模式和混合模式展示）
    if (item.key === "sub-film" && systemMode !== "collect") {
      items.push({
        key: "/manage/storage",
        icon: <HddOutlined />,
        label: "媒体存储",
      });
    }
  }

  return items;
}
