import { useEffect, useRef, useState } from 'react';
import { Card, Button, Input, Message, Space, Tag, Alert, Checkbox, Radio, InputNumber, Spin, Table, Typography } from '@arco-design/web-react';
import { IconLaunch, IconDelete } from '@arco-design/web-react/icon';
import { api } from '../api';

const { Text } = Typography;

/* ================================================================
 * SQL 注入检测（sqlmap 图形界面）——站点体检页内嵌卡片
 *  - 目标默认取上方体检 URL；授权勾选强制
 *  - 选项组：技术/等级/风险/线程/表单/枚举；「高级参数」自由传参兜底
 *  - 任务制：start → 轮询 status/log → data 结果
 * ================================================================ */

interface SqlmapCardProps { defaultTarget: string; }

const TECHNIQUES = [
  { key: 'B', label: '布尔盲注' }, { key: 'E', label: '报错注入' }, { key: 'U', label: '联合查询' },
  { key: 'S', label: '堆叠注入' }, { key: 'T', label: '时间盲注' }, { key: 'Q', label: '内联查询' },
];

const ENUMS = [
  { key: 'getBanner', label: '数据库 Banner' },
  { key: 'getCurrentUser', label: '当前用户' },
  { key: 'getCurrentDb', label: '当前数据库' },
  { key: 'getDbs', label: '全部数据库' },
  { key: 'getTables', label: '全部数据表' },
  { key: 'getColumns', label: '全部字段' },
  { key: 'getPrivileges', label: '权限信息' },
  { key: 'getHostname', label: '主机名' },
];

