import { useState, useEffect, useRef } from 'react';
import { Card, Form, Input, Button, Typography, Message, Tag, Modal } from '@arco-design/web-react';
import { IconUser, IconLock, IconRight, IconCheck } from '@arco-design/web-react/icon';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, setAuth } from '../api';

// 滑动解锁尺寸（与后端 slideRange 保持一致）
const TRACK_W = 320; // 轨道宽
const HANDLE_W = 48; // 滑块宽
const SLIDE_MAX = TRACK_W - HANDLE_W; // 272
const SLIDE_TOL = 4; // 容差

// 账号有效期提示：展示在密码输入框下方
interface ExpiryHint {
  kind: 'none' | 'loading' | 'limited' | 'unlimited' | 'expired';
  text: string;
}

interface CaptchaData {
  id: string;
}

export default function Login({ onSuccess }: { onSuccess: () => void }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [expiry, setExpiry] = useState<ExpiryHint>({ kind: 'none', text: '' });
  const [form] = Form.useForm();
  // 系统品牌（公开读取）：有配置则替换默认名称与 Logo
  const [sysInfo, setSysInfo] = useState<any>(null);

  // 滑动解锁
  const [cap, setCap] = useState<CaptchaData | null>(null);
  const [slideX, setSlideX] = useState(0);
  const [verified, setVerified] = useState(false);
  const [dragging, setDragging] = useState(false);
  const slideXRef = useRef(0);
  const dragRef = useRef<{ startX: number; startLeft: number } | null>(null);
  const trackRef = useRef<number[]>([]); // 拖动轨迹采样（x 坐标序列），随登录提交做人机校验

  // 锁定倒计时
  const [lock, setLock] = useState<{ seconds: number; level: number } | null>(null);
  const [remain, setRemain] = useState(0);

  useEffect(() => {
    api.systemInfo().then((s: any) => setSysInfo(s || {})).catch(() => {});
    loadCaptcha();
  }, []);

  // 锁定倒计时：每秒递减，归零自动解锁
  useEffect(() => {
    if (!lock) return;
    setRemain(lock.seconds);
    const t = setInterval(() => {
      setRemain((r) => {
        if (r <= 1) {
          clearInterval(t);
          setLock(null);
          return 0;
        }
        return r - 1;
      });
    }, 1000);
    return () => clearInterval(t);
  }, [lock]);

  // system_name 若为品牌默认变体（Link GEO/LinkGEO/LinkGeo）视为未自定义，走默认 LinkGeo 配色
  const sysName = ((sysInfo && sysInfo.system_name) || '').trim().replace(/^link\s*geo$/i, '');
  const sysLogo = (sysInfo && sysInfo.system_logo) || '';

  const loadCaptcha = async () => {
    try {
      const d: any = await api.getCaptcha();
      setCap(d);
      slideXRef.current = 0;
      setSlideX(0);
      setVerified(false);
      trackRef.current = [];
    } catch {
      /* 加载失败不阻塞页面，登录时再提示 */
    }
  };

  // ---- 滑块拖动 ----
  const startDrag = (clientX: number) => {
    if (verified) return; // 已验证不再重复拖动
    dragRef.current = { startX: clientX, startLeft: slideXRef.current };
    trackRef.current = [slideXRef.current]; // 起点
    setDragging(true);
  };
  const moveDrag = (clientX: number) => {
    if (!dragRef.current) return;
    const dx = clientX - dragRef.current.startX;
    let nx = dragRef.current.startLeft + dx;
    nx = Math.max(0, Math.min(SLIDE_MAX, nx));
    slideXRef.current = nx;
    setSlideX(nx);
    const t = trackRef.current;
    if (t[t.length - 1] !== nx) t.push(nx); // 位置变化才采样，避免冗余
  };
  const endDrag = () => {
    dragRef.current = null;
    setDragging(false);
    // 拖到最右端（容差内）视为验证通过；否则回弹重来
    if (slideXRef.current >= SLIDE_MAX - SLIDE_TOL) {
      slideXRef.current = SLIDE_MAX;
      setSlideX(SLIDE_MAX);
      setVerified(true);
      const t = trackRef.current;
      if (t[t.length - 1] !== SLIDE_MAX) t.push(SLIDE_MAX); // 补终点
    } else {
      slideXRef.current = 0;
      setSlideX(0);
      setVerified(false);
      trackRef.current = [];
    }
  };

  // 拖动过程中在 document 上监听，避免鼠标移出滑块后中断
  useEffect(() => {
    if (!dragging) return;
    const onMove = (e: MouseEvent) => moveDrag(e.clientX);
    const onUp = () => endDrag();
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
    return () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };
  }, [dragging]);

  // 账号失焦后查询服务有效期，在密码框旁展示剩余天数
  const queryExpiry = async (acct: string) => {
    const name = (acct || '').trim();
    if (!name) {
      setExpiry({ kind: 'none', text: '' });
      return;
    }
    setExpiry({ kind: 'loading', text: '查询中…' });
    try {
      const data = await api.authExpiry(name);
      if (!data) {
        setExpiry({ kind: 'none', text: '' });
        return;
      }
      const d: any = data;
      if (d.open_months && d.open_months > 0) {
        if ((d.remain_days ?? -1) <= 0) {
          setExpiry({ kind: 'expired', text: '' });
        } else {
          setExpiry({ kind: 'limited', text: `该账号服务剩余 ${d.remain_days} 天有效` });
        }
      } else {
        setExpiry({ kind: 'unlimited', text: '该账号为不限有效期账号' });
      }
    } catch {
      setExpiry({ kind: 'none', text: '' }); // 查询失败不阻塞登录
    }
  };

  const submit = async () => {
    const values = await form.validate();
    if (!cap) {
      Message.warning('验证加载中，请稍候');
      return;
    }
    if (!verified || slideX < SLIDE_MAX - SLIDE_TOL) {
      Message.warning('请将滑块拖到最右端完成验证');
      return;
    }
    setLoading(true);
    try {
      const data = await api.login({
        username: values.username,
        password: values.password,
        captcha_id: cap.id,
        slide_x: Math.round(slideX),
        track: trackRef.current,
      });
      setAuth(data);
      Message.success('登录成功');
      onSuccess();
    } catch (e: any) {
      if (e.locked) {
        setLock({ seconds: e.lock_seconds || 300, level: e.lock_level || 1 });
        loadCaptcha(); // 锁定后刷新验证，解锁后需重新滑动
        return;
      }
      Message.error(e.message);
      // 无论滑块失败还是密码错误，都刷新验证，防止同一凭证被重放
      loadCaptcha();
    } finally {
      setLoading(false);
    }
  };

  const fmtTime = (s: number) => {
    const m = Math.floor(s / 60);
    const sec = s % 60;
    return m > 0 ? `${m} 分 ${sec} 秒` : `${sec} 秒`;
  };

  const expiryTag = () =>
    expiry.kind === 'limited' ? (
      <Tag color="green" style={{ fontSize: 12 }}>
        {expiry.text}
      </Tag>
    ) : expiry.kind === 'expired' ? (
      <Tag color="red" style={{ fontSize: 12 }}>
        {expiry.text}
      </Tag>
    ) : expiry.kind === 'unlimited' ? (
      <Tag color="arcoblue" style={{ fontSize: 12 }}>
        不限有效期
      </Tag>
    ) : null;

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
      <Card style={{ width: 420, borderRadius: 16, boxShadow: '0 12px 40px rgba(22,93,255,0.12)' }}>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          {sysLogo ? (
            <img
              src={sysLogo}
              alt="logo"
              style={{ height: 56, maxWidth: 200, objectFit: 'contain', margin: '0 auto 10px', display: 'block' }}
            />
          ) : sysName ? (
            <Typography.Title heading={4} style={{ marginBottom: 4 }}>
              {sysName}
            </Typography.Title>
          ) : (
            <div
              style={{
                fontSize: 34,
                fontWeight: 800,
                letterSpacing: -0.5,
                lineHeight: 1.1,
                marginBottom: 6,
              }}
            >
              <span style={{ color: 'var(--geo-text)' }}>Link</span>
              <span style={{ color: '#4F46E5' }}>Geo</span>
            </div>
          )}
          <div style={{ fontSize: 13, color: '#86909c', letterSpacing: 1 }}>
            生成式引擎优化平台
          </div>
        </div>
        <Form form={form} layout="vertical" initialValues={{ username: '', password: '' }} autoComplete="off">
          <Form.Item
            label={t('login.account')}
            field="username"
            rules={[{ required: true, message: t('login.account') }]}
          >
            <Input
              prefix={<IconUser />}
              placeholder={t('login.account')}
              size="large"
              onBlur={(e) => queryExpiry(e.target.value)}
            />
          </Form.Item>
          <Form.Item
            label={t('login.password')}
            field="password"
            rules={[{ required: true, message: t('login.password') }]}
            extra={
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                {expiry.kind === 'loading' && (
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    查询服务有效期…
                  </Typography.Text>
                )}
                {expiryTag()}
              </span>
            }
          >
            <Input.Password prefix={<IconLock />} placeholder={t('login.password')} size="large" />
          </Form.Item>

          {/* 滑动解锁（无图像，拖到最右端即通过） */}
          {cap ? (
            <div style={{ width: TRACK_W, margin: '0 auto 4px' }}>
              <div
                style={{
                  position: 'relative',
                  width: TRACK_W,
                  height: 44,
                  background: 'var(--color-fill-2)',
                  borderRadius: 22,
                  overflow: 'hidden',
                  userSelect: 'none',
                  touchAction: 'none',
                }}
              >
                {/* 已拖动轨道高亮 */}
                <div
                  style={{
                    position: 'absolute',
                    left: 0,
                    top: 0,
                    width: slideX + HANDLE_W,
                    height: 44,
                    background: verified ? 'var(--geo-tint-green)' : 'var(--geo-tint-blue)',
                    borderRadius: 22,
                    transition: dragging ? 'none' : 'width .2s',
                  }}
                />
                <div
                  style={{
                    position: 'absolute',
                    left: 0,
                    top: '50%',
                    transform: 'translateY(-50%)',
                    width: '100%',
                    textAlign: 'center',
                    color: verified ? '#00b42a' : '#86909c',
                    fontSize: 13,
                    pointerEvents: 'none',
                  }}
                >
                  {verified ? '✓ ' + t('login.captchaHint') : t('login.captchaHint')}
                </div>
                <div
                  onMouseDown={(e) => { e.preventDefault(); startDrag(e.clientX); }}
                  onTouchStart={(e) => startDrag(e.touches[0].clientX)}
                  onTouchMove={(e) => moveDrag(e.touches[0].clientX)}
                  onTouchEnd={() => endDrag()}
                  style={{
                    position: 'absolute',
                    left: slideX,
                    top: 0,
                    width: HANDLE_W,
                    height: 44,
                    background: verified ? '#00b42a' : '#165dff',
                    borderRadius: 22,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    cursor: verified ? 'default' : 'grab',
                    boxShadow: '0 2px 6px rgba(22,93,255,0.35)',
                  }}
                >
                  {verified ? (
                    <IconCheck style={{ color: '#fff', fontSize: 18 }} />
                  ) : (
                    <IconRight style={{ color: '#fff', fontSize: 18 }} />
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div style={{ width: TRACK_W, height: 44, margin: '0 auto 4px', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#86909c' }}>
              验证加载中…
            </div>
          )}

          <Button
            type="primary"
            long
            size="large"
            loading={loading}
            disabled={!!lock}
            onClick={submit}
            style={{ marginTop: 12 }}
          >
            {lock ? t('login.loginFailed') : t('login.login')}
          </Button>
        </Form>

        <div style={{ textAlign: 'center', marginTop: 16 }}>
          <Typography.Text type="secondary" style={{ fontSize: 13 }}>
            {t('login.noAccount')}
          </Typography.Text>{' '}
          <Button type="text" size="small" onClick={() => navigate('/register')} style={{ padding: 0 }}>
            {t('login.registerNow')}
          </Button>
        </div>
      </Card>

      {/* 版本号（每次更新记一次版本号） */}
      {sysInfo?.version && (
        <div style={{ textAlign: 'center', marginTop: 14, fontSize: 12, color: '#86909c' }}>
          v{sysInfo.version}
        </div>
      )}

      {/* 锁定倒计时弹窗 */}
      <Modal
        visible={!!lock}
        footer={null}
        closable={false}
        maskClosable={false}
        title={lock && lock.level > 1 ? '账号已锁定' : '账号已锁定'}
      >
        <div style={{ textAlign: 'center', padding: '12px 0 4px' }}>
          <div style={{ fontSize: 15, marginBottom: 16, color: 'var(--geo-text)' }}>
            密码连续输错次数过多，为保障账号安全已临时锁定
            {lock && lock.level > 1 ? ' 1 小时' : ' 5 分钟'}
          </div>
          <div style={{ fontSize: 32, color: '#f53f3f', fontWeight: 600, fontVariantNumeric: 'tabular-nums' }}>
            {fmtTime(remain)}
          </div>
          <div style={{ color: '#86909c', marginTop: 12 }}>倒计时结束后可重新尝试登录</div>
        </div>
      </Modal>
    </div>
  );
}
