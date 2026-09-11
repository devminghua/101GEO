import { useEffect, useState, type CSSProperties } from 'react';
import {
  Card, Form, Input, Message, Button, Space, Select, Typography,
  Alert, Modal, Spin, Tag, Table, Upload, Tabs, Grid, Divider, Radio,
} from '@arco-design/web-react';
import {
  IconSave, IconUpload, IconInfoCircle, IconHistory,
} from '@arco-design/web-react/icon';
import { api, getStoredUser } from '../api';
import PayConfig from './super/PayConfig';

const { TabPane } = Tabs;
const { Col } = Grid;

// 统一卡片风格：大圆角 + 轻投影，移动端优先（内容自适应宽度）
const cardStyle: CSSProperties = {
  borderRadius: 16,
  boxShadow: '0 2px 14px rgba(0, 0, 0, 0.05)',
};
const sectionStyle: CSSProperties = {
  padding: '0 4px 24px',
};
// 保存按钮统一样式：Arco Form 是 flex-column，子元素默认被拉伸撑满整行；
// 这里限宽到容器 1/3 并靠左，避免出现超长按钮。
const saveBtnStyle: CSSProperties = {
  width: '33%',
  minWidth: 150,
  alignSelf: 'flex-start',
};

// 角色展示文案与颜色
const roleMeta: Record<string, { label: string; color: string }> = {
  super: { label: '总后台管理员', color: 'gold' },
  admin: { label: '分站管理员', color: 'arcoblue' },
  operator: { label: 'AI 优化员', color: 'purple' },
};

