"use client";

import React, { useEffect, useState } from "react";
import { Card, Typography } from "antd";
import Link from "next/link";
import {
  AppstoreOutlined,
  DatabaseOutlined,
  FileTextOutlined,
  FolderOpenOutlined,
  PictureOutlined,
  VideoCameraOutlined,
  HddOutlined,
  SettingOutlined,
} from "@ant-design/icons";
import { ApiGet } from "@/lib/client-api";
import { useManagePermission } from "@/lib/manage-permission";
import { useSiteConfig } from "@/components/common/SiteGuard";
import ManagePageHeader from "@/app/manage/components/page-header";
import CollectOverview from "@/app/manage/collect/view/collect-overview";
import { Alert, Tag } from "antd";
import styles from "./index.module.less";

interface FilmInventoryStats {
  films: number;
  categories: number;
  failures: number;
}

const quickEntries = [
  {
    key: "film",
    icon: VideoCameraOutlined,
    title: "影片列表",
    description: "快速查看、更新和编辑主库存影片。",
    href: "/manage/film",
  },
  {
    key: "collect",
    icon: DatabaseOutlined,
    title: "采集中心",
    description: "配置主站、附属站与批量采集任务。",
    href: "/manage/collect",
  },
  {
    key: "category",
    icon: AppstoreOutlined,
    title: "分类管理",
    description: "维护当前主站分类框架、显示状态与排序。",
    href: "/manage/collect/category",
  },
  {
    key: "category-rules",
    icon: DatabaseOutlined,
    title: "分类规则",
    description: "配置来源分类到展示分类的合并映射。",
    href: "/manage/collect/category/rules",
  },
  {
    key: "assets",
    icon: PictureOutlined,
    title: "素材中心",
    description: "上传、预览和整理站内会用到的封面图与素材图。",
    href: "/manage/file",
  },
];

export default function ManagePageView() {
  const { isAdmin } = useManagePermission();
  const { config: siteInfo, refresh: refreshSiteConfig } = useSiteConfig();
  const [stats, setStats] = useState<FilmInventoryStats | null>(null);

  const isPrivate = siteInfo?.systemMode === "private";

  useEffect(() => {
    void refreshSiteConfig();
    let active = true;
    ApiGet<FilmInventoryStats>("/manage/spider/clear/stats")
      .then((resp) => {
        if (active && resp.code === 0 && resp.data) {
          setStats(resp.data);
        }
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, []);

  const dynamicQuickEntries = React.useMemo(() => {
    if (isPrivate) {
      return [
        {
          key: "film",
          icon: VideoCameraOutlined,
          title: "影片列表",
          description: "快速查看、更新和管理入库的私有影视资源。",
          href: "/manage/film",
        },
        {
          key: "storage",
          icon: HddOutlined,
          title: "媒体存储",
          description: "管理 WebDAV / Alist 挂载源、扫描状态与 TMDB 刮削。",
          href: "/manage/storage",
        },
        {
          key: "category",
          icon: AppstoreOutlined,
          title: "分类管理",
          description: "维护当前影视分类、显示状态与排序。",
          href: "/manage/collect/category",
        },
        {
          key: "website",
          icon: SettingOutlined,
          title: "网站配置",
          description: "配置系统运行模式、TMDB 密钥及站点基本信息。",
          href: "/manage/system/website",
        },
        {
          key: "assets",
          icon: PictureOutlined,
          title: "素材中心",
          description: "上传、预览和整理站内封面图与素材图。",
          href: "/manage/file",
        },
      ];
    }
    return quickEntries;
  }, [isPrivate]);

  return (
    <div className={styles.dashboard}>
      <ManagePageHeader
        title="工作台"
        description={
          isPrivate
            ? "私有媒体库模式已开启。展示挂载存储源状态、私有影视数据与常用管理入口。"
            : "采集运行概况、影视数据规模与常用入口。"
        }
      />

      {!isPrivate && <CollectOverview />}

      <Card
        className={styles.panelCard}
        title="当前影视数据规模"
        extra={
          isAdmin ? (
            <Link href="/manage/system?tab=security" className={styles.statsLink}>
              数据安全
            </Link>
          ) : null
        }
      >
        {isPrivate && (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="系统当前运行在「私有媒体库模式」"
            description={
              <span>
                前台首页与列表已隔离网络采集。若当前库中留存有测试或历史采集数据，可前往
                <Link href="/manage/system?tab=security" style={{ margin: "0 4px", fontWeight: 600 }}>
                  系统设置 · 数据安全
                </Link>
                一键「清空影视与采集数据」，即可完全重置为干净的纯私有影视库。
              </span>
            }
          />
        )}
        <div className={styles.statsGrid}>
          {[
            {
              title: "影视库存",
              value: stats?.films,
              icon: VideoCameraOutlined,
            },
            {
              title: "分类",
              value: stats?.categories,
              icon: FolderOpenOutlined,
            },
            {
              title: "失败记录",
              value: stats?.failures,
              icon: FileTextOutlined,
            },
          ].map((item) => {
            const Icon = item.icon;
            return (
              <div key={item.title} className={styles.statsCard}>
                <div className={styles.statsIcon}>
                  <Icon />
                </div>
                <div className={styles.statsBody}>
                  <div className={styles.statsValue}>{item.value ?? "—"}</div>
                  <div className={styles.statsTitle}>{item.title}</div>
                </div>
              </div>
            );
          })}
        </div>
        {isAdmin && (
          <Typography.Text className={styles.statsNote}>
            反映当前库内影视相关体量。清空影视与采集派生数据请前往「系统设置 · 数据安全」。
          </Typography.Text>
        )}
      </Card>

      <Card className={styles.panelCard} title="快捷入口">
        <div className={styles.entryGrid}>
          {dynamicQuickEntries.map((entry) => {
            const Icon = entry.icon;
            return (
              <Link
                key={entry.key}
                href={entry.href}
                className={styles.entryCard}
              >
                <div className={styles.entryCardHead}>
                  <div className={styles.entryIconWrap}>
                    <Icon />
                  </div>
                  <div className={styles.entryTitle}>{entry.title}</div>
                </div>
                <div className={styles.stepDesc}>{entry.description}</div>
              </Link>
            );
          })}
        </div>
      </Card>
    </div>
  );
}
