import { useState, useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { Card, Button, Spin, Space, Tag, Empty, Typography, Grid } from '@arco-design/web-react';
import { IconRefresh } from '@arco-design/web-react/icon';
import { api } from '../api';

const { Text } = Typography;
const { Row: GridRow, Col: GridCol } = Grid;

// 差距诊断（三缺口：内容 → 阵地 → 事实）
export default function GapDiagnose() {
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<any>(null);
  const location = useLocation();

  const load = async () => {
    setLoading(true);
    try {
      setData(await api.gapDiagnose());
    } catch (e: any) {
      /* 忽略 */
    } finally {
      setLoading(false);
    }
  };
  // 每次进入本页面（路由切换到差距诊断）自动拉取最新数据
  useEffect(() => { load(); /* eslint-disable-next-line */ }, [location.pathname]);

  const gap = (step: string, title: string, num: number, unit: string, desc: string, effect: string, color: string, list: string[], listLabel: string) => (
    <Card style={{ borderRadius: 12, height: '100%' }}>
      <Tag color={color} size="small">{step}</Tag>
      <div style={{ fontSize: 17, fontWeight: 700, marginTop: 8 }}>{title}</div>
      <div style={{ marginTop: 8, fontSize: 30, fontWeight: 800, color }}>
        {num}<span style={{ fontSize: 14, fontWeight: 400, color: '#86909C', marginLeft: 4 }}>{unit}</span>
      </div>
      <div style={{ fontSize: 13, color: '#86909C', marginTop: 6, lineHeight: 1.6 }}>{desc}</div>
      <div style={{ marginTop: 10, fontSize: 12, color: '#86909C' }}>
        <Tag color={color === '#F53F3F' ? 'red' : color === '#FF7D00' ? 'orange' : 'arcoblue'} size="small">{effect}</Tag>
      </div>
      {list && list.length > 0 ? (
        <div style={{ marginTop: 12 }}>
          <div style={{ fontSize: 12, color: '#86909C', marginBottom: 6 }}>{listLabel}</div>
          {list.map((x: string) => (
            <div key={x} style={{ padding: '5px 0', fontSize: 13, borderBottom: '1px solid var(--color-border-1)' }}>{x}</div>
          ))}
        </div>
      ) : (
        <div style={{ marginTop: 12, fontSize: 13, color: '#00B42A' }}>✓ {listLabel}暂无，保持现状</div>
      )}
    </Card>
  );

  return (
    <div style={{ paddingTop: 4 }}>
      <Card style={{ borderRadius: 12, marginBottom: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <div style={{ fontSize: 18, fontWeight: 700 }}>分数低只有三个原因，按顺序修</div>
            <div style={{ fontSize: 13, color: '#86909C', marginTop: 6 }}>
              AI 要引用你，需要同时满足：有东西可引（内容）、在它看的地方（阵地）、说得对（事实）。三者有序——内容没写，铺阵地是白铺。
            </div>
          </div>
          <Button icon={<IconRefresh />} loading={loading} onClick={load}>刷新</Button>
        </div>
      </Card>

      {loading ? (
        <div style={{ textAlign: 'center', padding: 60 }}><Spin size={24} /></div>
      ) : data ? (
        <GridRow gutter={[16, 16]}>
          <GridCol span={8}>
            {gap('第一步 · 先修', '内容缺口', data.content_gap, '个问题没有可被抽取的答案', '没有内容，铺阵地和改口径都没有落点。实测：含数字 +61.6%、定义 +57.3%、对比 +55.3% 被引用概率。', '影响最大', '#F53F3F', data.content_questions, '缺口问题')}
          </GridCol>
          <GridCol span={8}>
            {gap('第二步 · 再铺', '阵地缺口', data.channel_gap, '个阵地未被引用', '引擎实际会引用、你却没有内容的站点。优先建设高引用权重的阵地。', '见效最快', '#FF7D00', data.channel_list, '缺口阵地')}
          </GridCol>
          <GridCol span={8}>
            {gap('第三步 · 校准', '事实偏差', data.fact_gap, '处口径待修或未比对', '提及率上升会放大错误说法。从样本回放里人工比对后，在品牌事实库记录。', '越曝光越亏', '#165DFF', data.fact_list, '偏差口径')}
          </GridCol>
        </GridRow>
      ) : (
        <Empty description="暂无诊断数据，请先运行巡检任务" />
      )}
    </div>
  );
}
