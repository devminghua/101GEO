import { useEffect, useState } from 'react';
import { Card, Form, Switch, Input, InputNumber, Button, Divider, Message, Grid, Alert, Space } from '@arco-design/web-react';
import { api } from '../../api';
const FormItem = Form.Item;
const Row = Grid.Row;
const Col = Grid.Col;

// 支付配置：密钥类字段回显为 ****** 占位符，留空=保留原值（仅首次填写新值才覆盖）。
// 与后端 services/pay 的 Key 常量一一对应。
const FIELDS = [
  // 通用
  { key: 'point_price_fen', label: '点卡单价（分/点）', type: 'number', tip: '每 1 点对应的价格，单位分，默认 100 分 = 1 元', group: 'common', def: 100 },
  { key: 'extend_price_fen_month', label: '续费月单价（分/月）', type: 'number', tip: '客户续费 1 个月的价格，单位分，默认 9800 分 = 98 元/月。续费流水金额 = 月单价 × 月数', group: 'common', def: 9800 },
  // 微信
  { key: 'wechat_pay_enabled', label: '启用微信支付', type: 'switch', group: 'wechat' },
  { key: 'wechat_pay_mch_id', label: '微信商户号 mch_id', type: 'text', group: 'wechat' },
  { key: 'wechat_pay_app_id', label: '公众号/小程序 AppID', type: 'text', group: 'wechat' },
  { key: 'wechat_pay_serial_no', label: '商户证书序列号 serial_no', type: 'text', group: 'wechat' },
  { key: 'wechat_pay_api_v3_key', label: 'APIv3 密钥', type: 'secret', group: 'wechat', tip: '32 位，填写后加密存储，回显不显示明文' },
  { key: 'wechat_pay_private_key', label: '商户 API 私钥（PKCS8 PEM）', type: 'textarea', group: 'wechat', tip: '-----BEGIN PRIVATE KEY----- 开头，填写后加密存储' },
  { key: 'wechat_pay_notify_base_url', label: '微信回调基础地址', type: 'text', group: 'wechat', tip: '形如 https://your-domain.com，实际回调为 {base}/notify/wechat' },
  // 支付宝
  { key: 'alipay_pay_enabled', label: '启用支付宝支付', type: 'switch', group: 'alipay' },
  { key: 'alipay_pay_app_id', label: '支付宝应用 AppID', type: 'text', group: 'alipay' },
  { key: 'alipay_pay_private_key', label: '应用私钥（RSA2）', type: 'textarea', group: 'alipay', tip: '填写后加密存储，回显不显示明文' },
  { key: 'alipay_pay_public_key', label: '支付宝公钥', type: 'textarea', group: 'alipay', tip: '填写后加密存储，回显不显示明文' },
  { key: 'alipay_pay_notify_base_url', label: '支付宝回调基础地址', type: 'text', group: 'alipay', tip: '形如 https://your-domain.com，实际回调为 {base}/notify/alipay' },
  // 告警通知（企微/钉钉群机器人）
  { key: 'notify_enabled', label: '启用每日告警推送', type: 'switch', group: 'notify' },
  { key: 'notify_wecom_webhook', label: '企微群机器人 Webhook', type: 'textarea', group: 'notify', tip: '企微群 → 添加群机器人 → 复制 Webhook 地址；与钉钉可只配其一' },
  { key: 'notify_dingtalk_webhook', label: '钉钉群机器人 Webhook', type: 'textarea', group: 'notify', tip: '钉钉群 → 智能群助手 → 添加自定义机器人（安全设置选「自定义关键词」填 LinkGeo）→ 复制 Webhook' },
  { key: 'notify_expiry_days', label: '到期预警天数', type: 'number', group: 'notify', tip: '服务有效期多少天内到期即预警，默认 30 天', def: 30 },
  { key: 'notify_points_floor', label: '点数不足阈值', type: 'number', group: 'notify', tip: '分站点数余额低于该值即预警，默认 20 点', def: 20 },
  { key: 'notify_push_hour', label: '每日推送时间（点）', type: 'number', group: 'notify', tip: '每天几点推送告警汇总（0-23），默认 9 点', def: 9 },
];

const SECRET_KEYS = new Set([
  'wechat_pay_api_v3_key',
  'wechat_pay_private_key',
  'alipay_pay_private_key',
  'alipay_pay_public_key',
]);

const MASK = '******';

