import { useEffect, useState } from 'react';
import { Card, Checkbox, Form, Input, Button, Typography, Message, Modal } from '@arco-design/web-react';
import { IconUser, IconLock, IconMobile, IconEmail, IconIdcard } from '@arco-design/web-react/icon';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { api, setAuth } from '../api';

// 手机号校验（中国大陆 11 位）
const PHONE_RE = /^1[3-9]\d{9}$/;
// 邮箱校验
const EMAIL_RE = /^[\w.+-]+@[\w-]+(\.[\w-]+)+$/;
// 登录账号：字母/数字/下划线/连字符，3-32 位
const USERNAME_RE = /^[a-zA-Z0-9_-]{3,32}$/;

export default function Register({ onSuccess }: { onSuccess: () => void }) {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const refCode = (searchParams.get('ref') || '').trim(); // 邀请码（受邀注册）
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [sending, setSending] = useState(false);
  const [countdown, setCountdown] = useState(0);
  const [debugCode, setDebugCode] = useState(''); // 【Mock】联调回显验证码，接入真实发送后移除
  const [sysInfo, setSysInfo] = useState<any>(null);
  const [verifyMode, setVerifyMode] = useState<'sms' | 'email' | 'off'>('sms'); // 验证方式（总后台设置控制）
  // 网站注册安全协议（SaaS 端可编辑）
  const [agreement, setAgreement] = useState({ enabled: true, title: '网站注册安全协议', content: '', updated: '' });
  const [agreed, setAgreed] = useState(false);
  const [agreementVisible, setAgreementVisible] = useState(false);

  useEffect(() => {
    api.systemInfo().then((s: any) => setSysInfo(s || {})).catch(() => {});
    api
      .registerConfig()
      .then((c: any) => {
        setVerifyMode(c?.verify_mode || (c?.sms_required ? 'sms' : 'off'));
        setAgreement({
          enabled: c?.agreement_enabled !== false,
          title: (c?.agreement_title || '网站注册安全协议').trim(),
          content: c?.agreement_content || '',
          updated: c?.agreement_updated || '',
        });
      })
      .catch(() => {});
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
    if (verifyMode === 'email') {
      let email = '';
      try {
        await form.validate(['email']);
        email = (form.getFieldValue('email') || '').trim();
      } catch {
        return;
      }
      if (!EMAIL_RE.test(email)) {
        Message.error('请先填写正确的邮箱');
        return;
      }
      setSending(true);
      try {
        const data: any = await api.emailCode(email);
        setDebugCode(data?.debug_code || '');
        setCountdown(60);
        Message.success('验证码已发送至邮箱' + (data?.debug_code ? '（Mock：见下方提示）' : ''));
      } catch (e: any) {
        Message.error(e.message || '发送失败');
      } finally {
        setSending(false);
      }
      return;
    }
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
    if (agreement.enabled && !agreed) {
      Message.error(`请先阅读并勾选同意《${agreement.title}》`);
      return;
    }
    if (!USERNAME_RE.test((values.username || '').trim())) {
      Message.error('登录账号仅限 3-32 位字母/数字/下划线/连字符');
      return;
    }
    if (verifyMode === 'email') {
      if (!EMAIL_RE.test((values.email || '').trim())) {
        Message.error('邮箱格式不正确');
        return;
      }
      if (!values.code) {
        Message.error('请输入邮箱验证码');
        return;
      }
    } else if (!PHONE_RE.test((values.phone || '').trim())) {
      Message.error('手机号格式不正确');
      return;
    } else if (verifyMode === 'sms' && !values.code) {
      Message.error('请输入短信验证码');
      return;
    }
    setLoading(true);
    try {
      const data = await api.register({
        username: (values.username || '').trim(),
        phone: (values.phone || '').trim(),
        email: (values.email || '').trim(),
        code: (values.code || '').trim(),
        password: values.password,
        company_name: (values.company_name || '').trim(),
        ref: refCode,
        agreed,
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
            label="登录账号"
            field="username"
            extra="用于登录系统，全局唯一；手机号/邮箱仅用于接收验证码"
            rules={[
              { required: true, message: '请输入登录账号' },
              { match: USERNAME_RE, message: '3-32 位字母/数字/下划线/连字符' },
            ]}
          >
            <Input prefix={<IconIdcard />} placeholder="如：hongniang01" size="large" maxLength={32} autoComplete="off" />
          </Form.Item>

          {verifyMode === 'email' ? (
            <Form.Item
              label="邮箱"
              field="email"
              rules={[
                { required: true, message: '请输入邮箱' },
                { match: EMAIL_RE, message: '邮箱格式不正确' },
              ]}
            >
              <Input prefix={<IconEmail />} placeholder="用于登录与接收验证码" size="large" maxLength={64} />
            </Form.Item>
          ) : (
            <Form.Item
              label="手机号"
              field="phone"
              rules={[
                { required: true, message: '请输入手机号' },
                { match: PHONE_RE, message: '手机号格式不正确' },
              ]}
            >
              <Input prefix={<IconMobile />} placeholder="用于登录" size="large" maxLength={11} />
            </Form.Item>
          )}

          {verifyMode !== 'off' && (
            <Form.Item
              label={verifyMode === 'email' ? '邮箱验证码' : '短信验证码'}
              field="code"
              rules={[{ required: true, message: '请输入验证码' }]}
            >
              <div style={{ display: 'flex', gap: 8 }}>
                <Input prefix={<IconUser />} placeholder="6 位验证码" size="large" maxLength={6} style={{ flex: 1 }} />
                <Button size="large" loading={sending} disabled={countdown > 0} onClick={sendCode} style={{ width: 128 }}>
                  {countdown > 0 ? `${countdown}s 后重发` : '获取验证码'}
                </Button>
              </div>
            </Form.Item>
          )}
          {verifyMode !== 'off' && debugCode && (
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

          {agreement.enabled && (
            <div style={{ marginTop: -4, marginBottom: 12 }}>
              <Checkbox checked={agreed} onChange={setAgreed}>
                <span style={{ fontSize: 13 }}>我已阅读并同意</span>
                <a
                  onClick={(e) => {
                    e.preventDefault();
                    setAgreementVisible(true);
                  }}
                  href="#"
                  style={{ fontSize: 13, color: '#4F46E5' }}
                >
                  《{agreement.title}》
                </a>
              </Checkbox>
            </div>
          )}

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

      <Modal
        title={agreement.title}
        visible={agreementVisible}
        onCancel={() => setAgreementVisible(false)}
        footer={
          <div style={{ textAlign: 'right' }}>
            <Button onClick={() => setAgreementVisible(false)}>关闭</Button>
            <Button
              type="primary"
              onClick={() => {
                setAgreed(true);
                setAgreementVisible(false);
              }}
            >
              我已阅读并同意
            </Button>
          </div>
        }
        autoFocus={false}
        style={{ width: 620 }}
      >
        <div style={{ maxHeight: '52vh', overflow: 'auto', whiteSpace: 'pre-wrap', fontSize: 13, lineHeight: 1.9, color: 'var(--color-text-1)' }}>
          {agreement.content || '协议内容暂未配置。'}
        </div>
        {agreement.updated && (
          <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 10 }}>
            最近更新日期：{agreement.updated}
          </Typography.Text>
        )}
      </Modal>
    </div>
  );
}
