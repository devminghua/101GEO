import { useState, useEffect } from 'react';
import { Card, Form, Input, Switch, Radio, Button, Message, Typography, Alert, Divider, InputNumber } from '@arco-design/web-react';
import { api } from '../../api';

// 注册验证设置（总后台 super）：验证方式（短信 / 邮箱 / 关闭）+ 短信服务商 + SMTP 邮箱配置 + 网站注册安全协议。
// 配置存全局（tenant_id=0），所有分站的自主注册共用同一套。
export default function SmsConfig() {
  const [loading, setLoading] = useState(false);
  const [provider, setProvider] = useState('mock');
  const [form] = Form.useForm();
  const verifyMode = Form.useWatch('verify_mode', form) ?? 'sms';
  const agreementEnabled = Form.useWatch('agreement_enabled', form) ?? true;

  const load = async () => {
    try {
      const c = await api.smsGetConfig();
      const p = c.provider || 'mock';
      setProvider(p);
      form.setFieldsValue({
        verify_mode: c.verify_mode || (c.sms_required ? 'sms' : 'off'),
        provider: p,
        access_key_id: c.access_key_id,
        access_key_secret: c.access_key_secret, // 脱敏值，留空/含 * 表示保存时不修改
        sign_name: c.sign_name,
        template_code: c.template_code,
        smtp_host: c.smtp_host,
        smtp_port: c.smtp_port || 465,
        smtp_user: c.smtp_user,
        smtp_pass: c.smtp_pass, // 脱敏值，留空/含 * 表示保存时不修改
        smtp_from: c.smtp_from,
        agreement_enabled: c.agreement_enabled !== false,
        agreement_title: c.agreement_title,
        agreement_content: c.agreement_content,
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
        verify_mode: v.verify_mode,
        sms_required: v.verify_mode === 'sms',
        provider: v.provider,
        access_key_id: v.access_key_id || '',
        access_key_secret: v.access_key_secret || '',
        sign_name: v.sign_name || '',
        template_code: v.template_code || '',
        smtp_host: v.smtp_host || '',
        smtp_port: Number(v.smtp_port) || 465,
        smtp_user: v.smtp_user || '',
        smtp_pass: v.smtp_pass || '',
        smtp_from: v.smtp_from || '',
        agreement_enabled: v.agreement_enabled !== false,
        agreement_title: v.agreement_title || '',
        agreement_content: v.agreement_content || '',
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
    <Card title="注册验证设置" style={{ borderRadius: 12, maxWidth: 760 }}>
      <Alert
        type="info"
        style={{ marginBottom: 20 }}
        content="以下配置对全部分站的自主注册统一生效。验证方式三选一：短信验证码 / 邮箱验证码 / 全部关闭（直接注册）；页面底部可维护注册页的《网站注册安全协议》。"
      />
      <Form form={form} layout="vertical" initialValues={{ verify_mode: 'sms', provider: 'mock' }}>
        <Form.Item label="注册验证方式" field="verify_mode">
          <Radio.Group>
            <Radio value="sms">短信验证码</Radio>
            <Radio value="email">邮箱验证码</Radio>
            <Radio value="off">全部关闭（直接注册）</Radio>
          </Radio.Group>
        </Form.Item>
        <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: -12, marginBottom: 16 }}>
          短信：注册需手机号收验证码；邮箱：注册用邮箱收验证码（账号即邮箱）；关闭：填手机号 + 密码即可直接注册。
        </div>

        {verifyMode === 'sms' && (
          <>
            <Divider />
            <Typography.Title heading={6} style={{ marginTop: 4 }}>短信服务商</Typography.Title>
            <Form.Item label="服务商" field="provider">
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
          </>
        )}

        {verifyMode === 'email' && (
          <>
            <Divider />
            <Typography.Title heading={6} style={{ marginTop: 4 }}>SMTP 邮箱配置（发送验证码）</Typography.Title>
            <Form.Item label="SMTP 服务器" field="smtp_host" rules={[{ required: true, message: '请输入 SMTP 服务器地址' }]}>
              <Input placeholder="如：smtp.qq.com / smtp.exmail.qq.com / smtp.163.com" />
            </Form.Item>
            <Form.Item label="端口" field="smtp_port">
              <InputNumber min={1} max={65535} style={{ width: 160 }} />
              <Typography.Text type="secondary" style={{ fontSize: 12, marginLeft: 10 }}>
                465 = SSL（推荐）；587 = STARTTLS
              </Typography.Text>
            </Form.Item>
            <Form.Item label="发件账号" field="smtp_user" rules={[{ required: true, message: '请输入发件邮箱账号' }]}>
              <Input placeholder="如：service@yourdomain.com" />
            </Form.Item>
            <Form.Item
              label="授权码 / 密码"
              field="smtp_pass"
              extra="QQ/163 邮箱需使用「授权码」而非登录密码（邮箱设置 → 开启 SMTP 服务获取）"
            >
              <Input.Password placeholder="留空或保持 **** 表示不修改" />
            </Form.Item>
            <Form.Item label="发件人地址" field="smtp_from" extra="一般与发件账号一致">
              <Input placeholder="如：LinkGeo <service@yourdomain.com>" />
            </Form.Item>
          </>
        )}

        <Divider />
        <Typography.Title heading={6} style={{ marginTop: 4 }}>网站注册安全协议</Typography.Title>
        <Form.Item
          label="注册时需勾选同意"
          field="agreement_enabled"
          triggerPropName="checked"
        >
          <Switch checkedText="开启" uncheckedText="关闭" />
        </Form.Item>
        <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: -12, marginBottom: 16 }}>
          开启后，注册页展示协议勾选框，未勾选不允许提交注册；关闭则注册页不展示该勾选项。协议内容对全部分站统一生效。
        </div>

        {agreementEnabled && (
          <>
            <Form.Item label="协议标题" field="agreement_title" rules={[{ required: true, message: '请输入协议标题' }]}>
              <Input placeholder="如：网站注册安全协议" maxLength={64} />
            </Form.Item>
            <Form.Item
              label="协议正文"
              field="agreement_content"
              rules={[{ required: true, message: '请输入协议正文' }]}
              extra="支持换行，按纯文本展示；注册页点协议标题可弹窗查看全文"
            >
              <Input.TextArea autoSize={{ minRows: 12, maxRows: 24 }} placeholder="请输入注册安全协议全文…" />
            </Form.Item>
          </>
        )}

        {/* 保存按钮限宽到容器 1/3（Arco Form 为 flex-column，默认会被拉伸撑满整行） */}
        <Button
          type="primary"
          loading={loading}
          onClick={save}
          style={{ width: '33%', minWidth: 150, alignSelf: 'flex-start' }}
        >
          保存
        </Button>
      </Form>
      <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 12 }}>
        说明：验证方式切换后立即生效；未配置 SMTP 时邮箱验证码为 Mock 模式（验证码回显在注册页，仅联调用），配置完整后自动切换为真实发送。
      </Typography.Text>
    </Card>
  );
}
