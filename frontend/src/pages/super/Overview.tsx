import { useEffect, useState } from 'react';
import {
  Card,
  Message,
  Grid,
  Skeleton,
  Typography,
  Tag,
  Table,
  Button,
  Modal,
  Form,
  Input,
  InputNumber,
  Select,
} from '@arco-design/web-react';
import { api } from '../../api';
import StatCard from '../../components/StatCard';
import {
  IconApps,
  IconUserGroup,
  IconThunderbolt,
  IconCheckCircle,
  IconStorage,
  IconFire,
  IconSafe,
  IconGift,
  IconClockCircle,
} from '@arco-design/web-react/icon';

const { Row, Col } = Grid;

// 近 7 天点数消耗迷你条形图（纯 CSS，直观展示用量走势）
function MiniBar({ days }: { days: any[] }) {
  const max = Math.max(1, ...days.map((d) => d.used || 0));
  return (
    <div style={{ display: 'flex', alignItems: 'flex-end', gap: 10, height: '100%', minHeight: 200, padding: '4px 4px 0' }}>
      {days.map((d) => {
        const h = ((d.used || 0) / max) * 100;
        const label = d.day.slice(5);
        return (
          <div key={d.day} style={{ flex: 1, textAlign: 'center', height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'flex-end' }}>
            <div style={{ fontSize: 12, color: 'var(--color-text-2)', marginBottom: 4, height: 16 }}>{d.used || 0}</div>
            <div
              style={{
                height: `calc(${Math.max(h, 2)}% - 20px)`,
                minHeight: 4,
                borderRadius: 6,
                background: 'linear-gradient(180deg, #165DFF 0%, #6AA1FF 100%)',
                transition: 'height .3s',
              }}
              title={`${d.day}：消耗 ${d.used || 0} 点`}
            />
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 6 }}>{label}</div>
          </div>
        );
      })}
    </div>
  );
}

