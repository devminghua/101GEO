import { useEffect, useState } from 'react';
import { Card, Button, Form, Input, Message, Typography } from '@arco-design/web-react';
import { IconSave } from '@arco-design/web-react/icon';
import { api } from '../../api';

// 渠道品牌与客服设置：设置自己的品牌（名称/版权）与客服联系方式，
// 自动应用到旗下所有分站的客户端（分站自设品牌时优先分站）。
export default function ChannelProfile() {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [points, setPoints] = useState(0);
  const [form] = Form.useForm();

  useEffect(() => {
    api.channelProfile().then((d: any) => {
      setPoints(d?.points || 0);
      form.setFieldsValue({
        brand_name: d?.brand_name || '',
        copyright: d?.copyright || '',
        service_wechat: d?.service_wechat || '',
        service_phone: d?.service_phone || '',
      });
    }).catch(() => {}).finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const save = async () => {
    const v = await form.validate();
    setSaving(true);
    try {
      await api.channelSaveProfile(v);
      Message.success('已保存，旗下分站客户端立即生效');
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <Typography.Paragraph type="secondary" style={{ fontSize: 13, marginTop: 0 }}>
        设置你的品牌与客服：自动应用到旗下所有分站的客户端展示（分站自行设置品牌时以分站为准）。
      </Typography.Paragraph>
      <Card style={{ borderRadius: 12, maxWidth: 720, marginBottom: 16, background: 'linear-gradient(135deg, #4F46E5, #7B61FF)' }} bordered={false}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', color: '#fff' }}>
          <div>
            <div style={{ fontSize: 13, opacity: 0.85 }}>渠道点数余额（可拨付给旗下客户）</div>
            <div style={{ fontSize: 28, fontWeight: 800, marginTop: 4 }}>{points.toLocaleString('en-US')} 点</div>
          </div>
          <div style={{ fontSize: 12, opacity: 0.8, textAlign: 'right', maxWidth: 260 }}>
            余额不足请联系平台充值；给客户充值会从本余额扣除
          </div>
        </div>
      </Card>
      <Card title="品牌设置" style={{ borderRadius: 12, maxWidth: 720 }} bordered>
        <Form form={form} layout="vertical" disabled={loading}>
          <Form.Item label="品牌名称" field="brand_name" extra="旗下分站登录页、侧边栏显示的品牌名；留空走平台默认">
            <Input placeholder="如：XX 科技" />
          </Form.Item>
          <Form.Item label="版权文案" field="copyright" extra="显示在客户端侧边栏底部">
            <Input placeholder="如：Copyright © XX All Rights Reserved" />
          </Form.Item>
        </Form>
      </Card>
      <Card title="客服联系方式" style={{ borderRadius: 12, maxWidth: 720, marginTop: 16 }} bordered>
        <Form form={form} layout="vertical" disabled={loading}>
          <Form.Item label="客服微信" field="service_wechat" extra="显示在旗下分站客户端的客服悬浮入口">
            <Input placeholder="如：kefu888" />
          </Form.Item>
          <Form.Item label="客服电话" field="service_phone">
            <Input placeholder="如：400-000-0000" />
          </Form.Item>
        </Form>
      </Card>
      <Button type="primary" icon={<IconSave />} loading={saving} onClick={save} style={{ marginTop: 16 }}>
        保存设置
      </Button>
    </div>
  );
}
