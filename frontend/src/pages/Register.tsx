import { useEffect, useState } from 'react';
import { Card, Form, Input, Button, Typography, Message } from '@arco-design/web-react';
import { IconUser, IconLock, IconMobile } from '@arco-design/web-react/icon';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { api, setAuth } from '../api';

// 手机号校验（中国大陆 11 位）
const PHONE_RE = /^1[3-9]\d{9}$/;

export default function Register({ onSuccess }: { onSuccess: () => void }) {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const refCode = (searchParams.get('ref') || '').trim(); // 邀请码（受邀注册）
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [sending, setSending] = useState(false);
  const [countdown, setCountdown] = useState(0);
  const [debugCode, setDebugCode] = useState(''); // 【Mock】联调回显验证码，接入真实短信后移除
  const [sysInfo, setSysInfo] = useState<any>(null);
  const [smsRequired, setSmsRequired] = useState(true); // 短信验证开关（总后台短信设置控制）

  useEffect(() => {
    api.systemInfo().then((s: any) => setSysInfo(s || {})).catch(() => {});
    api.registerConfig().then((c: any) => setSmsRequired(c?.sms_required ?? true)).catch(() => {});
  }, []);

  // 获取验证码 60s 倒计时
  useEffect(() => {
    if (countdown <= 0) return;
    const t = setInterval(() => {
      setCountdown((c) => {
        if (c <= 1) {
          clearInterval(t);
          return 0;
        }
        return c - 1;
      });
    }, 1000);
    return () => clearInterval(t);
  }, [countdown]);

  // system_name 若为品牌默认变体（Link GEO/LinkGEO/LinkGeo）视为未自定义，走默认 LinkGeo 配色
  const sysName = ((sysInfo && sysInfo.system_name) || '').trim().replace(/^link\s*geo$/i, '');
  const sysLogo = (sysInfo && sysInfo.system_logo) || '';

  const sendCode = async () => {
    let phone = '';
    try {
      await form.validate(['phone']);
      phone = (form.getFieldValue('phone') || '').trim();
    } catch {
      return;
    }
    if (!PHONE_RE.test(phone)) {
      Message.error('请先填写正确的手机号');
      return;
    }
    setSending(true);
    try {
      const data: any = await api.smsSend(phone);
      setDebugCode(data?.debug_code || '');
      setCountdown(60);
      Message.success('验证码已发送' + (data?.debug_code ? '（Mock：见下方提示）' : ''));
    } catch (e: any) {
      Message.error(e.message || '发送失败');
    } finally {
      setSending(false);
    }
  };

  const submit = async () => {
    const values = await form.validate();
    if (!PHONE_RE.test((values.phone || '').trim())) {
      Message.error('手机号格式不正确');
      return;
    }
    if (smsRequired && !values.code) {
      Message.error('请输入短信验证码');
      return;
    }
    setLoading(true);
    try {
      const data = await api.register({
        phone: (values.phone || '').trim(),
        code: smsRequired ? (values.code || '').trim() : '',
        password: values.password,
        company_name: (values.company_name || '').trim(),
        ref: refCode,
      });
      setAuth(data);
      Message.success('注册成功，已自动开通试用');
      onSuccess();
    } catch (e: any) {
      Message.error(e.message || '注册失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'linear-gradient(135deg,var(--geo-login-a) 0%,var(--geo-login-b) 60%,var(--geo-login-c) 100%)',
      }}
    >
      <Card style={{ width: 440, borderRadius: 16, boxShadow: '0 12px 40px rgba(22,93,255,0.12)' }}>
        <div style={{ textAlign: 'center', marginBottom: 20 }}>
          {sysLogo ? (
            <img
              src={sysLogo}
              alt="logo"
              style={{ height: 48, maxWidth: 180, objectFit: 'contain', margin: '0 auto 10px', display: 'block' }}
            />
          ) : sysName ? (
            <Typography.Title heading={4} style={{ marginBottom: 4 }}>
              注册 {sysName}
            </Typography.Title>
          ) : (
            <>
              <div
                style={{
                  fontSize: 30,
                  fontWeight: 800,
                  letterSpacing: -0.5,
                  lineHeight: 1.1,
                  marginBottom: 4,
                }}
              >
                <span style={{ color: 'var(--geo-text)' }}>Link</span>
                <span style={{ color: '#4F46E5' }}>Geo</span>
              </div>
              <div style={{ fontSize: 12, color: '#86909c', letterSpacing: 1, marginBottom: 10 }}>
                生成式引擎优化平台
              </div>
            </>
          )}
          <Typography.Text type="secondary" style={{ fontSize: 13 }}>
            注册即自动开通 7 天免费试用
          </Typography.Text>
        </div>

        <Form form={form} layout="vertical" autoComplete="off">
          <Form.Item
            label="手机号"
            field="phone"
            rules={[
              { required: true, message: '请输入手机号' },
              { match: PHONE_RE, message: '手机号格式不正确' },
            ]}
          >
            <Input prefix={<IconMobile />} placeholder="用于登录与接收验证码" size="large" maxLength={11} />
          </Form.Item>

          {smsRequired && (
            <Form.Item
              label="短信验证码"
              field="code"
              rules={[{ required: true, message: '请输入短信验证码' }]}
            >
              <div style={{ display: 'flex', gap: 8 }}>
                <Input prefix={<IconUser />} placeholder="6 位验证码" size="large" maxLength={6} style={{ flex: 1 }} />
                <Button size="large" loading={sending} disabled={countdown > 0} onClick={sendCode} style={{ width: 128 }}>
                  {countdown > 0 ? `${countdown}s 后重发` : '获取验证码'}
                </Button>
              </div>
            </Form.Item>
          )}
          {smsRequired && debugCode && (
            <Typography.Text type="warning" style={{ fontSize: 12 }}>
              【联调 Mock】本次验证码：{debugCode}
            </Typography.Text>
          )}

          <Form.Item
            label="登录密码"
            field="password"
            rules={[
              { required: true, message: '请输入密码' },
              { minLength: 8, message: '密码至少 8 位' },
              { match: /^(?=.*[A-Za-z])(?=.*\d).+$/, message: '密码须同时包含字母和数字' },
            ]}
          >
            <Input.Password prefix={<IconLock />} placeholder="至少 8 位，含字母和数字" size="large" />
          </Form.Item>

          <Form.Item
            label="确认密码"
            field="confirm_password"
            rules={[
              { required: true, message: '请再次输入密码' },
              {
                validator: (value, callback) => {
                  if (value && value !== form.getFieldValue('password')) {
                    callback('两次输入的密码不一致');
                  } else {
                    callback();
                  }
                },
              },
            ]}
          >
            <Input.Password prefix={<IconLock />} placeholder="再次输入密码" size="large" />
          </Form.Item>

          <Form.Item
            label="公司/机构名称"
            field="company_name"
            rules={[{ required: true, message: '请填写公司/机构名称' }]}
          >
            <Input prefix={<IconUser />} placeholder="将作为您的分站名称" size="large" maxLength={64} />
          </Form.Item>

          <Button type="primary" long size="large" loading={loading} onClick={submit} style={{ marginTop: 4 }}>
            注 册
          </Button>
        </Form>

        <div style={{ textAlign: 'center', marginTop: 16 }}>
          <Typography.Text type="secondary" style={{ fontSize: 13 }}>
            已有账号？
          </Typography.Text>{' '}
          <Button type="text" size="small" onClick={() => navigate('/login')} style={{ padding: 0 }}>
            返回登录
          </Button>
        </div>
      </Card>
    </div>
  );
}
