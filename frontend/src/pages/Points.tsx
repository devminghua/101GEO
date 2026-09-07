import { useEffect, useRef, useState } from 'react';
import {
  Card, Table, Statistic, Space, Tag, TableColumnProps,
  Button, Modal, Form, Select, Input, Message, Typography, Empty,
} from '@arco-design/web-react';
import { QRCodeSVG } from 'qrcode.react';
import { api } from '../api';

interface PointRecord {
  id: number;
  tenant_id: number;
  amount: number;
  type: string; // recharge / consume
  remark: string;
  balance_after: number;
  created_at: string;
}

interface RechargeOrder {
  order_no: string;
  channel: string;
  code_url: string;
  amount_fen: number;
  points: number;
  expire_time: string;
}

const POLL_INTERVAL = 3000; // 支付状态轮询间隔（毫秒）
const ORDER_EXPIRE_MS = 30 * 60 * 1000; // 订单过期 30 分钟

export default function Points() {
  const [balance, setBalance] = useState(0);
  const [records, setRecords] = useState<PointRecord[]>([]);
  const [loading, setLoading] = useState(false);

  // 扫码充值 Modal 状态
  const [rechargeVisible, setRechargeVisible] = useState(false);
  const [channel, setChannel] = useState<'wechat' | 'alipay'>('wechat');
  const [plans, setPlans] = useState<any[]>([]);
  const [selectedPlan, setSelectedPlan] = useState<any>(null);
  const [creating, setCreating] = useState(false);
  const [order, setOrder] = useState<RechargeOrder | null>(null);
  const [polling, setPolling] = useState(false);
  const timerRef = useRef<number | null>(null);

  // 卡密兑换 Modal 状态
  const [redeemVisible, setRedeemVisible] = useState(false);
  const [redeemCode, setRedeemCode] = useState('');
  const [redeeming, setRedeeming] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const data = await api.getPoints();
      setBalance(data.balance || 0);
      setRecords(data.records || []);
    } catch {
      /* 查询失败由统一封装提示 */
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  // 组件卸载时清理轮询定时器
  useEffect(() => () => {
    if (timerRef.current) window.clearInterval(timerRef.current);
  }, []);

  const stopPolling = () => {
    if (timerRef.current) {
      window.clearInterval(timerRef.current);
      timerRef.current = null;
    }
    setPolling(false);
  };

  // 关闭 Modal：停止轮询、清空订单
  const closeRecharge = () => {
    stopPolling();
    setOrder(null);
    setRechargeVisible(false);
  };

  // 打开充值弹窗：加载套餐列表
  const openRecharge = async () => {
    setRechargeVisible(true);
    setSelectedPlan(null);
    setOrder(null);
    try {
      setPlans((await api.listRechargePlans()) || []);
    } catch {
      setPlans([]);
    }
  };

  // 选套餐下单
  const createOrder = async () => {
    if (!selectedPlan) {
      Message.warning('请选择套餐');
      return;
    }
    setCreating(true);
    try {
      const data = await api.rechargePlanOrder(selectedPlan.id, channel);
      setOrder(data);
      // 开启轮询：每 3s 查一次订单状态，成功后刷新余额与流水
      setPolling(true);
      timerRef.current = window.setInterval(async () => {
        try {
          const st = await api.rechargeStatus(data.order_no);
          if (st.status === 1) {
            stopPolling();
            Message.success(`充值成功，已到账 ${st.points} token`);
            setOrder(null);
            setRechargeVisible(false);
            load();
          } else if (st.status === 2 || st.status === 3) {
            stopPolling();
            Message.error('订单已关闭或失败，请重新下单');
            setOrder(null);
          }
        } catch {
          /* 轮询失败忽略，下轮重试 */
        }
      }, POLL_INTERVAL);
    } catch {
      /* 下单失败由统一封装提示 */
    } finally {
      setCreating(false);
    }
  };

  // 卡密兑换 token
  const doRedeem = async () => {
    const code = redeemCode.trim();
    if (!code) {
      Message.warning('请输入充值卡密');
      return;
    }
    setRedeeming(true);
    try {
      const data = await api.redeemCard(code);
      Message.success(`兑换成功，已到账 ${data.points} token`);
      setRedeemVisible(false);
      setRedeemCode('');
      load();
    } catch (e: any) {
      Message.error(e.message || '兑换失败');
    } finally {
      setRedeeming(false);
    }
  };

  const columns: TableColumnProps[] = [
    { title: '时间', dataIndex: 'created_at', width: 170 },
    {
      title: '类型',
      dataIndex: 'type',
      width: 100,
      render: (v: string) => v === 'recharge'
        ? <Tag color="green">充值</Tag>
        : <Tag color="red">消费</Tag>,
    },
    {
      title: 'token',
      dataIndex: 'amount',
      width: 100,
      render: (v: number) => (
        <span style={{ color: (v || 0) > 0 ? '#00b42a' : '#f53f3f', fontWeight: 600 }}>
          {(v || 0) > 0 ? `+${v}` : v}
        </span>
      ),
    },
    { title: '说明', dataIndex: 'remark' },
    { title: '操作后余额', dataIndex: 'balance_after', width: 120 },
  ];

  // 应付金额（分 -> 元）
  const amountYuan = order ? (order.amount_fen / 100).toFixed(2) : '0.00';
  const isExpired = order ? Date.now() - new Date(order.expire_time).getTime() > ORDER_EXPIRE_MS : false;

  return (
    <div>
      <div style={{ fontSize: 20, fontWeight: 600, marginBottom: 16 }}>充值中心</div>
      <Card style={{ marginBottom: 16 }}>
        <Space size="large" align="start">
          <Statistic title="Token 余额" value={balance} groupSeparator suffix="token" />
          <div style={{ color: 'var(--color-text-3)', fontSize: 13, paddingTop: 6, maxWidth: 420 }}>
            AI 调用按次扣 token（每次 1 token），余额不足时将无法使用 AI 功能。
            支持两种充值方式：<b>扫码充值</b> 或 <b>卡密兑换</b>，任选其一。
          </div>
          <Space>
            <Button type="primary" onClick={openRecharge}>充值</Button>
            <Button type="outline" onClick={() => setRedeemVisible(true)}>卡密兑换</Button>
          </Space>
        </Space>
      </Card>
      <Card title="Token 流水">
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          data={records}
          pagination={{ pageSize: 10, showTotal: true }}
        />
      </Card>

      {/* 扫码充值 Modal：选渠道 -> 选套餐 -> 下单 -> 展示二维码 -> 轮询到账 */}
      <Modal
        title={order ? '扫码支付' : '选择套餐'}
        visible={rechargeVisible}
        onCancel={closeRecharge}
        footer={null}
        maskClosable={false}
        style={{ width: order ? 360 : 560 }}
      >
        {!order ? (
          <div>
            <div style={{ marginBottom: 14 }}>
              <div style={{ marginBottom: 6 }}><Typography.Text bold>支付渠道</Typography.Text></div>
              <Select
                value={channel}
                onChange={(v) => setChannel(v as 'wechat' | 'alipay')}
                style={{ width: '100%' }}
                options={[
                  { label: '微信支付（扫码）', value: 'wechat' },
                  { label: '支付宝（扫码）', value: 'alipay' },
                ]}
              />
            </div>
            <div style={{ marginBottom: 10 }}><Typography.Text bold>选择套餐</Typography.Text></div>
            {plans.length === 0 ? (
              <Empty description="暂无可用套餐，请联系总后台设置" />
            ) : (
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 12 }}>
                {plans.map((p) => {
                  const active = selectedPlan?.id === p.id;
                  return (
                    <div
                      key={p.id}
                      onClick={() => setSelectedPlan(p)}
                      style={{
                        position: 'relative',
                        padding: '16px',
                        borderRadius: 14,
                        cursor: 'pointer',
                        border: `2px solid ${active ? '#4F46E5' : 'var(--color-border-2)'}`,
                        background: active ? 'rgba(79,70,229,0.06)' : 'var(--color-fill-1)',
                        transition: 'all .15s',
                        userSelect: 'none',
                      }}
                    >
                      {p.tag && (
                        <span style={{
                          position: 'absolute', top: -10, right: 12,
                          background: 'linear-gradient(135deg,#4F46E5,#7B61FF)', color: '#fff',
                          fontSize: 11, fontWeight: 600, padding: '2px 10px', borderRadius: 10,
                        }}>{p.tag}</span>
                      )}
                      <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--geo-text)' }}>{p.name}</div>
                      <div style={{ marginTop: 4, fontSize: 12, color: '#86909C' }}>{p.points.toLocaleString('en-US')} token</div>
                      <div style={{ marginTop: 8, display: 'flex', alignItems: 'baseline', gap: 6 }}>
                        <span style={{ fontSize: 22, fontWeight: 800, color: '#4F46E5' }}>¥{(p.price_fen / 100).toFixed(2)}</span>
                        {p.orig_fen > 0 && (
                          <span style={{ fontSize: 13, color: '#86909C', textDecoration: 'line-through' }}>¥{(p.orig_fen / 100).toFixed(2)}</span>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
            <Button
              type="primary"
              long
              loading={creating}
              disabled={!selectedPlan}
              onClick={createOrder}
              style={{ marginTop: 16, height: 42, fontSize: 15 }}
            >
              {selectedPlan ? `立即支付 ¥${(selectedPlan.price_fen / 100).toFixed(2)}` : '请选择套餐'}
            </Button>
          </div>
        ) : (
          <div style={{ textAlign: 'center' }}>
            <QRCodeSVG value={order.code_url} size={220} level="M" marginSize={2} />
            <div style={{ marginTop: 12, fontSize: 15, fontWeight: 600 }}>
              应付 ¥{amountYuan}
            </div>
            <div style={{ marginTop: 4, fontSize: 12, color: 'var(--color-text-3)' }}>
              {order.channel === 'wechat' ? '请使用微信「扫一扫」' : '请使用支付宝「扫一扫」'}完成支付
              ，到账 {order.points} token
            </div>
            <div style={{ marginTop: 8, fontSize: 12, color: '#f53f3f' }}>
              {isExpired ? '订单已过期，请关闭后重新下单' : (polling ? '支付成功后自动到账，无需手动刷新' : '')}
            </div>
            <Button
              style={{ marginTop: 16 }}
              onClick={closeRecharge}
              disabled={polling}
            >
              关闭
            </Button>
          </div>
        )}
      </Modal>

      {/* 卡密兑换 Modal：输入充值卡密 -> 兑换 token */}
      <Modal
        title="卡密兑换 token"
        visible={redeemVisible}
        onCancel={() => { setRedeemVisible(false); setRedeemCode(''); }}
        footer={null}
        maskClosable={false}
        style={{ width: 420 }}
      >
        <Form layout="vertical">
          <Form.Item
            label="充值卡密"
            required
            extra="请输入服务商提供的充值卡密（LG-XXXXX-...），兑换后 token 立即到账"
          >
            <Input
              value={redeemCode}
              onChange={(v) => setRedeemCode(v)}
              placeholder="请输入充值卡密"
              style={{ width: '100%' }}
            />
          </Form.Item>
          <Button type="primary" long loading={redeeming} onClick={doRedeem}>
            立即兑换
          </Button>
        </Form>
      </Modal>
    </div>
  );
}