export default function Settings() {
  const [form] = Form.useForm();
  const [sysForm] = Form.useForm();
  const [loadingSettings, setLoadingSettings] = useState(true);
  // 当前登录账号（含服务有效期），进入页面时刷新
  const [user, setUser] = useState<any>(getStoredUser());

  const role = user?.role || 'admin';
  const isOperator = role === 'operator';
  const isAdmin = role === 'super' || role === 'admin';
  const isTenantAdmin = role === 'admin';

  // ===== 系统信息（super/admin）=====
  const [sysSaving, setSysSaving] = useState(false);
  const [qrUrl, setQrUrl] = useState('');
  const [qrUploading, setQrUploading] = useState(false);
  // 当前分站 Token 余额（用于品牌名称/版权定制门槛 ≥2000）
  const [tokenBalance, setTokenBalance] = useState<number | null>(null);

  // ===== 登录日志（super/admin）=====
  const [logs, setLogs] = useState<any[]>([]);
  const [logLoading, setLogLoading] = useState(false);
  const [logStatus, setLogStatus] = useState('');
  const [logLimit, setLogLimit] = useState(100);

  const loadLoginLogs = () => {
    setLogLoading(true);
    const params = new URLSearchParams();
    params.set('limit', String(logLimit));
    if (logStatus) params.set('status', logStatus);
    api.listLoginLogs(params.toString()).then((rows: any) => setLogs(rows || [])).catch((e: any) => {
      Message.error(e.message);
    }).finally(() => setLogLoading(false));
  };

  useEffect(() => {
    if (!isOperator) {
      api.getSettings().then((s: any) => {
        form.setFieldsValue({
          default_brand: s.default_brand || '',
          // 阿里云短信
          aliyun_sms_mode: s.aliyun_sms_mode || 'official',
          aliyun_sms_access_key_id: s.aliyun_sms_access_key_id || '',
          aliyun_sms_access_key_secret: s.aliyun_sms_access_key_secret || '',
          aliyun_sms_sign_name: s.aliyun_sms_sign_name || '',
          aliyun_sms_template_code: s.aliyun_sms_template_code || '',
          aliyun_sms_region: s.aliyun_sms_region || 'cn-hangzhou',
          aliyun_sms_endpoint: s.aliyun_sms_endpoint || '',
          // 阿里云 OSS
          aliyun_oss_mode: s.aliyun_oss_mode || 'official',
          aliyun_oss_access_key_id: s.aliyun_oss_access_key_id || '',
          aliyun_oss_access_key_secret: s.aliyun_oss_access_key_secret || '',
          aliyun_oss_bucket: s.aliyun_oss_bucket || '',
          aliyun_oss_region: s.aliyun_oss_region || '',
          aliyun_oss_endpoint: s.aliyun_oss_endpoint || '',
          aliyun_oss_custom_domain: s.aliyun_oss_custom_domain || '',
          // 豆包文生图接口（Seedream 5.0 Pro）
          doubao_image_base_url: s.doubao_image_base_url || '',
          doubao_image_model: s.doubao_image_model || '',
          doubao_image_api_key: s.doubao_image_api_key || '',
          doubao_image_status: s.doubao_image_status || 'enabled',
        });
      }).catch(() => {});
    }
    // 刷新当前账号信息（有效期）
    api.me().then((u: any) => {
      if (u) {
        setUser(u);
        const stored = getStoredUser();
        if (stored) {
          localStorage.setItem('geo_user', JSON.stringify({ ...stored, ...u }));
        }
      }
    }).catch(() => {});
    setLoadingSettings(false);

    if (isAdmin) {
      loadLoginLogs();
      // 系统信息：super 读全局；分站读分站级（my-brand，只影响自己平台）
      if (role === 'super') {
        api.systemInfo().then((s: any) => {
          sysForm.setFieldsValue({
            system_name: s.system_name || '',
            copyright: s.copyright || '',
            service_phone: s.service_phone || '',
            service_wechat_qr: s.service_wechat_qr || '',
          });
          setQrUrl(s.service_wechat_qr || '');
        }).catch(() => {});
      } else {
        api.myBrandInfo().then((s: any) => {
          sysForm.setFieldsValue({
            system_name: s.system_name || '',
            copyright: s.copyright || '',
          });
        }).catch(() => {});
      }
      // 分站 Token 余额（品牌定制门槛）
      if (role !== 'super') {
        api.getPoints().then((p: any) => setTokenBalance(typeof p?.balance === 'number' ? p.balance : 0)).catch(() => {});
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const save = async () => {
    const v = await form.validate();
    try {
      await api.saveSettings(v);
      Message.success('设置已保存');
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  // ===== 系统信息 =====
  // 品牌名称/版权定制门槛：分站需 Token ≥ 2000
  const brandUnlocked = role === 'super' || (tokenBalance ?? 0) >= 2000;
  const promptRecharge = () => {
    Message.warning('自定义品牌名称/版权需 Token ≥ 2000，请先充值');
  };

  // 上传客服微信二维码
  const handleQR = async (option: any) => {
    const { file, onSuccess, onError } = option;
    setQrUploading(true);
    try {
      const data: any = await api.uploadImage(file, 'service_qr');
      setQrUrl(data.url);
      sysForm.setFieldValue('service_wechat_qr', data.url);
      Message.success('二维码已上传');
      onSuccess?.({});
    } catch (e: any) {
      Message.error(e.message || '上传失败');
      onError?.(e);
    } finally {
      setQrUploading(false);
    }
  };

  const saveSystemInfo = async () => {
    const v = await sysForm.validate();
    // 分站品牌定制门槛：Token < 2000 拦截
    if (role !== 'super' && (tokenBalance ?? 0) < 2000) {
      promptRecharge();
      return;
    }
    setSysSaving(true);
    try {
      const res: any = await api.saveSystemInfo({
        system_name: v.system_name || '',
        copyright: v.copyright || '',
        // 客服电话/二维码仅 super 可配置；非 super 不提交（避免把全局配置清空）
        ...(role === 'super' ? { service_phone: v.service_phone || '', service_wechat_qr: qrUrl || '' } : {}),
      });
      if (res && res.msg) Message.success(res.msg);
      // 分站：同步更新本地 user.brand_name，让顶栏品牌名即时生效
      if (role !== 'super') {
        const stored = getStoredUser();
        if (stored) {
          const updated = { ...stored, brand_name: v.system_name || '' };
          localStorage.setItem('geo_user', JSON.stringify(updated));
          setUser(updated);
        }
      }
      // 保存后刷新侧边栏 / 页脚品牌
      window.dispatchEvent(new Event('geo-system-info-updated'));
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSysSaving(false);
    }
  };

  const roleTag = (r: string) => {
    const meta = roleMeta[r] || { label: r || '-', color: 'gray' };
    return <Tag color={meta.color}>{meta.label}</Tag>;
  };

  const logColumns = [
    {
      title: '时间',
      dataIndex: 'created_at',
      width: 180,
      render: (v: any) => (v ? new Date(v).toLocaleString() : '-'),
    },
    {
      title: '账号',
      width: 160,
      render: (_: any, row: any) => (
        <Space direction="vertical" size={0}>
          <Typography.Text>{row.username}</Typography.Text>
          {row.nickname ? (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{row.nickname}</Typography.Text>
          ) : null}
        </Space>
      ),
    },
    { title: '角色', width: 120, render: (_: any, row: any) => roleTag(row.role) },
    ...(role === 'super' ? [{ title: '租户', dataIndex: 'tenant_id', width: 90, render: (v: any) => (v ? `#${v}` : '总后台') }] : []),
    { title: 'IP', dataIndex: 'ip', width: 150, render: (v: any) => v || '-' },
    {
      title: '状态',
      width: 90,
      render: (_: any, row: any) =>
        row.status === 'success' ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag>,
    },
    { title: '原因', dataIndex: 'reason', render: (v: any) => v || '-' },
  ];

  // 按角色动态生成 Tab 列表（系统信息 / 豆包接口 / 短信·OSS 仅 SaaS 端）
  const tabList: { key: string; title: string }[] = [];
  if (isAdmin) {
    tabList.push({ key: 'system', title: '系统信息' });
    tabList.push({ key: 'doubao-image', title: '豆包文生图' });
  }
  // 短信 / OSS / 支付（微信·支付宝）/ 定价 / 告警：仅 SaaS 端（super）可见，客户端不自行配置
  if (role === 'super') {
    tabList.push({ key: 'aliyun-sms', title: '短信设置' });
    tabList.push({ key: 'aliyun-oss', title: 'OSS 设置' });
    tabList.push({ key: 'wechat-pay', title: '微信设置' });
    tabList.push({ key: 'alipay-pay', title: '支付宝设置' });
    tabList.push({ key: 'pay-pricing', title: '充值定价' });
    tabList.push({ key: 'pay-notify', title: '告警通知' });
  }
  if (isAdmin) tabList.push({ key: 'login-logs', title: '登录日志' });

  const [activeTab, setActiveTab] = useState<string>(tabList[0]?.key || 'system');

  const renderSystemTab = () => (
    <Card title="系统信息" style={cardStyle} bordered={false}>
      <Form form={sysForm} layout="vertical" style={{ maxWidth: 560 }}>
        {role !== 'super' && !brandUnlocked && (
          <div style={{ background: 'var(--color-fill-2)', borderRadius: 8, padding: '10px 14px', fontSize: 13, color: '#86909C', marginBottom: 16 }}>
            自定义「系统名称」和「底部版权」需 Token ≥ 2000，当前 {tokenBalance ?? 0}。不足请到「充值」页面充值。
          </div>
        )}
        <Form.Item
          label="系统名称"
          field="system_name"
          extra={role === 'super' ? '留空则使用默认名称' : 'Token ≥ 2000 可自定义，留空使用默认名称'}
        >
          <Input
            placeholder="如：LinkGeo"
            maxLength={40}
            disabled={role !== 'super' && !brandUnlocked}
            onClick={role !== 'super' && !brandUnlocked ? promptRecharge : undefined}
          />
        </Form.Item>
        <Form.Item
          label="底部版权"
          field="copyright"
          extra={role === 'super' ? '留空则不显示底部版权' : 'Token ≥ 2000 可自定义，留空不显示'}
        >
          <Input.TextArea
            placeholder="如：© 2026 轻媒（苏州）科技有限公司 版权所有"
            maxLength={120}
            autoSize
            disabled={role !== 'super' && !brandUnlocked}
            onClick={role !== 'super' && !brandUnlocked ? promptRecharge : undefined}
          />
        </Form.Item>
        {role === 'super' && (
          <>
            <Form.Item
              label="技术服务电话"
              field="service_phone"
              extra="客户端右下角客服悬浮展示的电话，留空则不显示客服悬浮"
            >
              <Input placeholder="如：400-123-4567 或 13800000000" maxLength={30} />
            </Form.Item>
            <Form.Item label="技术服务微信二维码" field="service_wechat_qr">
              <Space>
                <Upload
                  customRequest={handleQR}
                  showUploadList={false}
                  accept="image/png,image/jpeg,image/webp,image/gif"
                >
                  <Button icon={<IconUpload />} loading={qrUploading}>
                    上传二维码
                  </Button>
                </Upload>
                {qrUrl ? (
                  <img
                    src={qrUrl}
                    alt="二维码"
                    style={{ height: 88, width: 88, objectFit: 'contain', background: 'var(--color-fill-2)', borderRadius: 8, padding: 4 }}
                  />
                ) : (
                  <Typography.Text type="secondary">未设置二维码</Typography.Text>
                )}
              </Space>
            </Form.Item>
          </>
        )}
        <Button type="primary" icon={<IconSave />} loading={sysSaving} onClick={saveSystemInfo} style={saveBtnStyle}>
          保存系统信息
        </Button>
      </Form>
    </Card>
  );

  const renderSmsTab = () => (
    <Card title="短信设置" style={cardStyle} bordered={false}>
      <Spin loading={loadingSettings} tip="加载中..." style={{ display: "block", width: "100%" }}>
        <Form form={form} layout="vertical" style={{ width: "100%", maxWidth: 900 }}>
          {/* 阿里云短信接口 */}
          <Card
            title="阿里云短信接口"
            size="small"
            style={{ ...cardStyle, boxShadow: 'none', border: '1px solid var(--color-border-2)' }}
            extra={
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                支持阿里云官方短信服务（验证码/通知），或接入自有阿里云接口网关
              </Typography.Text>
            }
          >
            <Form.Item label="接入方式" field="aliyun_sms_mode">
              <Radio.Group>
                <Radio value="official">官方统一接口</Radio>
                <Radio value="custom">自定义接口</Radio>
              </Radio.Group>
            </Form.Item>
            <Form.Item
              label="AccessKey ID"
              field="aliyun_sms_access_key_id"
              rules={[{ required: true, message: '请输入 AccessKey ID' }]}
            >
              <Input placeholder="阿里云 AccessKey ID" />
            </Form.Item>
            <Form.Item
              label="AccessKey Secret"
              field="aliyun_sms_access_key_secret"
              extra="已设置时显示 ******。留空表示保留原密钥；填写新值才会覆盖"
            >
              <Input.Password placeholder="已设置则留空保留原密钥" autoComplete="new-password" />
            </Form.Item>
            <Form.Item
              label="短信签名 SignName"
              field="aliyun_sms_sign_name"
              rules={[{ required: true, message: '请输入短信签名' }]}
              extra="阿里云短信服务申请获得，如：轻媒"
            >
              <Input placeholder="短信签名" />
            </Form.Item>
            <Form.Item
              label="模板 CODE"
              field="aliyun_sms_template_code"
              rules={[{ required: true, message: '请输入模板 CODE' }]}
              extra="阿里云短信模板编号，如 SMS_123456789"
            >
              <Input placeholder="SMS_123456789" />
            </Form.Item>
            <Form.Item label="Region" field="aliyun_sms_region">
              <Input placeholder="cn-hangzhou" />
            </Form.Item>
            <Form.Item noStyle shouldUpdate={(prev, cur) => prev.aliyun_sms_mode !== cur.aliyun_sms_mode}>
              {(formData) => {
                const isCustom = formData.aliyun_sms_mode === 'custom';
                return (
                  <Form.Item
                    label="自定义 Endpoint"
                    field="aliyun_sms_endpoint"
                    extra={isCustom ? '填写自有网关地址，如 https://sms.example.com' : '官方模式默认 dysmsapi.aliyuncs.com，可留空'}
                  >
                    <Input placeholder="https://sms.example.com" disabled={!isCustom} />
                  </Form.Item>
                );
              }}
            </Form.Item>
          </Card>
          <Button type="primary" icon={<IconSave />} onClick={save} style={{ ...saveBtnStyle, marginTop: 20 }}>
            保存设置
          </Button>
        </Form>
      </Spin>
    </Card>
  );

  const renderOssTab = () => (
    <Card title="OSS 设置" style={cardStyle} bordered={false}>
      <Spin loading={loadingSettings} tip="加载中..." style={{ display: "block", width: "100%" }}>
        <Form form={form} layout="vertical" style={{ width: "100%", maxWidth: 900 }}>
          {/* 阿里云 OSS 接口 */}
          <Card
            title="阿里云 OSS 接口"
            size="small"
            style={{ ...cardStyle, boxShadow: 'none', border: '1px solid var(--color-border-2)' }}
            extra={
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                对象存储：官方模式自动拼 https://oss-{`{region}`}.aliyuncs.com，或接入自有网关
              </Typography.Text>
            }
          >
            <Form.Item label="接入方式" field="aliyun_oss_mode">
              <Radio.Group>
                <Radio value="official">官方统一接口</Radio>
                <Radio value="custom">自定义接口</Radio>
              </Radio.Group>
            </Form.Item>
            <Form.Item
              label="AccessKey ID"
              field="aliyun_oss_access_key_id"
              rules={[{ required: true, message: '请输入 AccessKey ID' }]}
            >
              <Input placeholder="阿里云 AccessKey ID" />
            </Form.Item>
            <Form.Item
              label="AccessKey Secret"
              field="aliyun_oss_access_key_secret"
              extra="已设置时显示 ******。留空表示保留原密钥；填写新值才会覆盖"
            >
              <Input.Password placeholder="已设置则留空保留原密钥" autoComplete="new-password" />
            </Form.Item>
            <Form.Item
              label="Bucket"
              field="aliyun_oss_bucket"
              rules={[{ required: true, message: '请输入 Bucket' }]}
              extra="存储空间名称，如 geo-tool"
            >
              <Input placeholder="geo-tool" />
            </Form.Item>
            <Form.Item
              label="Region"
              field="aliyun_oss_region"
              rules={[{ required: true, message: '请输入 Region' }]}
              extra="官方模式必填，如 cn-hangzhou"
            >
              <Input placeholder="cn-hangzhou" />
            </Form.Item>
            <Form.Item noStyle shouldUpdate={(prev, cur) => prev.aliyun_oss_mode !== cur.aliyun_oss_mode}>
              {(formData) => {
                const isCustom = formData.aliyun_oss_mode === 'custom';
                return (
                  <Form.Item
                    label="自定义 Endpoint"
                    field="aliyun_oss_endpoint"
                    extra={isCustom ? '填写自有网关地址，如 https://oss.example.com' : '官方模式自动拼 https://oss-{region}.aliyuncs.com，可留空'}
                  >
                    <Input placeholder="https://oss.example.com" disabled={!isCustom} />
                  </Form.Item>
                );
              }}
            </Form.Item>
            <Form.Item
              label="自定义访问域名"
              field="aliyun_oss_custom_domain"
              extra="可选，CDN/自有域名（https://cdn.example.com），留空则用默认 endpoint 访问"
            >
              <Input placeholder="https://cdn.example.com" />
            </Form.Item>
          </Card>
          <Button type="primary" icon={<IconSave />} onClick={save} style={{ ...saveBtnStyle, marginTop: 20 }}>
            保存设置
          </Button>
        </Form>
      </Spin>
    </Card>
  );

  const renderDoubaoImageTab = () => (
    <Card title="豆包文生图接口" style={cardStyle} bordered={false}>
      <Spin loading={loadingSettings} tip="加载中..." style={{ display: "block", width: "100%" }}>
        <Form form={form} layout="vertical" style={{ width: "100%", maxWidth: 900 }}>
          {/* 豆包文生图接口（Seedream 5.0 Pro） */}
          <Card
            title="豆包文生图接口（Seedream 5.0 Pro）"
            size="small"
            style={{ ...cardStyle, boxShadow: 'none', border: '1px solid var(--color-border-2)' }}
            extra={
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                火山方舟 Seedream 文生图，OpenAI 兼容 /images/generations
              </Typography.Text>
            }
          >
            <Form.Item
              label="启用状态"
              field="doubao_image_status"
              extra="关闭后智能创作中心的图片生成将回退使用 AiPlatform 配置"
            >
              <Radio.Group>
                <Radio value="enabled">启用</Radio>
                <Radio value="disabled">停用</Radio>
              </Radio.Group>
            </Form.Item>
            <Form.Item
              label="接口地址 Base URL"
              field="doubao_image_base_url"
              rules={[{ required: true, message: '请输入豆包文生图接口地址' }]}
              extra="火山方舟 OpenAI 兼容地址，如 https://ark.cn-beijing.volces.com/api/v3"
            >
              <Input placeholder="https://ark.cn-beijing.volces.com/api/v3" />
            </Form.Item>
            <Form.Item
              label="模型名 Model"
              field="doubao_image_model"
              rules={[{ required: true, message: '请输入模型名' }]}
              extra="如 doubao-seedream-5-0-pro-250828"
            >
              <Input placeholder="doubao-seedream-5-0-pro-250828" />
            </Form.Item>
            <Form.Item
              label="API Key"
              field="doubao_image_api_key"
              extra="已设置时显示 ******。留空表示保留原密钥；填写新值才会覆盖"
            >
              <Input.Password placeholder="已设置则留空保留原密钥" autoComplete="new-password" />
            </Form.Item>
          </Card>
          <Button type="primary" icon={<IconSave />} onClick={save} style={{ ...saveBtnStyle, marginTop: 20 }}>
            保存设置
          </Button>
        </Form>
      </Spin>
    </Card>
  );

  const renderLoginLogsTab = () => (
    <Card
      title="登录日志"
      style={cardStyle}
      bordered={false}
      extra={
        <Space>
          <Select
            value={logStatus}
            onChange={(v) => setLogStatus(v || '')}
            style={{ width: 110 }}
            placeholder="全部状态"
            options={[
              { label: '全部状态', value: '' },
              { label: '成功', value: 'success' },
              { label: '失败', value: 'fail' },
            ]}
          />
          <Select
            value={logLimit}
            onChange={(v) => setLogLimit(v as number)}
            style={{ width: 100 }}
            options={[50, 100, 200].map((n) => ({ label: `${n} 条`, value: n }))}
          />
          <Button icon={<IconHistory />} loading={logLoading} onClick={loadLoginLogs}>
            刷新
          </Button>
        </Space>
      }
    >
      <Table
        rowKey="id"
        columns={logColumns}
        data={logs}
        loading={logLoading}
        pagination={{ pageSize: 10, showTotal: true }}
        scroll={{ x: 900 }}
        noDataElement="暂无登录记录"
      />
    </Card>
  );

  return (
    <div style={sectionStyle}>
      {/* AI 优化员 权限说明（operator 专属） */}
      {isOperator && (
        <Card style={{ ...cardStyle, marginBottom: 16 }} bordered={false}>
          <Alert
            type="info"
            icon={<IconInfoCircle />}
            content="当前账号为 AI 优化员，仅可进行日常业务操作（关键词监控、巡检、百度/抖音/小红书获客、智能创作等）。无以下权限：配置或修改 API、查看 API 密钥、修改或设置后台密码、管理系统信息与账号。如有需要请联系管理员。"
          />
        </Card>
      )}

      <Tabs
        activeTab={activeTab}
        onChange={(key) => setActiveTab(String(key))}
        type="line"
        style={{ marginBottom: 16 }}
      >
        {tabList.map((t) => (
          <TabPane key={t.key} title={t.title}>
            {t.key === 'system' && renderSystemTab()}
            {t.key === 'aliyun-sms' && renderSmsTab()}
            {t.key === 'aliyun-oss' && renderOssTab()}
            {/* 支付：微信 / 支付宝配置 + 定价 / 告警（复用 super/PayConfig，按分组拆分独立 Tab） */}
            {t.key === 'wechat-pay' && <PayConfig groups={['wechat']} title="微信支付设置" />}
            {t.key === 'alipay-pay' && <PayConfig groups={['alipay']} title="支付宝支付设置" />}
            {t.key === 'pay-pricing' && <PayConfig groups={['common']} title="充值定价设置" />}
            {t.key === 'pay-notify' && <PayConfig groups={['notify']} title="告警通知设置" />}
            {t.key === 'doubao-image' && renderDoubaoImageTab()}
            {t.key === 'login-logs' && renderLoginLogsTab()}
          </TabPane>
        ))}
      </Tabs>
      <Divider style={{ margin: '0 0 4px' }} />
    </div>
  );
}
