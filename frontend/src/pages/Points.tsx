import { useEffect, useRef, useState } from 'react';
import {
  Card, Table, Statistic, Space, Tag, TableColumnProps, Tabs,
  Button, Modal, Form, Select, Input, Message, Typography, Empty, Slider,
} from '@arco-design/web-react';
import { QRCodeSVG } from 'qrcode.react';
import { useSearchParams } from 'react-router-dom';
import { api } from '../api';
import { useTranslation } from 'react-i18next';
import UsageDashboard from './UsageDashboard';

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

/* ================================================================
 * 邀约奖励卡（2026-09-13 老板规则）：
 *   - 邀请 1 人注册 → 本人 +2000 token + 服务延长 1 个月（上不封顶）
 *   - 被邀请人注册成功 → 自动充值 2000 token
 * 数据源 /api/invite/summary；邀请链接 = 注册页 + ?ref=邀请码
 * ================================================================ */
function InviteCard() {
  const [data, setData] = useState<any>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    api
      .inviteSummary()
      .then((d: any) => setData(d))
      .catch(() => {})
      .finally(() => setLoaded(true));
  }, []);

  const inviteLink = `${window.location.origin}/#/register?ref=${data?.invite_code || ''}`;

  const copyLink = async () => {
    if (!data?.invite_code) return;
    try {
      await navigator.clipboard.writeText(inviteLink);
      Message.success('邀请链接已复制，发给客户注册即可');
    } catch {
      Message.warning('复制失败，请手动复制');
    }
  };

  return (
    <Card style={{ marginBottom: 16, background: 'linear-gradient(135deg, #FDF6FF 0%, #FFF9F0 100%)' }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 20, flexWrap: 'wrap' }}>
        {/* 左：规则说明 */}
        <div style={{ flex: '1 1 320px', minWidth: 260 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
            <span style={{ fontSize: 20 }}>🎁</span>
            <span style={{ fontSize: 16, fontWeight: 700, color: 'var(--geo-text)' }}>邀约奖励</span>
            <Tag color="magenta" size="small">上不封顶</Tag>
          </div>
          <div style={{ color: '#4e5969', fontSize: 13, lineHeight: 2 }}>
            <div>· 邀请 1 个新客户注册 → <b>本人 +2000 token</b>，服务<b>延长 1 个月使用</b>（上不封顶）</div>
            <div>· 被邀请的新人注册成功 → <b>自动充值 2000 token</b></div>
            <div style={{ color: '#86909c', fontSize: 12 }}>
              注册方式：客户打开你的邀请链接注册，或注册时填写你的邀请码
            </div>
          </div>
        </div>
        {/* 右：邀请码 + 数据 + 记录 */}
        <div style={{ flex: '1 1 340px', minWidth: 280 }}>
          {loaded && !data?.invite_code ? (
            <div style={{ color: '#86909c', fontSize: 13 }}>暂不可用（总后台账号无邀请码）</div>
          ) : (
            <>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                <div>
                  <div style={{ fontSize: 12, color: '#86909c' }}>我的邀请码</div>
                  <div style={{ fontSize: 22, fontWeight: 800, color: '#722ED1', letterSpacing: 1 }}>
                    {data?.invite_code || '—'}
                  </div>
                </div>
                <div style={{ flex: 1 }} />
                <Space direction="vertical" size={4} align="end">
                  <Space size={8}>
                    <Tag color="green" size="small">已邀请 {data?.total_invited || 0} 人</Tag>
                    <Tag color="arcoblue" size="small">累计 +{data?.total_reward || 0} token</Tag>
                  </Space>
                  <Button type="primary" size="small" icon={<span style={{ fontSize: 14 }}>📋</span>} onClick={copyLink}>
                    复制邀请链接
                  </Button>
                </Space>
              </div>
              {(data?.records || []).length > 0 && (
                <div style={{ marginTop: 10, borderTop: '1px dashed #f0e0f8', paddingTop: 8 }}>
                  {data.records.slice(0, 5).map((r: any) => (
                    <div key={r.id} style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: '#4e5969', lineHeight: 1.9 }}>
                      <span>{r.company_name || r.phone}（{r.phone}）</span>
                      <span>
                        <Tag size="small" color={r.status === 1 ? 'green' : 'orange'}>
                          {r.status === 1 ? `已奖励 +${r.reward_points}` : '待注册'}
                        </Tag>
                        <span style={{ color: '#a9aeb8', marginLeft: 6 }}>{r.created_at}</span>
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </Card>
  );
}

// 套餐版本图标：从猿人到星辰大海（进化主题，对应三档套餐）
const PLAN_ICONS = ['🦍', '🚀', '🌌'];
const THUMB_W = 48; // 滑块手柄宽（与登录页滑动解锁一致）

// 支付渠道图标卡片（微信 / 支付宝）
function PayChannel({
  active, color, glyph, label, onClick,
}: {
  active: boolean; color: string; glyph: string; label: string; onClick: () => void;
}) {
  return (
    <div
      onClick={onClick}
      style={{
        flex: 1, display: 'flex', alignItems: 'center', gap: 10, padding: '11px 14px',
        borderRadius: 14, cursor: 'pointer', userSelect: 'none',
        border: `2px solid ${active ? color : 'var(--color-border-2)'}`,
        background: active ? `${color}14` : 'var(--color-fill-1)',
        transition: 'all .15s',
      }}
    >
      <span
        style={{
          width: 30, height: 30, borderRadius: '50%', background: color, color: '#fff',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: 14, fontWeight: 700, flexShrink: 0,
        }}
      >
        {glyph}
      </span>
      <span style={{ fontWeight: 600, fontSize: 14, color: 'var(--geo-text)' }}>{label}</span>
      {active && <span style={{ marginLeft: 'auto', color, fontSize: 16, fontWeight: 700 }}>✓</span>}
    </div>
  );
}

export default function Points() {
  const { t } = useTranslation();
  const [balance, setBalance] = useState(0);
  const [records, setRecords] = useState<PointRecord[]>([]);
  const [loading, setLoading] = useState(false);
  // 页签：充值兑换 / Token 用量（v1.0.57 集成，?tab=usage 直达用量看板）
  const [searchParams] = useSearchParams();
  const [tab, setTab] = useState(searchParams.get('tab') === 'usage' ? 'usage' : 'recharge');

  // 扫码充值 Modal 状态
  const [rechargeVisible, setRechargeVisible] = useState(false);
  const [channel, setChannel] = useState<'wechat' | 'alipay'>('wechat');
  const [plans, setPlans] = useState<any[]>([]);
  const [selectedPlan, setSelectedPlan] = useState<any>(null);
  // 套餐滑块索引（三个版本：猿人 → 火箭 → 星辰大海）
  const [slideIdx, setSlideIdx] = useState(0);
  const [dragging, setDragging] = useState(false); // 滑块拖动中
  const [thumbX, setThumbX] = useState(0); // 手柄位置（相对轨道）
  const dragRef = useRef({ startX: 0, startThumb: 0 });
  const trackRef = useRef<HTMLDivElement>(null);

  // 档位手柄位置：usable 宽按档位数均分
  const stopPos = (i: number) => {
    const usable = (trackRef.current?.offsetWidth || 480) - THUMB_W;
    return plans.length <= 1 ? 0 : usable * (i / (plans.length - 1));
  };

  // 手柄位置 → 最近档位索引
  const nearestIdx = (x: number) => {
    const usable = (trackRef.current?.offsetWidth || 480) - THUMB_W;
    if (plans.length <= 1) return 0;
    const r = Math.round(x / (usable / (plans.length - 1)));
    return Math.max(0, Math.min(plans.length - 1, r));
  };

  // 滑动手柄：按住拖动实时跟随，松手吸附到最近档位（登录页滑动解锁同款手感）
  useEffect(() => {
    if (!dragging) return;
    const move = (e: MouseEvent) => {
      const usable = (trackRef.current?.offsetWidth || 480) - THUMB_W;
      const nx = Math.max(0, Math.min(usable, dragRef.current.startThumb + (e.clientX - dragRef.current.startX)));
      setThumbX(nx);
      const idx = nearestIdx(nx);
      setSlideIdx(idx);
      if (plans[idx]) setSelectedPlan(plans[idx]);
    };
    const up = () => {
      setDragging(false);
      const idx = nearestIdx(thumbX);
      pickSlide(idx);
      setThumbX(stopPos(idx));
    };
    const touchMove = (e: TouchEvent) => {
      const t = e.touches[0];
      if (!t) return;
      const usable = (trackRef.current?.offsetWidth || 480) - THUMB_W;
      const nx = Math.max(0, Math.min(usable, dragRef.current.startThumb + (t.clientX - dragRef.current.startX)));
      setThumbX(nx);
      const idx = nearestIdx(nx);
      setSlideIdx(idx);
      if (plans[idx]) setSelectedPlan(plans[idx]);
    };
    const touchEnd = () => {
      setDragging(false);
      const idx = nearestIdx(thumbX);
      pickSlide(idx);
      setThumbX(stopPos(idx));
    };
    document.addEventListener('mousemove', move);
    document.addEventListener('mouseup', up);
    document.addEventListener('touchmove', touchMove);
    document.addEventListener('touchend', touchEnd);
    return () => {
      document.removeEventListener('mousemove', move);
      document.removeEventListener('mouseup', up);
      document.removeEventListener('touchmove', touchMove);
      document.removeEventListener('touchend', touchEnd);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dragging, plans.length]);
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

  // 打开充值弹窗：加载套餐列表，默认选中第一档
  const openRecharge = async () => {
    setRechargeVisible(true);
    setOrder(null);
    setSlideIdx(0);
    setThumbX(0);
    try {
      const list = (await api.listRechargePlans()) || [];
      setPlans(list);
      setSelectedPlan(list[0] || null);
    } catch {
      setPlans([]);
      setSelectedPlan(null);
    }
  };

  // 滑块切档：同步选中套餐
  const pickSlide = (i: number) => {
    const idx = Math.max(0, Math.min(plans.length - 1, i));
    setSlideIdx(idx);
    setSelectedPlan(plans[idx] || null);
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
      <div style={{ fontSize: 20, fontWeight: 600, marginBottom: 16 }}>{t('pointsPage.title')}</div>
      <Tabs activeTab={tab} onChange={setTab} type="line" style={{ marginBottom: 16 }}>
        <Tabs.TabPane key="recharge" title={t('pointsPage.tabRecharge')} />
        <Tabs.TabPane key="usage" title={t('pointsPage.tabUsage')} />
      </Tabs>

      {tab === 'usage' ? (
        <UsageDashboard />
      ) : (
      <div>
      <Card style={{ marginBottom: 16 }}>
        <Space size="large" align="start">
          <Statistic title={t('pointsPage.balance')} value={balance} groupSeparator suffix="token" />
          <div style={{ color: 'var(--color-text-3)', fontSize: 13, paddingTop: 6, maxWidth: 420 }}>
            {t('pointsPage.hint')}
            {t('pointsPage.hint2')}
          </div>
          <Space>
            <Button type="primary" onClick={openRecharge}>充值</Button>
            <Button type="outline" onClick={() => setRedeemVisible(true)}>{t('pointsPage.cardRedeem')}</Button>
          </Space>
        </Space>
      </Card>
      {/* 邀约奖励：邀请新客户注册，双向得 2000 token，邀请人还延 1 个月使用（上不封顶） */}
      <InviteCard />
      <Card title={t('pointsPage.flowTitle')}>
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          data={records}
          pagination={{ pageSize: 10, showTotal: true }}
        />
      </Card>
      </div>
      )}

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
            {/* 支付渠道：微信 / 支付宝图标选择 */}
            <div style={{ marginBottom: 18 }}>
              <div style={{ marginBottom: 8 }}><Typography.Text bold>支付渠道</Typography.Text></div>
              <div style={{ display: 'flex', gap: 12 }}>
                <PayChannel
                  active={channel === 'wechat'}
                  color="#07C160"
                  glyph="微"
                  label="微信支付"
                  onClick={() => setChannel('wechat')}
                />
                <PayChannel
                  active={channel === 'alipay'}
                  color="#1677FF"
                  glyph="支"
                  label="支付宝"
                  onClick={() => setChannel('alipay')}
                />
              </div>
            </div>

            {plans.length === 0 ? (
              <Empty description="暂无可用套餐，请联系总后台设置" />
            ) : (
              <div>
                {/* 套餐卡片横排：金额 + token + 折扣 + 选择按钮（推荐档高亮） */}
                <div style={{ marginBottom: 8 }}><Typography.Text bold>选择充值套餐</Typography.Text></div>
                <div style={{ display: 'grid', gridTemplateColumns: `repeat(${plans.length}, 1fr)`, gap: 10 }}>
                  {plans.map((p, i) => {
                    const active = selectedPlan?.id === p.id;
                    const discount = p.orig_fen > 0 && p.orig_fen > p.price_fen
                      ? Math.round((p.price_fen / p.orig_fen) * 10) : 0;
                    return (
                      <div
                        key={p.id}
                        onClick={() => pickSlide(i)}
                        style={{
                          position: 'relative',
                          borderRadius: 12,
                          padding: '16px 8px 10px',
                          textAlign: 'center',
                          cursor: 'pointer',
                          userSelect: 'none',
                          border: `1.5px solid ${active ? '#4860D8' : 'var(--color-border-2)'}`,
                          background: active ? 'rgba(72,96,216,0.06)' : 'var(--color-fill-1)',
                          boxShadow: active ? '0 4px 14px rgba(72,96,216,.18)' : 'none',
                          transition: 'all .15s',
                        }}
                      >
                        {i === 1 && plans.length > 2 && (
                          <span style={{
                            position: 'absolute', top: -9, left: '50%', transform: 'translateX(-50%)',
                            background: '#4860D8', color: '#fff', fontSize: 10, fontWeight: 600,
                            padding: '2px 10px', borderRadius: 9, whiteSpace: 'nowrap',
                          }}>推荐</span>
                        )}
                        <div style={{ fontSize: 22, lineHeight: 1 }}>
                          {PLAN_ICONS[i % PLAN_ICONS.length]}
                        </div>
                        <div style={{ fontSize: 12, fontWeight: 600, marginTop: 6, color: 'var(--geo-text)' }}>
                          {p.name}
                        </div>
                        <div style={{ fontSize: 17, fontWeight: 800, marginTop: 6, color: 'var(--geo-text)' }}>
                          ¥{(p.price_fen / 100).toFixed(0)}元
                        </div>
                        <div style={{ fontSize: 11, color: '#86909C', marginTop: 2 }}>
                          {p.points.toLocaleString('en-US')} token
                        </div>
                        {discount > 0 && discount < 10 ? (
                          <div style={{ fontSize: 11, color: '#F53F3F', marginTop: 3 }}>
                            {discount} 折优惠
                          </div>
                        ) : p.tag ? (
                          <div style={{ fontSize: 11, color: '#F53F3F', marginTop: 3 }}>{p.tag}</div>
                        ) : null}
                        <div style={{
                          marginTop: 9, padding: '7px 0', borderRadius: 8,
                          background: active ? '#4860D8' : 'var(--color-fill-2)',
                          color: active ? '#fff' : 'var(--color-text-2)',
                          fontSize: 12, fontWeight: 600,
                        }}>
                          {active ? '立即充值' : '选择'}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
            <Button
              type="primary"
              long
              loading={creating}
              disabled={!selectedPlan}
              onClick={createOrder}
              style={{ marginTop: 18, height: 42, fontSize: 15 }}
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
