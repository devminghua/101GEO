import { useState, type CSSProperties } from 'react';
import { Card, Button, Message, Space, Select, Typography, Modal } from '@arco-design/web-react';
import { IconFile, IconThunderbolt } from '@arco-design/web-react/icon';
import { api } from '../api';

const cardStyle: CSSProperties = {
  borderRadius: 16,
  boxShadow: '0 2px 14px rgba(0, 0, 0, 0.05)',
};

// 巡检与报告（原设置页 ops tab，移入 GEO 智能·阵地地图之后）
export default function OpsReportTab() {
  const [reportModal, setReportModal] = useState(false);
  const [reportLoading, setReportLoading] = useState(false);
  const [report, setReport] = useState<any>(null);
  const [reportDays, setReportDays] = useState(7);

  const genReport = async () => {
    setReportLoading(true);
    try {
      const r = await api.generateReport(reportDays);
      setReport(r);
      setReportModal(true);
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setReportLoading(false);
    }
  };

  return (
    <>
      <Card
        title="定时自动巡检"
        style={{ ...cardStyle, marginBottom: 16 }}
        bordered={false}
        extra={
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            <IconThunderbolt style={{ marginRight: 4 }} />
            每 60 分钟（.env GEO_CRON_MINUTES 调整）在 8:00-22:00 时段自动巡检；关闭请设 GEO_CRON_ENABLED=false
          </Typography.Text>
        }
      />
      <Card title="GEO 报告" style={cardStyle} bordered={false}>
        <Space size="large" wrap>
          <Space>
            <span>统计周期：</span>
            <Select
              value={reportDays}
              onChange={setReportDays}
              style={{ width: 120 }}
              options={[7, 14, 30].map((d) => ({ label: `近 ${d} 天`, value: d }))}
            />
          </Space>
          <Button type="primary" icon={<IconFile />} loading={reportLoading} onClick={genReport}>
            生成 GEO 报告
          </Button>
        </Space>
        <Modal
          title={`GEO 优化报告（近 ${reportDays} 天）`}
          visible={reportModal}
          footer={null}
          onCancel={() => setReportModal(false)}
          style={{ width: 760 }}
          unmountOnExit
        >
          {report && (
            <pre
              style={{
                whiteSpace: 'pre-wrap',
                background: 'var(--color-fill-2)',
                padding: 16,
                borderRadius: 8,
                maxHeight: 560,
                overflow: 'auto',
                fontSize: 13,
                lineHeight: 1.7,
              }}
            >
              {report.report}
            </pre>
          )}
        </Modal>
      </Card>
    </>
  );
}
