import { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Tag,
  Typography,
  Select,
  Space,
  Message,
  Button,
} from '@arco-design/web-react';
import { IconRefresh, IconDownload } from '@arco-design/web-react/icon';
import { api } from '../../api';

// 续费记录：总后台给客户续费的流水（谁在什么时候给哪个客户续了几个月），供对账
export default function ExtendRecords({ tenants = [] }: { tenants?: any[] }) {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [totalMonths, setTotalMonths] = useState(0);
  const [totalAmountFen, setTotalAmountFen] = useState(0);
  const [tenantFilter, setTenantFilter] = useState<number | undefined>(undefined);
  const [monthFilter, setMonthFilter] = useState<string | undefined>(undefined);

  // 可选月份：从流水最早记录到当前月
  const months: string[] = [];
  const now = new Date();
  for (let i = 0; i < 24; i++) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
    months.push(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`);
  }

  const load = async () => {
    setLoading(true);
    try {
      const data: any = await api.listExtendRecords({
        tenant_id: tenantFilter,
        month: monthFilter,
        limit: 500,
      });
      setList(data?.list || []);
      setTotal(data?.total || 0);
      setTotalMonths(data?.total_months || 0);
      setTotalAmountFen(data?.total_amount_fen || 0);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenantFilter, monthFilter]);

  // 分转元显示
  const fen2yuan = (fen: number) => `¥${((fen || 0) / 100).toFixed(2)}`;

  // 导出 CSV（对账用）
  const exportCsv = () => {
    const header = ['时间', '操作者', '客户（分站）', '账号', '续费月数', '月单价', '金额（元）', '收款方式', '续费前到期', '续费后到期', '备注'];
    const rows = list.map((r) => [
      r.created_at ? new Date(r.created_at).toLocaleString() : '',
      r.operator,
      r.tenant_name,
      r.username,
      r.months,
      fen2yuan(r.price_fen),
      fen2yuan(r.amount_fen),
      r.pay_method || '',
      r.expire_before ? new Date(r.expire_before).toLocaleDateString() : '原不限',
      new Date(r.expire_after).toLocaleDateString(),
      r.remark || '',
    ]);
    // 合计行
    const sumAmount = list.reduce((s, r) => s + (r.amount_fen || 0), 0);
    const sumMonths = list.reduce((s, r) => s + (r.months || 0), 0);
    rows.push(['合计', '', '', '', sumMonths, '', fen2yuan(sumAmount), '', '', '', '']);
    const csv = [header, ...rows]
      .map((cells) => cells.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(','))
      .join('\n');
    const blob = new Blob(['\ufeff' + csv], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `续费流水_${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const columns = [
    {
      title: '时间',
      dataIndex: 'created_at',
      width: 170,
      render: (v: string) => (v ? new Date(v).toLocaleString() : '-'),
    },
    { title: '操作者', dataIndex: 'operator', width: 100 },
    {
      title: '客户（分站）',
      dataIndex: 'tenant_name',
      render: (v: string, row: any) => (
        <span>
          <Tag color="arcoblue" size="small">{row.tenant_id}</Tag> {v}
        </span>
      ),
    },
    { title: '账号', dataIndex: 'username', width: 120 },
    {
      title: '续费月数',
      dataIndex: 'months',
      width: 100,
      render: (v: number) => <Tag color="green">+{v} 个月</Tag>,
    },
    {
      title: '金额',
      dataIndex: 'amount_fen',
      width: 120,
      render: (v: number, row: any) => (
        <span>
          <b style={{ color: 'rgb(var(--orange-6))' }}>{fen2yuan(v)}</b>
          <Typography.Text type="secondary" style={{ fontSize: 11, marginLeft: 4 }}>
            （{fen2yuan(row.price_fen)}×{row.months}）
          </Typography.Text>
        </span>
      ),
    },
    {
      title: '收款方式',
      dataIndex: 'pay_method',
      width: 90,
      render: (v: string, row: any) => {
        if (!v) return <Typography.Text type="secondary">-</Typography.Text>;
        const color = v === '赠送' ? 'purple' : v === '微信' ? 'green' : v === '支付宝' ? 'arcoblue' : 'gray';
        return (
          <span>
            <Tag color={color} size="small">{v}</Tag>
            {row.amount_fen === 0 && v !== '赠送' && (
              <Typography.Text type="secondary" style={{ fontSize: 11 }}> ¥0</Typography.Text>
            )}
          </span>
        );
      },
    },
    {
      title: '续费前 → 续费后',
      width: 220,
      render: (_: any, row: any) => (
        <Typography.Text style={{ fontSize: 13 }}>
          {row.expire_before ? new Date(row.expire_before).toLocaleDateString() : '原不限'}
          {' → '}
          <b>{new Date(row.expire_after).toLocaleDateString()}</b>
        </Typography.Text>
      ),
    },
    { title: '备注', dataIndex: 'remark', render: (v: string) => v || '-' },
  ];

  return (
    <Card
      title={
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span>续费记录</span>
          <Space>
            <Select
              size="small"
              placeholder="全部客户"
              style={{ width: 160 }}
              allowClear
              value={tenantFilter}
              onChange={(v) => setTenantFilter(v || undefined)}
            >
              {tenants.map((t) => (
                <Select.Option key={t.id} value={t.id}>
                  {t.name}（{t.code}）
                </Select.Option>
              ))}
            </Select>
            <Select
              size="small"
              placeholder="全部月份"
              style={{ width: 130 }}
              allowClear
              value={monthFilter}
              onChange={(v) => setMonthFilter(v || undefined)}
            >
              {months.map((m) => (
                <Select.Option key={m} value={m}>
                  {m}
                </Select.Option>
              ))}
            </Select>
            <Button size="small" icon={<IconRefresh />} onClick={load}>
              刷新
            </Button>
            <Button size="small" icon={<IconDownload />} onClick={exportCsv} disabled={!list.length}>
              导出 CSV
            </Button>
          </Space>
        </div>
      }
      extra={
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          共 {total} 条 · 合计 {totalMonths} 个月 · 合计金额 {fen2yuan(totalAmountFen)}
        </Typography.Text>
      }
      style={{ borderRadius: 12 }}
      bordered
    >
      <Table rowKey="id" columns={columns} data={list} loading={loading} pagination={list.length > 20 ? { pageSize: 20 } : false} size="middle" />
    </Card>
  );
}
