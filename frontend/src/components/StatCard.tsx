import { Card, Statistic } from '@arco-design/web-react';

// 仪表盘数据卡片
export default function StatCard({
  title,
  value,
  suffix = '',
  precision = 0,
  icon,
  color = '#165DFF',
  extra,
}: {
  title: string;
  value: number | string;
  suffix?: string;
  precision?: number;
  icon?: React.ReactNode;
  color?: string;
  extra?: React.ReactNode;
}) {
  return (
    <Card bordered hoverable style={{ borderRadius: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <div style={{ color: 'var(--color-text-2)', fontSize: 14, marginBottom: 8 }}>
            {title}
            {extra}
          </div>
          <Statistic value={value as any} suffix={suffix} precision={precision} style={{ color, fontWeight: 700 }} />
        </div>
        {icon ? (
          <div
            style={{
              width: 44,
              height: 44,
              borderRadius: 10,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: color + '1A',
              color,
              fontSize: 22,
            }}
          >
            {icon}
          </div>
        ) : null}
      </div>
    </Card>
  );
}