export default function SuperOverview() {
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  // 在线客户（每 10 秒轮询，后端内存统计零数据库查询）
  const [online, setOnline] = useState<any>({ online: 0, total: 0, online_list: [] });
  // 续费弹窗
  const [extendUser, setExtendUser] = useState<any>(null);
  const [extendSaving, setExtendSaving] = useState(false);
  const [extendForm] = Form.useForm();
  const [extendPriceFen, setExtendPriceFen] = useState(9800); // 续费月单价（分）
  const monthsWatch = Form.useWatch('months', extendForm) as number | undefined;
  const payMethodWatch = Form.useWatch('pay_method', extendForm) as string | undefined;

  useEffect(() => {
    // 续费单价来自支付设置（extend_price_fen_month），用于弹窗金额预估
    api
      .payGetConfig()
      .then((cfg: any) => {
        const fen = Number(cfg?.extend_price_fen_month);
        if (fen > 0) setExtendPriceFen(fen);
      })
      .catch(() => {});
  }, []);

  const load = () =>
    api
      .superOverview()
      .then(setData)
      .catch((e: any) => Message.error('加载失败：' + e.message))
      .finally(() => setLoading(false));

  useEffect(() => {
    load();
  }, []);

  // 在线客户统计：10 秒轮询（后端纯内存读，开销极低）
  useEffect(() => {
    let timer: any;
    const poll = () => {
      api
        .superOnlineCount()
        .then(setOnline)
        .catch(() => {});
    };
    poll();
    timer = setInterval(poll, 10000);
    return () => clearInterval(timer);
  }, []);

  // 预警行一键续费
  const openExtend = (w: any) => {
    setExtendUser(w);
    extendForm.resetFields();
    extendForm.setFieldsValue({ months: 12, pay_method: '微信' });
  };

  const submitExtend = async () => {
    const values = await extendForm.validate();
    setExtendSaving(true);
    try {
      const res: any = await api.extendUser(
        extendUser.user_id,
        Number(values.months),
        (values.remark || '').trim(),
        values.pay_method || '微信',
      );
      Message.success(res?.msg || '续费成功');
      setExtendUser(null);
      load(); // 刷新看板，预警列表随之更新
    } catch (e: any) {
      Message.error(e.message || '续费失败');
    } finally {
      setExtendSaving(false);
    }
  };

  if (loading) return <Skeleton text={true} animation />;

  const d = data || {};
  const tenants = d.tenants || [];
  const days = d.days || [];
  const expiryWarnings = d.expiry_warnings || [];

  // 到期预警 Tag 颜色：已到期红 / 7 天内橙红 / 30 天内橙
  const expiryTag = (r: number) => {
    if (r <= 0) return <Tag color="red">已到期 {-r} 天</Tag>;
    if (r <= 7) return <Tag color="red">仅剩 {r} 天</Tag>;
    return <Tag color="orange">剩 {r} 天</Tag>;
  };

  // 分站简况表列（含点数经营）
  const tenantColumns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    {
      title: '客户（分站）',
      dataIndex: 'name',
      render: (v: string, row: any) => (
        <span>
          <Tag color="arcoblue" size="small">{row.code}</Tag> {v}
        </span>
      ),
    },
    {
      title: '点数余额',
      dataIndex: 'points',
      width: 110,
      render: (v: number) => (
        <span style={{ fontWeight: 600, color: (v ?? 0) <= 20 ? 'rgb(var(--danger-6))' : undefined }}>
          {v ?? 0} 点{((v ?? 0) <= 20) ? ' ⚠' : ''}
        </span>
      ),
    },
    { title: '近 7 天消耗', dataIndex: 'used_7d', width: 110, render: (v: number) => `${v ?? 0} 点` },
    { title: '近 7 天任务', dataIndex: 'tasks', width: 100, render: (v: number) => v ?? 0 },
    { title: '近 7 天命中', dataIndex: 'hits', width: 100, render: (v: number) => v ?? 0 },
  ];

  return (
    <div>
      {/* 核心经营指标 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <StatCard
            title="在线客户（实时）"
            value={online.online || 0}
            suffix={`/${online.total || 0} 家`}
            icon={<IconUserGroup />}
            color="#00B42A"
            extra={
              <span style={{ fontSize: 11, color: '#00B42A', marginLeft: 6 }}>
                ● 10 秒刷新
              </span>
            }
          />
        </Col>
        <Col span={6}>
          <StatCard
            title="总客户数量"
            value={d.tenant_count || 0}
            suffix="家"
            icon={<IconApps />}
            color="#165DFF"
          />
        </Col>
        <Col span={6}>
          <StatCard
            title="新注册客户（近 7 天）"
            value={d.new_customers_7d || 0}
            suffix="家"
            icon={<IconGift />}
            color="#00B42A"
          />
        </Col>
        <Col span={6}>
          <StatCard
            title="点数累计使用量"
            value={d.points_used || 0}
            suffix="点"
            icon={<IconFire />}
            color="#F77234"
          />
        </Col>
        <Col span={6}>
          <StatCard
            title="点数剩余总量"
            value={d.points_left || 0}
            suffix="点"
            icon={<IconSafe />}
            color="#722ED1"
          />
        </Col>
      </Row>

      {/* 客户到期预警 */}
      <Card
        title={
          <span>
            <IconClockCircle style={{ color: '#FF7D00', marginRight: 6 }} />
            客户到期预警
          </span>
        }
        extra={
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            服务有效期 30 天内到期的客户，建议提前联系续费
          </Typography.Text>
        }
        style={{ marginBottom: 16, borderRadius: 12, display: expiryWarnings.length ? 'block' : 'none' }}
        bordered
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {expiryWarnings.map((w: any) => (
            <div
              key={w.tenant_id}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 12,
                padding: '10px 14px',
                borderRadius: 10,
                background: w.remain_days <= 0 ? 'rgb(var(--danger-1))' : 'var(--color-fill-2)',
              }}
            >
              <Tag color="arcoblue" size="small">{w.tenant_code}</Tag>
              <span style={{ fontWeight: 600, minWidth: 120 }}>{w.tenant_name}</span>
              <Typography.Text type="secondary" style={{ fontSize: 13 }}>
                账号 {w.username}{w.nickname && w.nickname !== w.tenant_name ? `（${w.nickname}）` : ''}
              </Typography.Text>
              <span style={{ flex: 1 }} />
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                到期日 {w.expire_at ? new Date(w.expire_at).toLocaleDateString() : '-'}
              </Typography.Text>
              {expiryTag(w.remain_days)}
              <Button size="mini" type="primary" status={w.remain_days <= 0 ? 'danger' : undefined} onClick={() => openExtend(w)}>
                续费
              </Button>
            </div>
          ))}
        </div>
      </Card>

      {/* 次级指标 + 消耗趋势 */}
      <Row gutter={16} style={{ marginBottom: 16 }} align="stretch">
        <Col span={14}>
          <Card
            title="近 7 天点数消耗走势"
            extra={<Tag color="orangered">近 7 天共 {d.points_used_7d || 0} 点</Tag>}
            style={{ borderRadius: 12, height: '100%' }}
            bodyStyle={{ height: 'calc(100% - 48px)' }}
            bordered
          >
            <MiniBar days={days} />
          </Card>
        </Col>
        <Col span={10}>
          <Card title="今日动态" style={{ borderRadius: 12, height: '100%' }} bordered>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <RowMetric icon={<IconFire />} color="#F77234" label="今日点数消耗" value={`${d.points_used_today || 0} 点`} />
              <RowMetric icon={<IconStorage />} color="#165DFF" label="今日巡检任务" value={`${d.today_tasks || 0} 个`} />
              <RowMetric icon={<IconCheckCircle />} color="#00B42A" label="今日品牌命中" value={`${d.today_hits || 0} 次`} />
              <RowMetric icon={<IconThunderbolt />} color="#FF7D00" label="运行中任务" value={`${d.running_count || 0} 个`} />
              <RowMetric icon={<IconUserGroup />} color="#722ED1" label="分站账号总数" value={`${d.account_count || 0} 个`} />
            </div>
          </Card>
        </Col>
      </Row>

      {/* 客户经营明细 */}
      <Card
        title="客户经营明细（近 7 天）"
        extra={
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            近 30 天新注册 {d.new_customers_30d || 0} 家 · 累计巡检任务 {d.task_count || 0} 个
          </Typography.Text>
        }
        style={{ borderRadius: 12 }}
        bordered
      >
        <Table rowKey="id" columns={tenantColumns} data={tenants} pagination={false} size="middle" />
      </Card>

      {/* 续费弹窗 */}
      <Modal
        title={`续费 - ${extendUser?.tenant_name || ''}`}
        visible={!!extendUser}
        onCancel={() => setExtendUser(null)}
        onOk={submitExtend}
        confirmLoading={extendSaving}
        okText="确认续费"
        cancelText="取消"
        maskClosable={false}
      >
        <Typography.Paragraph type="secondary" style={{ fontSize: 13 }}>
          客户「{extendUser?.tenant_name}」（账号 {extendUser?.username}）当前到期时间：
          {extendUser?.expire_at ? new Date(extendUser.expire_at).toLocaleString() : '不限'}。
          {extendUser?.remain_days <= 0 ? '已到期，续费将从今天起算。' : '续费将从当前到期时间顺延。'}
        </Typography.Paragraph>
        <Form form={extendForm} layout="vertical">
          <Form.Item
            label="续费时长"
            field="months"
            rules={[{ required: true, message: '请输入续费月数' }]}
          >
            <InputNumber min={1} max={36} precision={0} suffix="个月" style={{ width: '100%' }} placeholder="1 - 36 个月" />
          </Form.Item>
          <Form.Item label="收款方式（入流水便于对账；赠送金额记 0）" field="pay_method" initialValue="微信">
            <Select
              placeholder="选择收款方式"
              style={{ width: '100%' }}
            >
              {['微信', '支付宝', '银行转账', '现金', '赠送'].map((m) => (
                <Select.Option key={m} value={m}>{m}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item>
            <Typography.Text style={{ fontSize: 13 }}>
              应收金额：
              <b style={{ fontSize: 18, color: 'rgb(var(--orange-6))' }}>
                ¥{(payMethodWatch === '赠送' ? 0 : (extendPriceFen * (monthsWatch || 0)) / 100).toFixed(2)}
              </b>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                （月单价 ¥{(extendPriceFen / 100).toFixed(2)} × {monthsWatch || 0} 个月，可在「支付设置」调整单价）
              </Typography.Text>
            </Typography.Text>
          </Form.Item>
          <Form.Item label="备注（选填，入续费流水便于对账）" field="remark">
            <Input placeholder="如：2026 年度续费 · 微信收款" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}

function RowMetric({ icon, color, label, value }: { icon: React.ReactNode; color: string; label: string; value: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
      <div
        style={{
          width: 36,
          height: 36,
          borderRadius: 8,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: color + '1A',
          color,
          fontSize: 18,
        }}
      >
        {icon}
      </div>
      <div style={{ flex: 1, color: 'var(--color-text-2)', fontSize: 14 }}>{label}</div>
      <div style={{ fontWeight: 700, fontSize: 16, color }}>{value}</div>
    </div>
  );
}