export default function SqlmapCard({ defaultTarget }: SqlmapCardProps) {
  const [target, setTarget] = useState('');
  const [agreed, setAgreed] = useState(false);
  const [technique, setTechnique] = useState('BEUSTQ');
  const [level, setLevel] = useState(1);
  const [risk, setRisk] = useState(1);
  const [threads, setThreads] = useState(5);
  const [forms, setForms] = useState(false);
  const [enums, setEnums] = useState<string[]>(['getBanner', 'getCurrentDb', 'getDbs']);
  const [advArgs, setAdvArgs] = useState('');

  const [task, setTask] = useState('');
  const [running, setRunning] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const [data, setData] = useState<any>(null);
  const [err, setErr] = useState('');
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const poll = async (tid: string) => {
    try {
      const [st, lg] = await Promise.all([api.sqlmapStatus(tid), api.sqlmapLog(tid)]);
      const entries: string[] = Array.isArray(lg?.log) ? lg.log.map((x: any) => x.message).filter(Boolean) : [];
      setLogs((prev) => (entries.length > prev.length ? entries : prev));
      if (st?.status !== 'running') {
        setRunning(false);
        if (timerRef.current) clearInterval(timerRef.current);
        try {
          const d: any = await api.sqlmapData(tid);
          setData(d?.data || d || null);
        } catch { /* 忽略 */ }
      }
    } catch { /* 轮询失败不打断 */ }
  };

  const start = async () => {
    const t = (target || defaultTarget).trim().replace(/^https?:\/\//, '').replace(/\/.*$/, '');
    if (!t) { Message.warning('请输入要检测的站点域名'); return; }
    if (!agreed) { Message.warning('请先勾选授权声明：仅对有权测试的站点使用'); return; }
    setErr('');
    setData(null);
    setLogs([]);
    setRunning(true);
    try {
      const opts: any = {
        technique: technique || 'BEUSTQ',
        level, risk, threads, forms,
      };
      ENUMS.forEach((e) => { if (enums.includes(e.key)) opts[e.key] = true; });
      const r: any = await api.sqlmapStart({ url: t, options: opts });
      setTask(r.task);
      timerRef.current = setInterval(() => poll(r.task), 2000);
    } catch (e: any) {
      setRunning(false);
      setErr(e?.message || '检测启动失败');
    }
  };

  const cleanup = async () => {
    if (task) { try { await api.sqlmapDelete(task); } catch { /* 忽略 */ } }
    setTask('');
    setData(null);
    setLogs([]);
  };

  useEffect(() => () => { if (timerRef.current) clearInterval(timerRef.current); }, []);

  return (
    <Card
      title={<span>🛡️ SQL 注入检测（sqlmap）</span>}
      style={{ borderRadius: 12 }}
    >
      <Alert
        type="warning"
        style={{ marginBottom: 12 }}
        content={
          <Checkbox checked={agreed} onChange={setAgreed}>
            我确认仅对<b>有权测试的站点</b>使用本功能（自己的网站或已获书面授权的渗透测试目标），并自行承担全部责任
          </Checkbox>
        }
      />

      <Space style={{ width: '100%' }} direction="vertical" size={10}>
        <Input
          value={target}
          onChange={setTarget}
          placeholder="默认使用上方站点域名"
          style={{ width: 420 }}
        />

        <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap' }}>
          <div>
            <div style={{ fontSize: 12, color: '#86909c', marginBottom: 6 }}>注入技术（可多选）</div>
            <Space wrap size={4}>
              {TECHNIQUES.map((t) => (
                <Checkbox
                  key={t.key}
                  checked={technique.includes(t.key)}
                  onChange={(v) => setTechnique(v ? technique + t.key : technique.replace(t.key, ''))}
                >{t.label}</Checkbox>
              ))}
            </Space>
          </div>
          <div>
            <div style={{ fontSize: 12, color: '#86909c', marginBottom: 6 }}>检测等级 / 风险 / 线程</div>
            <Space size={8}>
              <span style={{ fontSize: 12 }}>等级</span>
              <InputNumber min={1} max={5} value={level} onChange={(v) => setLevel(v || 1)} style={{ width: 64 }} />
              <span style={{ fontSize: 12 }}>风险</span>
              <InputNumber min={1} max={3} value={risk} onChange={(v) => setRisk(v || 1)} style={{ width: 64 }} />
              <span style={{ fontSize: 12 }}>线程</span>
              <InputNumber min={1} max={10} value={threads} onChange={(v) => setThreads(v || 1)} style={{ width: 64 }} />
            </Space>
          </div>
          <Checkbox checked={forms} onChange={setForms}>自动搜索表单参数</Checkbox>
        </div>

        <div>
          <div style={{ fontSize: 12, color: '#86909c', marginBottom: 6 }}>枚举内容（可多选）</div>
          <Space wrap size={4}>
            {ENUMS.map((e) => (
              <Checkbox
                key={e.key}
                checked={enums.includes(e.key)}
                onChange={(v) => setEnums(v ? [...enums, e.key] : enums.filter((x) => x !== e.key))}
              >{e.label}</Checkbox>
            ))}
          </Space>
        </div>

        <Input
          value={advArgs}
          onChange={setAdvArgs}
          placeholder="高级参数（选填）：如 cookie=xxx, userAgent=xxx, proxy=http://127.0.0.1:8080"
          style={{ width: 420 }}
        />

        <Space>
          <Button type="primary" icon={<IconLaunch />} loading={running} onClick={start}>
            {running ? '检测中…' : '开始检测'}
          </Button>
          {task && <Button icon={<IconDelete />} onClick={cleanup}>结束并清理任务</Button>}
          {task && <Tag color="arcoblue" size="small">任务 {task.slice(0, 8)}…</Tag>}
        </Space>
      </Space>

      {err && <Alert type="error" style={{ marginTop: 12 }} content={err} />}

      {logs.length > 0 && (
        <div style={{ marginTop: 12, background: 'var(--color-fill-2)', borderRadius: 8, padding: '10px 14px', maxHeight: 180, overflow: 'auto' }}>
          {logs.slice(-50).map((l, i) => (
            <div key={i} style={{ fontSize: 12, color: '#4e5969', lineHeight: 1.7, wordBreak: 'break-all' }}>{l}</div>
          ))}
        </div>
      )}

      {data && Array.isArray(data) && data.length > 0 && (
        <div style={{ marginTop: 12 }}>
          <Text style={{ fontSize: 13, fontWeight: 600 }}>检测结果</Text>
          <Table
            rowKey={(r: any) => r.place + '-' + r.parameter + '-' + r.length}
            data={data}
            pagination={false}
            size="small"
            style={{ marginTop: 8 }}
            columns={[
              { title: '参数', dataIndex: 'parameter', width: 160, ellipsis: true },
              {
                title: '注入类型',
                dataIndex: 'data',
                ellipsis: true,
                render: (v: any) => {
                  const t = v && (v.title || v['1']?.title || v['2']?.title || v['3']?.title || JSON.stringify(v).slice(0, 60));
                  return <span style={{ fontSize: 12 }}>{t}</span>;
                },
              },
              { title: '占位符', dataIndex: 'place', width: 100 },
              { title: 'payload 数', dataIndex: 'length', width: 90, align: 'center' as const },
            ]}
          />
        </div>
      )}
      {running && <Spin style={{ marginTop: 12 }} size={16} />}
    </Card>
  );
}
