"use client";

import React, { useState, useEffect, useCallback } from "react";
import {
  Table,
  Button,
  Card,
  Space,
  Tag,
  Modal,
  Form,
  Input,
  Select,
  Popconfirm,
  Typography,
  message,
  Tooltip,
} from "antd";
import {
  PlusOutlined,
  ReloadOutlined,
  PlayCircleOutlined,
  EditOutlined,
  DeleteOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  ApiOutlined,
  StopOutlined,
} from "@ant-design/icons";
import { ApiGet, ApiPost, ApiDelete } from "@/lib/client-api";
import styles from "./index.module.less";

export interface StorageSourceItem {
  ID: number;
  name: string;
  type: string;
  endpoint: string;
  rootPath: string;
  username: string;
  scanStatus: "idle" | "scanning" | "success" | "failed";
  lastScanAt: number;
  fileCount: number;
  movieCount: number;
  lastError: string;
}

export default function StorageManagePageView() {
  const [list, setList] = useState<StorageSourceItem[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [modalOpen, setModalOpen] = useState<boolean>(false);
  const [editingItem, setEditingItem] = useState<StorageSourceItem | null>(null);
  const [testing, setTesting] = useState<boolean>(false);
  const [form] = Form.useForm();

  const fetchList = useCallback(async () => {
    setLoading(true);
    try {
      const res = await ApiGet<StorageSourceItem[]>("/manage/storage/list");
      if (res && (res.code === 0 || res.code === 200)) {
        setList(res.data || []);
      }
    } catch {
      // 错误提示已由 axios 拦截器处理
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchList();
  }, [fetchList]);

  // 如果有正在扫描的任务，开启 3 秒轮询
  useEffect(() => {
    const hasScanning = list.some((item) => item.scanStatus === "scanning");
    if (!hasScanning) return;
    const timer = setInterval(() => {
      fetchList();
    }, 3000);
    return () => clearInterval(timer);
  }, [list, fetchList]);

  const handleOpenAdd = () => {
    setEditingItem(null);
    form.resetFields();
    form.setFieldsValue({
      type: "webdav",
      rootPath: "/",
    });
    setModalOpen(true);
  };

  const handleOpenEdit = (item: StorageSourceItem) => {
    setEditingItem(item);
    form.setFieldsValue({
      name: item.name,
      type: item.type,
      endpoint: item.endpoint,
      rootPath: item.rootPath,
      username: item.username,
      password: "", // 密码不回显
    });
    setModalOpen(true);
  };

  const handleTestConnect = async () => {
    try {
      const values = await form.validateFields();
      setTesting(true);
      const payload = {
        ...values,
        id: editingItem ? editingItem.ID : 0,
      };
      const res = await ApiPost("/manage/storage/test", payload);
      if (res && (res.code === 0 || res.code === 200)) {
        message.success("WebDAV 连接测试通过！");
      }
    } catch {
      // 校验失败或请求失败
    } finally {
      setTesting(false);
    }
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      const payload = {
        ...values,
        id: editingItem ? editingItem.ID : 0,
      };
      const res = await ApiPost("/manage/storage/save", payload);
      if (res && (res.code === 0 || res.code === 200)) {
        message.success("保存存储源成功");
        setModalOpen(false);
        fetchList();
      }
    } catch {
      // 校验失败
    }
  };

  const handleDelete = async (id: number) => {
    try {
      const res = await ApiDelete(`/manage/storage/delete?id=${id}`);
      if (res && (res.code === 0 || res.code === 200)) {
        message.success("删除成功，关联影片将在后台同步清理");
        fetchList();
      }
    } catch {
      // 错误已拦截
    }
  };

  const handleScan = async (id: number) => {
    try {
      const res = await ApiPost(`/manage/storage/scan?id=${id}`);
      if (res && (res.code === 0 || res.code === 200)) {
        message.success("已启动后台扫描与 TMDB 刮削任务");
        fetchList();
      }
    } catch {
      // 错误已拦截
    }
  };

  const handleStopScan = async (id: number) => {
    try {
      const res = await ApiPost(`/manage/storage/stop?id=${id}`);
      if (res && (res.code === 0 || res.code === 200)) {
        message.success("已成功停止扫描任务");
        fetchList();
      }
    } catch {
      // 错误已拦截
    }
  };

  const renderStatus = (status: string, lastError: string) => {
    switch (status) {
      case "scanning":
        return (
          <Space direction="vertical" size={2}>
            <Tag icon={<SyncOutlined spin />} color="processing">扫描刮削中</Tag>
            {lastError && (
              <Typography.Text style={{ fontSize: 12, maxWidth: 240, color: "#1677ff" }} ellipsis={{ tooltip: lastError }}>
                {lastError}
              </Typography.Text>
            )}
          </Space>
        );
      case "success":
        return <Tag icon={<CheckCircleOutlined />} color="success">正常</Tag>;
      case "failed":
        return (
          <Space direction="vertical" size={2}>
            <Tooltip title={lastError || "未知异常"}>
              <Tag icon={<CloseCircleOutlined />} color="error">失败</Tag>
            </Tooltip>
            {lastError && (
              <Typography.Text type="danger" style={{ fontSize: 12, maxWidth: 240 }} ellipsis={{ tooltip: lastError }}>
                {lastError.includes("401") ? "认证失败: 账号或密码错误" : lastError}
              </Typography.Text>
            )}
          </Space>
        );
      default:
        return <Tag color="default">空闲待扫描</Tag>;
    }
  };

  const columns = [
    {
      title: "挂载点名称",
      dataIndex: "name",
      key: "name",
      render: (text: string, record: StorageSourceItem) => (
        <Space orientation="vertical" size={2}>
          <Typography.Text strong>{text}</Typography.Text>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {record.endpoint} (路径: {record.rootPath})
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: "类型",
      dataIndex: "type",
      key: "type",
      width: 100,
      render: (type: string) => <Tag color="blue">{type.toUpperCase()}</Tag>,
    },
    {
      title: "状态",
      dataIndex: "scanStatus",
      key: "scanStatus",
      width: 120,
      render: (_: any, record: StorageSourceItem) =>
        renderStatus(record.scanStatus, record.lastError),
    },
    {
      title: "文件/影视数",
      key: "counts",
      width: 140,
      render: (_: any, record: StorageSourceItem) => (
        <Typography.Text>
          {record.movieCount || 0} 部 / {record.fileCount || 0} 视频
        </Typography.Text>
      ),
    },
    {
      title: "最后扫描时间",
      dataIndex: "lastScanAt",
      key: "lastScanAt",
      width: 160,
      render: (ts: number) =>
        ts > 0 ? new Date(ts * 1000).toLocaleString("zh-CN") : "从未扫描",
    },
    {
      title: "操作",
      key: "actions",
      width: 220,
      render: (_: any, record: StorageSourceItem) => (
        <Space size={8}>
          {record.scanStatus === "scanning" ? (
            <Button
              type="link"
              size="small"
              danger
              icon={<StopOutlined />}
              onClick={() => handleStopScan(record.ID)}
            >
              停止扫描
            </Button>
          ) : (
            <Button
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              onClick={() => handleScan(record.ID)}
            >
              立即扫描
            </Button>
          )}
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleOpenEdit(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定删除该存储源吗？"
            description="删除后，通过此存储源刮削入库的影片记录将被同步清除"
            onConfirm={() => handleDelete(record.ID)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className={styles.pageContainer}>
      <Card className={styles.panelCard} bordered={false}>
        <div className={styles.headerRow}>
          <div>
            <Typography.Title level={4} style={{ margin: 0 }}>
              媒体存储与挂载
            </Typography.Title>
            <Typography.Text type="secondary" className={styles.statusDesc}>
              通过标准 WebDAV 协议连接 Alist、局域网 NAS 或个人网盘，结合 TMDB 自动刮削高清海报墙与零带宽直链播放
            </Typography.Text>
          </div>
          <div className={styles.actionsGroup}>
            <Button icon={<ReloadOutlined />} onClick={fetchList} loading={loading}>
              刷新
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenAdd}>
              挂载存储源
            </Button>
          </div>
        </div>

        <Table
          rowKey="ID"
          loading={loading}
          dataSource={list}
          columns={columns}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        title={editingItem ? "编辑存储源" : "挂载新存储源"}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        footer={[
          <Button key="test" icon={<ApiOutlined />} loading={testing} onClick={handleTestConnect}>
            测试连接
          </Button>,
          <Button key="cancel" onClick={() => setModalOpen(false)}>
            取消
          </Button>,
          <Button key="submit" type="primary" onClick={handleSave}>
            保存
          </Button>,
        ]}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label="挂载点名称"
            rules={[{ required: true, message: "请输入挂载点名称" }]}
          >
            <Input placeholder="例如：Alist-115、家庭群晖NAS" />
          </Form.Item>

          <Form.Item name="type" label="存储类型" rules={[{ required: true }]}>
            <Select options={[{ label: "WebDAV 协议（推荐 Alist / NAS）", value: "webdav" }]} />
          </Form.Item>

          <Form.Item
            name="endpoint"
            label="服务地址 (Endpoint)"
            rules={[{ required: true, message: "请输入 WebDAV 服务地址" }]}
          >
            <Input placeholder="http://127.0.0.1:5244/dav" />
          </Form.Item>

          <Form.Item 
            name="rootPath" 
            label="扫描根路径" 
            extra="相对于 WebDAV 根目录的路径，例如 / 或 /电影"
          >
            <Input placeholder="默认 /，可指定子目录如 /电影" />
          </Form.Item>

          <Form.Item 
            name="username" 
            label="认证用户名" 
            extra="小雅 xiaoya-alist 默认访客账号为 guest"
          >
            <Input placeholder="若无账号可留空" />
          </Form.Item>

          <Form.Item 
            name="password" 
            label="认证密码" 
            extra="小雅默认访客密码为 guest_Api789（编辑时留空则保留原密码）"
          >
            <Input.Password placeholder={editingItem ? "留空则保持原密码不变" : "若无密码可留空"} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
