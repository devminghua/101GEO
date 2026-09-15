import { useState } from 'react';
import { Card, Grid } from '@arco-design/web-react';
import NmapCard from '../../components/NmapCard';
import SqlmapCard from '../../components/SqlmapCard';

const { Row, Col } = Grid;

/* ================================================================
 * 安全检测（总后台专属，老板 2026-09-15 拍板）
 *  - 端口扫描（Nmap 功能）+ SQL 注入检测（sqlmap）
 *  - 客户端不显示本页，接口也仅 super 可调
 * ================================================================ */

export default function Security() {
  const [target] = useState('');
  return (
    <div style={{ padding: '4px 0 24px' }}>
      <Card style={{ borderRadius: 12, marginBottom: 16 }}>
        <div style={{ fontSize: 16, fontWeight: 600, marginBottom: 6 }}>安全检测</div>
        <div style={{ fontSize: 13, color: '#86909C' }}>
          对站点域名做端口暴露面扫描与 SQL 注入检测。仅用于<b>自有资产或已获书面授权</b>的渗透测试目标，请遵守当地法律法规。
        </div>
      </Card>
      <Row gutter={[16, 16]}>
        <Col xs={24} xl={12}>
          <NmapCard />
        </Col>
        <Col xs={24} xl={12}>
          <SqlmapCard defaultTarget={target} />
        </Col>
      </Row>
    </div>
  );
}
