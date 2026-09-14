import { useEffect, useState } from 'react';
import { Card, Form, Input, Button, Message, Tag, Alert, Spin } from '@arco-design/web-react';
import { IconSave } from '@arco-design/web-react/icon';
import { useTranslation } from 'react-i18next';
import { api } from '../api';

/* ================================================================
 * 国际数据源设置（入口：侧栏「国际搜索优化」下拉最底部，老板 2026-09-14 拍板）
 *  - super：可编辑保存（平台级密钥）
 *  - 分站：只读状态 + 提示联系平台管理员
 * ================================================================ */

export default function IntlDataSource() {
  const { t } = useTranslation();
  const [cfg, setCfg] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [serper, setSerper] = useState('');
  const [serpapi, setSerpapi] = useState('');
  const [dlId, setDlId] = useState('');
  const [dlSecret, setDlSecret] = useState('');
  const [saving, setSaving] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const c = await api.intlDataSourceGet();
      setCfg(c || null);
    } catch { /* 忽略 */ } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const save = async () => {
    const anySet = serper.trim() || serpapi.trim() || dlId.trim() || dlSecret.trim();
    if (!anySet) { Message.warning(t('intlDs.empty')); return; }
    if ((dlId.trim() && !dlSecret.trim()) || (!dlId.trim() && dlSecret.trim())) {
      Message.warning(t('intlDs.pairRequired')); return;
    }
    setSaving(true);
    try {
      const res = await api.intlDataSourceSave({ serper_key: serper.trim(), serpapi_key: serpapi.trim(), datalab_client_id: dlId.trim(), datalab_client_secret: dlSecret.trim() });
      setSerper(''); setSerpapi(''); setDlId(''); setDlSecret('');
      setCfg(res as any);
      Message.success(t('intlDs.saved'));
    } catch (e: any) {
      Message.error(e?.message || t('intlDs.saveFailed'));
    } finally {
      setSaving(false);
    }
  };

  const card = { borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' };

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 900, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4, flexWrap: 'wrap' }}>
        <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#4285F4,#34A853,#FBBC05,#EA4335)' }} />
        <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>{t('intlDs.title')}</span>
      </div>
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 16 }}>{t('intlDs.sub')}</div>

      {loading ? (
        <div style={{ padding: '60px 0', textAlign: 'center' }}><Spin size={24} /></div>
      ) : (
        <>
          {/* Serper */}
          <Card title="Google SERP（Serper.dev）" style={card} bordered={false}>
            <Alert type="info" style={{ marginBottom: 16 }} content={t('intlDs.serperHint')} />
            <Form layout="vertical" style={{ maxWidth: 560 }}>
              <Form.Item label={t('intlDs.status')}>
                {cfg?.serper_enabled ? <Tag color="green">{t('intlDs.configured')}（{cfg?.serper_masked}）</Tag> : <Tag color="orange">{t('intlDs.notConfigured')}</Tag>}
                {cfg?.serper_own && <Tag color="arcoblue" size="small">{t('intlDs.ownKey')}</Tag>}
              </Form.Item>
              <Form.Item label="Serper API Key" extra={t('intlDs.keepEmpty')}>
                <Input.Password value={serper} onChange={setSerper} placeholder="Serper.dev API Key" autoComplete="new-password" />
              </Form.Item>
            </Form>
          </Card>

          {/* SerpAPI */}
          <Card title="Naver SERP + Google Trends（SerpAPI）" style={{ ...card, marginTop: 16 }} bordered={false}>
            <Alert type="info" style={{ marginBottom: 16 }} content={t('intlDs.serpapiHint')} />
            <Form layout="vertical" style={{ maxWidth: 560 }}>
              <Form.Item label={t('intlDs.status')}>
                {cfg?.serpapi_enabled ? <Tag color="green">{t('intlDs.configured')}（{cfg?.serpapi_masked}）</Tag> : <Tag color="orange">{t('intlDs.notConfigured')}</Tag>}
                {cfg?.serpapi_own && <Tag color="arcoblue" size="small">{t('intlDs.ownKey')}</Tag>}
              </Form.Item>
              <Form.Item label="SerpAPI Key" extra={t('intlDs.keepEmpty')}>
                <Input.Password value={serpapi} onChange={setSerpapi} placeholder="SerpAPI Key" autoComplete="new-password" />
              </Form.Item>
            </Form>
          </Card>

          {/* Datalab */}
          <Card title="Naver 行业排行（Datalab 官方）" style={{ ...card, marginTop: 16 }} bordered={false}>
            <Alert type="info" style={{ marginBottom: 16 }} content={t('intlDs.datalabHint')} />
            <Form layout="vertical" style={{ maxWidth: 560 }}>
              <Form.Item label={t('intlDs.status')}>
                {cfg?.datalab_enabled ? <Tag color="green">{t('intlDs.configured')}（{cfg?.datalab_masked}）</Tag> : <Tag color="orange">{t('intlDs.notConfigured')}</Tag>}
                {cfg?.datalab_own && <Tag color="arcoblue" size="small">{t('intlDs.ownKey')}</Tag>}
              </Form.Item>
              <Form.Item label="Client ID" extra={t('intlDs.keepEmpty')}>
                <Input value={dlId} onChange={setDlId} placeholder="Naver Datalab Client ID" autoComplete="off" />
              </Form.Item>
              <Form.Item label="Client Secret" extra={t('intlDs.keepEmpty')}>
                <Input.Password value={dlSecret} onChange={setDlSecret} placeholder="Naver Datalab Client Secret" autoComplete="new-password" />
              </Form.Item>
            </Form>
          </Card>

          <div style={{ marginTop: 20 }}>
            <Button type="primary" size="large" icon={<IconSave />} loading={saving} onClick={save}>
              {t('intlDs.saveAll')}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
