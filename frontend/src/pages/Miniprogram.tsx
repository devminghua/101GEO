import { useEffect, useState } from 'react';
import { Card, Form, Input, Button, Message, Tag, Switch, Alert, Space, Typography, Spin } from '@arco-design/web-react';
import { IconSave, IconRefresh, IconCopy } from '@arco-design/web-react/icon';
import { useTranslation } from 'react-i18next';
import { api } from '../api';

const { Text } = Typography;

/* ================================================================
 * 小程序设置（分站客户端「系统中心 → 小程序设置」）
 *  - 配置小程序 AppID/AppSecret/启用开关
 *  - 展示对接参数：API 域名、对接密钥（可轮换）、白名单域名
 * ================================================================ */

export default function Miniprogram() {
  const { t } = useTranslation();
  const [cfg, setCfg] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [appid, setAppid] = useState('');
  const [secret, setSecret] = useState('');
  const [enabled, setEnabled] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const c: any = await api.miniprogramConfig();
      setCfg(c || null);
      setEnabled(c?.enabled ?? true);
    } catch { /* 忽略 */ } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);

  const save = async () => {
    if (!appid.trim() && !secret.trim()) { Message.warning(t('mp.emptyWarn')); return; }
    setSaving(true);
    try {
      await api.miniprogramSave({ appid: appid.trim(), appsecret: secret.trim(), enabled });
      setAppid(''); setSecret('');
      Message.success(t('mp.saved'));
      await load();
    } catch (e: any) {
      Message.error(e?.message || t('mp.saveFailed'));
    } finally {
      setSaving(false);
    }
  };

  const rotate = async () => {
    try {
      await api.miniprogramRotateKey();
      Message.success(t('mp.rotated'));
      await load();
    } catch (e: any) {
      Message.error(e?.message || t('mp.saveFailed'));
    }
  };

  const copy = (text: string) => {
    navigator.clipboard?.writeText(text).then(() => Message.success(t('mp.copied')));
  };

  const card = { borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' };

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 900, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 4, flexWrap: 'wrap' }}>
        <span style={{ width: 8, height: 26, borderRadius: 4, background: 'linear-gradient(180deg,#07C160,#1AAD19)' }} />
        <span style={{ fontSize: 20, fontWeight: 700, color: 'var(--geo-text)' }}>{t('mp.title')}</span>
      </div>
      <div style={{ color: '#86909C', fontSize: 13, marginBottom: 16 }}>{t('mp.sub')}</div>

      {loading ? (
        <div style={{ padding: '60px 0', textAlign: 'center' }}><Spin size={24} /></div>
      ) : (
        <>
          {/* 小程序基础信息 */}
          <Card title={t('mp.basicTitle')} style={card} bordered={false}>
            <Alert type="info" style={{ marginBottom: 16 }} content={t('mp.basicHint')} />
            <Form layout="vertical" style={{ maxWidth: 560 }}>
              <Form.Item label={t('mp.status')}>
                {cfg?.configured ? (
                  <Tag color="green">{t('mp.configured')}（AppID {cfg?.appid_masked}）</Tag>
                ) : (
                  <Tag color="orange">{t('mp.notConfigured')}</Tag>
                )}
              </Form.Item>
              <Form.Item label={t('mp.appidLabel')}>
                <Input value={appid} onChange={setAppid} placeholder="wx 开头的小程序 AppID" autoComplete="off" />
              </Form.Item>
              <Form.Item label={t('mp.secretLabel')} extra={t('mp.secretExtra')}>
                <Input.Password value={secret} onChange={setSecret} placeholder={t('mp.secretPlaceholder')} autoComplete="new-password" />
              </Form.Item>
              <Form.Item label={t('mp.enableLabel')}>
                <Switch checked={enabled} onChange={setEnabled} />
              </Form.Item>
              <Button type="primary" icon={<IconSave />} loading={saving} onClick={save}>{t('mp.saveBtn')}</Button>
            </Form>
          </Card>

          {/* 对接参数 */}
          <Card
            title={t('mp.paramTitle')}
            style={{ ...card, marginTop: 16 }}
            bordered={false}
            extra={<Button size="small" icon={<IconRefresh />} onClick={load}>{t('mp.refresh')}</Button>}
          >
            <Alert type="warning" style={{ marginBottom: 16 }} content={t('mp.paramHint')} />
            <Form layout="vertical" style={{ maxWidth: 560 }}>
              <Form.Item label={t('mp.apiBaseLabel')} extra={t('mp.apiBaseExtra')}>
                <Space>
                  <Text code>{cfg?.api_base || '-'}</Text>
                  <Button size="mini" icon={<IconCopy />} onClick={() => copy(cfg?.api_base || '')} />
                </Space>
              </Form.Item>
              <Form.Item label={t('mp.apiKeyLabel')} extra={t('mp.apiKeyExtra')}>
                <Space>
                  <Text code>{cfg?.api_key_masked || '-'}</Text>
                  <Button size="mini" icon={<IconCopy />} onClick={() => copy(cfg?.api_key_masked || '')} />
                </Space>
              </Form.Item>
              <Button type="outline" status="warning" icon={<IconRefresh />} onClick={rotate}>
                {t('mp.rotateBtn')}
              </Button>
            </Form>
          </Card>

          {/* 对接说明 */}
          <Card title={t('mp.guideTitle')} style={{ ...card, marginTop: 16 }} bordered={false}>
            <div style={{ fontSize: 13, color: '#4e5969', lineHeight: 2 }}>
              {t('mp.guide1')}<br />
              {t('mp.guide2')}<br />
              {t('mp.guide3')}<br />
              {t('mp.guide4')}
            </div>
          </Card>
        </>
      )}
    </div>
  );
}
