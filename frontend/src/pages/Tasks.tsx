import { useEffect, useState } from 'react';
import {
  Card, Table, Button, Modal, Message, Space, Tag, Descriptions,
  Tabs, Typography, Grid, Empty, Badge, Alert,
} from '@arco-design/web-react';
import { IconPlayArrow, IconSync, IconThunderbolt, IconEye } from '@arco-design/web-react/icon';
import { api } from '../api';

const { Row, Col } = Grid;

const statusMap: Record<string, { label: string; color: string }> = {
  pending: { label: '等待中', color: 'gray' },
  running: { label: '运行中', color: 'arcoblue' },
  success: { label: '成功', color: 'green' },
  partial: { label: '部分成功', color: 'orange' },
  failed: { label: '失败', color: 'red' },
};

export default function Tasks() {
  const [tasks, setTasks] = useState<any[]>([]);
  const [running, setRunning] = useState(false);
  const [detail, setDetail] = useState<any>(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [detailTask, setDetailTask] = useState<any>(null);

  const load = async () => {
    const t = await api.listTasks();
    setTasks(t || []);
  };
  useEffect(() => { load(); }, []);

  const run = async (mode = 'manual') => {
    setRunning(true);
    try {
      await api.runTask(mode);
      Message.success('巡检任务已启动，可能需 1-3 分钟，请稍后刷新');
      setTimeout(load, 3000);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setRunning(false);
    }
  };

  const openDetail = async (id: number) => {
    try {
      const d = await api.taskDetail(id);
      setDetail(d);
      setDetailTask(d.task);
      setDetailVisible(true);
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const columns = [
    { title: '任务 ID', dataIndex: 'id', width: 80 },
    {
      title: '类型', dataIndex: 'mode', width: 90,
      render: (v: string) => <Tag>{v === 'cron' ? '自动巡检' : '手动'}</Tag>,
    },
    {
      title: '状态', dataIndex: 'status', width: 110,
      render: (v: string) => {
        const m = statusMap[v] || { label: v, color: 'gray' };
        return <Tag color={m.color}><Badge status="processing" /> {m.label}</Tag>;
      },
    },
    { title: '查询数', dataIndex: 'total_queries', width: 90 },
    {
      title: '命中 / 覆盖平台', width: 150,
      render: (_: any, row: any) => (
        <Space>
          <Tag color="green">命中 {row.hit_count}</Tag>
          <Tag color="arcoblue">覆盖 {row.coverage} 平台</Tag>
        </Space>
      ),
    },
    {
      title: '耗时', width: 100,
      render: (_: any, row: any) => {
        if (!row.started_at || !row.finished_at) return '-';
        const ms = new Date(row.finished_at).getTime() - new Date(row.started_at).getTime();
        return `${(ms / 1000).toFixed(1)}s`;
      },
    },
    { title: '开始时间', dataIndex: 'started_at', render: (v: string) => (v ? new Date(v).toLocaleString() : '-') },
    {
      title: '操作', width: 90,
      render: (_: any, row: any) => (
        <Button size="mini" type="text" icon={<IconEye />} onClick={() => openDetail(row.id)}>详情</Button>
      ),
    },
  ];

  const results = detail?.results || [];

  return (
    <div>
      <Card title="巡检任务" style={{ marginBottom: 16, borderRadius: 12 }} bordered={false}>
        <Space>
          <Button type="primary" icon={<IconPlayArrow />} loading={running} onClick={() => run('manual')}>
            立即全量巡检
          </Button>
          <Button icon={<IconSync />} onClick={load}>刷新列表</Button>
          <Typography.Text type="secondary">
            全量巡检 = 所有「启用的平台」× 所有「启用的问题」，每轮耗时约几十秒到数分钟。
          </Typography.Text>
        </Space>
      </Card>

      <Card title="任务列表" style={{ borderRadius: 12 }}>
        <Table rowKey="id" columns={columns} data={tasks} pagination={{ pageSize: 10 }} />
      </Card>

      {/* 任务详情 */}
      <Modal
        title={`任务 #${detailTask?.id || ''} 详情`}
        visible={detailVisible}
        footer={null}
        onCancel={() => setDetailVisible(false)}
        style={{ width: 900 }}
        unmountOnExit
      >
        {detailTask && (
          <Descriptions
            column={4}
            data={[
              { label: '状态', value: <Tag color={(statusMap[detailTask.status] || {}).color}>{detailTask.status}</Tag> },
              { label: '查询数', value: detailTask.total_queries },
              { label: '命中', value: <Tag color="green">{detailTask.hit_count}</Tag> },
              { label: '覆盖平台', value: detailTask.coverage },
              { label: '未命中', value: detailTask.miss_count },
              { label: '错误', value: <Tag color="red">{detailTask.error_count}</Tag> },
            ]}
            style={{ marginBottom: 16 }}
          />
        )}
        {results.length ? (
          <Tabs defaultActiveTab="all">
            <Tabs.TabPane key="all" title={`全部 (${results.length})`}>
              <ResultTable results={results} />
            </Tabs.TabPane>
            <Tabs.TabPane key="hit" title={`命中 (${results.filter((r: any) => r.hit).length})`}>
              <ResultTable results={results.filter((r: any) => r.hit)} />
            </Tabs.TabPane>
            <Tabs.TabPane key="miss" title={`未命中 (${results.filter((r: any) => !r.hit && !r.error_msg).length})`}>
              <ResultTable results={results.filter((r: any) => !r.hit && !r.error_msg)} />
            </Tabs.TabPane>
            <Tabs.TabPane key="err" title={`错误 (${results.filter((r: any) => r.error_msg).length})`}>
              <ResultTable results={results.filter((r: any) => r.error_msg)} />
            </Tabs.TabPane>
          </Tabs>
        ) : (
          <Empty description="该任务暂无结果" />
        )}
      </Modal>
    </div>
  );
}

function ResultTable({ results }: { results: any[] }) {
  if (!results.length) return <Empty description="无数据" />;
  const columns = [
    { title: '平台', dataIndex: 'platform_name', width: 110, render: (v: string) => <Tag color="arcoblue">{v}</Tag> },
    { title: '问题', dataIndex: 'question' },
    {
      title: '命中', dataIndex: 'hit', width: 70,
      render: (v: boolean, row: any) =>
        row.error_msg ? (
          <Tag color="red">错误</Tag>
        ) : v ? (
          <Tag color="green">命中</Tag>
        ) : (
          <Tag color="gray">未命中</Tag>
        ),
    },
    {
      title: '提及次数', dataIndex: 'mention_count', width: 90,
      render: (v: number, row: any) => (row.error_msg ? '-' : v),
    },
    {
      title: '回答摘要', dataIndex: 'response',
      render: (v: string, row: any) => {
        if (row.error_msg) return <span style={{ color: '#F53F3F' }}>{row.error_msg}</span>;
        return <span style={{ color: 'var(--color-text-2)', fontSize: 12 }}>{v || '-'}</span>;
      },
    },
    { title: '耗时', dataIndex: 'cost_ms', width: 80, render: (v: number) => `${v}ms` },
  ];
  return <Table rowKey="id" columns={columns} data={results} pagination={{ pageSize: 8 }} size="small" />;
}
