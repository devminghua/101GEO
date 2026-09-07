import { useState, useEffect } from 'react';
import { Card, Form, Input, Switch, Radio, Button, Message, Typography, Alert, Divider } from '@arco-design/web-react';
import { api } from '../../api';

// 短信设置（总后台 super）：注册短信验证开关 + 短信服务商配置。
// 配置存全局（tenant_id=0），所有分站的自主注册共用同一套短信服务。
export default function SmsConfig() {
  const [loading, setLoading] = useState(false);
  const [provider, setProvider] = useState('mock');
  const [form] = Form.useForm();

  const load = async () => {
    try {
      const c = await api.smsGetConfig();
      const p = c.provider || 'mock';
      setProvider(p);
      form.setFieldsValue({
        sms_required: c.sms_required,
        provider: p,
        access_key_id: c.access_key_id,
        access_key_secret: c.access_key_secret, // 脱敏值，留空/含 * 表示保存时不修改
        sign_name: c.sign_name,
        template_code: c.template_code,
      });
    } catch {
      /* 忽略 */
    }
  };

  useEffect(() => { load(); }, []);

  const save = async () => {
    const v = await form.validate();
    setLoading(true);
    try {
      await api.smsSaveConfig({
        sms_required: v.sms_required,
        provider: v.provider,
        access_key_id: v.access_key_id || '',
        access_key_secret: v.access_key_secret || '',
        sign_name: v.sign_name || '',
        template_code: v.template_code || '',
      });
      Message.success('已保存');
      load();
    } catch (e: any) {
      Message.error(e.message || '保存失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card title="短信设置" style={{ borderRadius: 12, maxWidth: 760 }}>
      <Alert
        type="info"
        style={{ marginBottom: 20 }}
        content="以下配置对全部分站的自主注册统一生效。当前短信为 Mock 模式（不真发短信，验证码打印到服务日志，注册页回显 debug_code），接入阿里云短信后自动切换为真实发送。"
      />
      <Form form={form} layout="vertical" initialValues={{ sms_required: true, provider: 'mock' }}>
        <Form.Item label="注册短信验证" field="sms_required" triggerPropName="checked">
          <Switch checkedText="开启" uncheckedText="关闭" />
        </Form.Item>
        <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: -12, marginBottom: 16 }}>
          开启：注册需先手机号收验证码；关闭：客户可直接用手机号 + 密码注册，无需短信验证。
        </div>

        <Divider />

        <Form.Item label="短信服务商" field="provider">
          <Radio.Group onChange={(v) => setProvider(v)}>
            <Radio value="mock">Mock（联调用，不真发短信）</Radio>
            <Radio value="aliyun">阿里云短信</Radio>
          </Radio.Group>
        </Form.Item>

        {provider === 'aliyun' && (
          <>
            <Form.Item label="AccessKey ID" field="access_key_id">
              <Input placeholder="阿里云 AccessKey ID" />
            </Form.Item>
            <Form.Item label="AccessKey Secret" field="access_key_secret">
              <Input.Password placeholder="留空或保持 **** 表示不修改" />
            </Form.Item>
            <Form.Item label="短信签名" field="sign_name">
              <Input placeholder="如：LinkGeo" />
            </Form.Item>
            <Form.Item label="模板 CODE" field="template_code">
              <Input placeholder="阿里云短信模板 CODE" />
            </Form.Item>
          </>
        )}

        <Button type="primary" loading={loading} onClick={save}>
          保存
        </Button>
      </Form>
      <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 12 }}>
        说明：关闭「注册短信验证」后，注册页将不再显示验证码输入框，客户填手机号 + 密码 + 公司名即可直接注册。
      </Typography.Text>
    </Card>
  );
}