export default function PayConfig() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [hasSecret, setHasSecret] = useState(false);
  const [testing, setTesting] = useState(false);
  const [preview, setPreview] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    try {
      const cfg = await api.payGetConfig();
      const init: Record<string, any> = {};
      let secretConfigured = false;
      for (const f of FIELDS) {
        const v = cfg[f.key];
        if (SECRET_KEYS.has(f.key)) {
          // 密钥：已配置则回显占位符，未配置留空
          if (v && v !== '') {
            init[f.key] = MASK;
            secretConfigured = true;
          } else {
            init[f.key] = '';
          }
        } else if (f.type === 'switch') {
          init[f.key] = v === '1' || v === 'true';
        } else if (f.type === 'number') {
          init[f.key] = v ? Number(v) : (f.def ?? 100);
        } else {
          init[f.key] = v || '';
        }
      }
      setHasSecret(secretConfigured);
      form.setFieldsValue(init);
    } catch {
      /* 读取失败由统一封装提示 */
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const save = async () => {
    const values = await form.validate();
    setSaving(true);
    try {
      const body: Record<string, string> = {};
      for (const f of FIELDS) {
        const v = values[f.key];
        if (f.type === 'switch') {
          body[f.key] = v ? '1' : '0';
        } else if (f.type === 'number') {
          body[f.key] = String(v ?? (f.def ?? 100));
        } else {
          // 密钥类：留空或仍为占位符 => 不提交（后端保留原值）
          if (SECRET_KEYS.has(f.key) && (v === '' || v === MASK || v == null)) {
            continue;
          }
          body[f.key] = (v ?? '').toString();
        }
      }
      await api.paySaveConfig(body);
      Message.success('支付配置已保存');
      load(); // 重新加载，刷新脱敏占位状态
    } catch {
      /* 保存失败由统一封装提示 */
    } finally {
      setSaving(false);
    }
  };

  const renderField = (f: (typeof FIELDS)[number]) => {
    if (f.type === 'switch') {
      return (
        <FormItem key={f.key} field={f.key} label={f.label} triggerPropName="checked">
          <Switch />
        </FormItem>
      );
    }
    if (f.type === 'number') {
      return (
        <FormItem key={f.key} field={f.key} label={f.label} extra={f.tip}>
          <InputNumber min={1} max={100000000} style={{ width: '100%' }} />
        </FormItem>
      );
    }
    if (f.type === 'textarea') {
      return (
        <FormItem key={f.key} field={f.key} label={f.label} extra={f.tip}>
          <Input.TextArea
            autoSize={{ minRows: 2, maxRows: 6 }}
            placeholder={SECRET_KEYS.has(f.key) && hasSecret ? MASK : `请输入${f.label}`}
          />
        </FormItem>
      );
    }
    return (
      <FormItem key={f.key} field={f.key} label={f.label} extra={f.tip}>
        <Input
          placeholder={SECRET_KEYS.has(f.key) && hasSecret ? MASK : `请输入${f.label}`}
        />
      </FormItem>
    );
  };

  return (
    <div>
      <Card title="支付设置" style={{ marginBottom: 16 }}>
        <Alert
          type="info"
          style={{ marginBottom: 16 }}
          content="密钥类字段仅在首次填写时加密保存（enc:v1 AES-GCM），之后回显为 ****** 占位符；留空提交表示保留原值。回调地址需为公网可访问的 HTTPS 地址。"
        />
        <Form form={form} layout="vertical" disabled={loading}>
          <Divider orientation="left">通用</Divider>
          <Row gutter={16}>
            {FIELDS.filter((f) => f.group === 'common').map((f) => (
              <Col span={12} key={f.key}>{renderField(f)}</Col>
            ))}
          </Row>

          <Divider orientation="left">微信支付（Native 扫码）</Divider>
          <Row gutter={16}>
            {FIELDS.filter((f) => f.group === 'wechat').map((f) => (
              <Col span={12} key={f.key}>{renderField(f)}</Col>
            ))}
          </Row>

          <Divider orientation="left">支付宝（当面付扫码）</Divider>
          <Row gutter={16}>
            {FIELDS.filter((f) => f.group === 'alipay').map((f) => (
              <Col span={12} key={f.key}>{renderField(f)}</Col>
            ))}
          </Row>

          <Divider orientation="left">告警通知（到期预警 + 点数不足，推送企微/钉钉群）</Divider>
          <Row gutter={16}>
            {FIELDS.filter((f) => f.group === 'notify').map((f) => (
              <Col span={12} key={f.key}>{renderField(f)}</Col>
            ))}
          </Row>
          <Space style={{ marginBottom: 8 }}>
            <Button
              size="small"
              loading={testing}
              onClick={async () => {
                // 先保存当前表单再测试，保证测的是刚填的 webhook
                await save();
                setTesting(true);
                try {
                  const res: any = await api.notifyTest();
                  Message.success(res?.msg || '已发送');
                } catch {
                  /* 失败由统一封装提示 */
                } finally {
                  setTesting(false);
                }
              }}
            >
              保存并发送测试消息
            </Button>
            <Button
              size="small"
              onClick={async () => {
                try {
                  const d: any = await api.notifyPreview();
                  setPreview(d?.message || '');
                } catch {
                  /* 失败由统一封装提示 */
                }
              }}
            >
              预览当前告警内容
            </Button>
          </Space>
          {preview !== null && (
            <Alert
              type="info"
              style={{ whiteSpace: 'pre-wrap', fontFamily: 'inherit' }}
              content={preview}
            />
          )}

          <Button type="primary" long loading={saving} onClick={save} style={{ marginTop: 8 }}>
            保存支付配置
          </Button>
        </Form>
      </Card>
    </div>
  );
}
