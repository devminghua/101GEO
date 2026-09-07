import { useCallback, useEffect, useRef, useState } from 'react';
import {
  Card, Grid, Button, Message, Tag, Space, Input, Tabs, Table, Modal, Select, Radio,
  Spin, Empty, Alert, Divider, Progress, Typography, Badge,
} from '@arco-design/web-react';
import {
  IconPlus, IconDelete, IconEdit, IconSend, IconCopy, IconRefresh, IconDownload,
  IconSave, IconImage, IconPen, IconMessage, IconUserGroup,
} from '@arco-design/web-react/icon';
import { api } from '../api';
import './CreationCenter.css';

const { Row, Col } = Grid;
const TextArea = Input.TextArea;
const { Title, Text, Paragraph } = Typography;

/* ================================================================
 * 智能创作中心 · 完整内部功能（前后端全栈，已对接真实后端 API）
 *
 * 模块：
 *  1. AI 助手对话：多轮会话（新建/删除/续聊），角色系统提示词可控
 *  2. 角色设定：预制角色（内置 ≥6）/ 提示词模板 CRUD
 *  3. 文案创作：文案写作 / 抖音热门视频脚本 / 小红书热门文案 / 深度学习 / 洗稿
 *  4. 图片生成：OpenAI 兼容文生图，展示 + 保存
 *
 * AI 能力统一走后端 OpenAI 兼容客户端，模型基于当前租户 AiPlatform 配置；
 * 未配置可用模型时后端返回「请先配置 AI 平台」，本页做同文案提示。
 * ================================================================ */

const fmtTime = (t?: string | null): string => {
  if (!t) return '--';
  const d = new Date(t);
  if (Number.isNaN(d.getTime())) return '--';
  const p = (x: number) => (x < 10 ? `0${x}` : String(x));
  return `${d.getFullYear()}-${d.getMonth() + 1}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
};

const kindTag = (k?: string) => {
  const map: Record<string, { c: string; t: string }> = {
    copy: { c: 'arcoblue', t: '文案' },
    script: { c: 'purple', t: '抖音脚本' },
    xhs: { c: 'red', t: '小红书' },
    learn: { c: 'cyan', t: '深度学习' },
    xiegou: { c: 'orange', t: '洗稿' },
    image: { c: 'green', t: '图片' },
  };
  const m = map[k || ''];
  return m ? <Tag color={m.c} size="small">{m.t}</Tag> : <Tag size="small">{k}</Tag>;
};

export default function CreationCenter() {
  const [active, setActive] = useState('chat');
  const [loading, setLoading] = useState(false);

  return (
    <div>
      <Alert
        type="info"
        style={{ marginBottom: 16 }}
        title="智能创作中心"
        content="AI 生成能力统一基于左侧「AI 平台」中配置的可商用大模型。未配置可用模型时，各功能会提示“请先配置 AI 平台”。模型 API 调用由后端统一代理，密钥不会暴露到前端。"
      />
      <Card>
        <Tabs activeTab={active} onChange={setActive} type="line">
          <Tabs.TabPane key="chat" title={<span><IconMessage /> AI 助手对话</span>}>
            <ChatTab />
          </Tabs.TabPane>
          <Tabs.TabPane key="roles" title={<span><IconUserGroup /> 角色设定</span>}>
            <RolesTab />
          </Tabs.TabPane>
          <Tabs.TabPane key="copy" title={<span><IconPen /> 文案创作</span>}>
            <CopyTab />
          </Tabs.TabPane>
          <Tabs.TabPane key="image" title={<span><IconImage /> 图片生成</span>}>
            <ImageTab />
          </Tabs.TabPane>
          <Tabs.TabPane key="materials" title={<span><IconSave /> 素材库与记录</span>}>
            <MaterialsTab />
          </Tabs.TabPane>
        </Tabs>
      </Card>
    </div>
  );
}

/* ================= AI 助手对话 ================= */

function ChatTab() {
  const [sessions, setSessions] = useState<any[]>([]);
  const [cur, setCur] = useState<any>(null);
  const [roles, setRoles] = useState<any[]>([]);
  const [msgs, setMsgs] = useState<any[]>([]);
  const [input, setInput] = useState('');
  const [roleId, setRoleId] = useState<number>(0);
  const [sending, setSending] = useState(false);
  const [typing, setTyping] = useState('');
  const [newTitle, setNewTitle] = useState('');
  const boxRef = useRef<HTMLDivElement>(null);
  const typingRef = useRef<number>(0);

  const loadSessions = useCallback(async () => {
    try {
      setSessions(await api.listSessions());
    } catch (e: any) {
      Message.error(e.message);
    }
  }, []);

  const loadRoles = useCallback(async () => {
    try {
      setRoles(await api.listRoles());
    } catch {}
  }, []);

  useEffect(() => {
    loadSessions();
    loadRoles();
  }, [loadSessions, loadRoles]);

  useEffect(() => {
    if (boxRef.current) boxRef.current.scrollTop = boxRef.current.scrollHeight;
  }, [msgs, typing]);

  useEffect(() => () => window.clearTimeout(typingRef.current), []);

  const openSession = async (id: number) => {
    window.clearTimeout(typingRef.current);
    setTyping('');
    const s = sessions.find((x) => x.id === id) || null;
    setCur(s);
    try {
      const m = await api.listSessionMessages(id);
      setMsgs(m);
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const createSession = async () => {
    try {
      const s = await api.createSession({ title: newTitle, role_id: roleId });
      setNewTitle('');
      await loadSessions();
      setCur(s);
      setMsgs([]);
      Message.success('已新建会话');
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const removeSession = async (id: number) => {
    try {
      await api.deleteSession(id);
      if (cur && cur.id === id) {
        setCur(null);
        setMsgs([]);
      }
      await loadSessions();
      Message.success('已删除会话');
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  /* 逐字"打字"输出：标点处停顿稍长，配合心跳光标；期间通过 onTick 更新已打出的文本 */
  const typewriter = (full: string, onTick: (s: string) => void, onDone: () => void) => {
    window.clearTimeout(typingRef.current);
    let i = 0;
    const step = () => {
      if (i > full.length) {
        onDone();
        return;
      }
      onTick(full.slice(0, i));
      const ch = full[i] || '';
      i += 1;
      const pause = /[。！？；，、.,;!?]/.test(ch) ? 90 : 16;
      typingRef.current = window.setTimeout(step, pause);
    };
    step();
  };

  const send = async () => {
    const content = input.trim();
    if (!content) return;
    if (!cur) {
      Message.warning('请先新建或选择会话');
      return;
    }
    window.clearTimeout(typingRef.current);
    setTyping('');
    setInput('');
    const um = { id: Date.now(), role: 'user', content, created_at: new Date().toISOString() };
    const base: any[] = [...msgs, um];
    setMsgs(base);
    setSending(true);
    try {
      const data = await api.chatSend({ session_id: cur.id, content });
      const reply: string = data && data.reply ? String(data.reply) : '';
      await new Promise<void>((resolve) => {
        typewriter(reply, (s) => setTyping(s), resolve);
      });
      setMsgs([...base, { id: Date.now() + 1, role: 'assistant', content: reply, created_at: new Date().toISOString() }]);
      setTyping('');
      await loadSessions();
    } catch (e: any) {
      setTyping('');
      setMsgs([...base, { id: Date.now() + 1, role: 'assistant', content: `[生成失败] ${e.message}`, created_at: new Date().toISOString() }]);
    } finally {
      setSending(false);
    }
  };

  const copyMsg = (t: string) => {
    navigator.clipboard?.writeText(t).then(() => Message.success('已复制'));
  };

  return (
    <Row gutter={16}>
      {/* 左侧：会话管理 */}
      <Col xs={24} md={6}>
        <div className="chatp-side">
          <div className="chatp-side-head">
            <span style={{ fontWeight: 600, fontSize: 14 }}>会话</span>
            <Button size="mini" type="primary" shape="round" icon={<IconPlus />} onClick={createSession}>
              新建
            </Button>
          </div>
          <div className="chatp-side-create">
            <Input placeholder="会话标题（可选）" value={newTitle} onChange={(v) => setNewTitle(v)} style={{ borderRadius: 10 }} />
            <Select
              placeholder="选择角色（默认通用助手）"
              allowClear
              value={roleId || undefined}
              onChange={(v) => setRoleId(v || 0)}
              style={{ width: '100%' }}
            >
              {(roles || []).map((r) => (
                <Select.Option key={r.id} value={r.id}>{r.name}</Select.Option>
              ))}
            </Select>
          </div>
          <div className="chatp-side-list">
            {sessions.length === 0 && (
              <div style={{ textAlign: 'center', color: 'var(--color-text-3)', padding: 24, fontSize: 12 }}>
                暂无会话，点击「新建」
              </div>
            )}
            {sessions.map((s: any) => (
              <div
                key={s.id}
                className={`chatp-session ${cur && cur.id === s.id ? 'active' : ''}`}
                onClick={() => openSession(s.id)}
              >
                <div style={{ minWidth: 0 }}>
                  <div className="chatp-session-title">{s.title}</div>
                  <div className="chatp-session-sub">
                    {s.role_name || '通用助手'} · {s.message_count || 0} 条
                  </div>
                </div>
                <Button
                  size="mini"
                  status="danger"
                  icon={<IconDelete />}
                  onClick={(e) => { e.stopPropagation(); removeSession(s.id); }}
                />
              </div>
            ))}
          </div>
        </div>
      </Col>

      {/* 右侧：对话窗口 */}
      <Col xs={24} md={18}>
        <div className="chatp-win">
          <div className="chatp-header">
            <div style={{ minWidth: 0 }}>
              <div className="chatp-header-title">
                {cur ? cur.title : '新对话'}
                {cur && cur.role_name ? `（${cur.role_name}）` : ''}
              </div>
              <div className="chatp-header-sub">
                {cur ? '多轮对话进行中' : '请新建或选择左侧会话开始对话'}
              </div>
            </div>
            <Tag color="arcoblue" size="small">AI 助手</Tag>
          </div>

          <div className="chatp-body" ref={boxRef}>
            {msgs.length === 0 && !typing && (
              <div className="chatp-empty">
                选择左侧会话，或新建会话开始对话<br />回复将逐字呈现，如同真实对话
              </div>
            )}
            {msgs.map((m: any) => (
              <div key={m.id} className={`msg-row ${m.role === 'user' ? 'user' : 'ai'}`}>
                {m.role !== 'user' && <div className="avatar ai">AI</div>}
                <div className="bubble">
                  <div>{m.content}</div>
                  <div className="bubble-meta">
                    <span>{m.role === 'user' ? '我' : 'AI'} · {fmtTime(m.created_at)}</span>
                    {m.role !== 'user' && (
                      <button className="copy-btn" onClick={() => copyMsg(m.content)}>复制</button>
                    )}
                  </div>
                </div>
                {m.role === 'user' && <div className="avatar user">我</div>}
              </div>
            ))}
            {(sending || typing) && (
              <div className="msg-row ai">
                <div className="avatar ai">AI</div>
                <div className="bubble bubble-typing">
                  <div className="bubble-text">
                    {typing ? (
                      <span>{typing}<span className="chatp-caret" /></span>
                    ) : (
                      <span className="thinking">正在思考<span className="chatp-caret" /></span>
                    )}
                  </div>
                </div>
              </div>
            )}
          </div>

          <div className="chatp-input">
            <TextArea
              placeholder="输入消息，Enter 发送 / Shift+Enter 换行"
              value={input}
              onChange={(v) => setInput(v)}
              onPressEnter={(e) => {
                if (!e.shiftKey) {
                  e.preventDefault();
                  send();
                }
              }}
              style={{ flex: 1 }}
              autoSize={{ minRows: 2, maxRows: 6 }}
            />
            <div className="chatp-input-bar">
              <span className="chatp-hint">Enter 发送 · Shift+Enter 换行</span>
              <Button
                className="chatp-send"
                type="primary"
                shape="round"
                disabled={!input.trim() || sending || !cur}
                onClick={send}
              >
                {sending ? (
                  <>
                    <span className="chatp-spinner" />
                    <span>生成中…</span>
                  </>
                ) : (
                  <>
                    <IconSend />
                    <span>发送</span>
                  </>
                )}
              </Button>
            </div>
          </div>
        </div>
      </Col>
    </Row>
  );
}

/* ================= 角色设定 ================= */

function RolesTab() {
  const [list, setList] = useState<any[]>([]);
  const [modal, setModal] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [form, setForm] = useState({ name: '', category: '', description: '', system_prompt: '' });
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    try {
      setList(await api.listRoles());
    } catch (e: any) {
      Message.error(e.message);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const openCreate = () => {
    setEditing(null);
    setForm({ name: '', category: '', description: '', system_prompt: '' });
    setModal(true);
  };

  const openEdit = (r: any) => {
    setEditing(r);
    setForm({ name: r.name, category: r.category || '', description: r.description || '', system_prompt: r.system_prompt });
    setModal(true);
  };

  const save = async () => {
    if (!form.name.trim() || !form.system_prompt.trim()) {
      Message.warning('请填写角色名称与系统提示词');
      return;
    }
    setSaving(true);
    try {
      if (editing) {
        await api.updateRole(editing.id, form);
      } else {
        await api.createRole(form);
      }
      Message.success('已保存');
      setModal(false);
      await load();
    } catch (e: any) {
      Message.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  const del = async (r: any) => {
    try {
      await api.deleteRole(r.id);
      Message.success('已删除');
      await load();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  return (
    <div>
      <Space style={{ marginBottom: 12 }}>
        <Button type="primary" icon={<IconPlus />} onClick={openCreate}>新建角色</Button>
        <Text type="secondary">内置角色可编辑提示词但不可删除；角色提示词将作为对话 / 创作的系统提示词。</Text>
      </Space>
      <Table
        rowKey="id"
        data={list}
        loading={false}
        pagination={false}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 60 },
          {
            title: '角色名', dataIndex: 'name', width: 120,
            render: (v: string, r: any) => (
              <Space>
                {v}
                {r.builtin && <Tag color="arcoblue" size="small">内置</Tag>}
              </Space>
            ),
          },
          { title: '分类', dataIndex: 'category', width: 100, render: (v: string) => v || '--' },
          { title: '简介', dataIndex: 'description', width: 220, render: (v: string) => v || '--' },
          { title: '系统提示词', dataIndex: 'system_prompt', ellipsis: true },
          { title: '创建时间', dataIndex: 'created_at', width: 150, render: (v: string) => fmtTime(v) },
          {
            title: '操作', width: 120,
            render: (_: any, r: any) => (
              <Space>
                <Button size="mini" type="text" icon={<IconEdit />} onClick={() => openEdit(r)}>编辑</Button>
                {!r.builtin && (
                  <Button size="mini" type="text" status="danger" icon={<IconDelete />} onClick={() => del(r)}>删除</Button>
                )}
              </Space>
            ),
          },
        ]}
      />
      <Modal
        title={editing ? '编辑角色' : '新建角色'}
        visible={modal}
        onCancel={() => setModal(false)}
        onOk={save}
        confirmLoading={saving}
        style={{ width: 640 }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Space>
            <Input style={{ width: 200 }} placeholder="角色名称（如 专业文案）" value={form.name} onChange={(v) => setForm({ ...form, name: v })} />
            <Input style={{ width: 160 }} placeholder="分类（如 文案/脚本/运营）" value={form.category} onChange={(v) => setForm({ ...form, category: v })} />
          </Space>
          <Input placeholder="一句话简介" value={form.description} onChange={(v) => setForm({ ...form, description: v })} />
          <TextArea
            placeholder="系统提示词（决定该角色在对话/创作中的行为风格）"
            value={form.system_prompt}
            onChange={(v) => setForm({ ...form, system_prompt: v })}
            autoSize={{ minRows: 5 }}
          />
        </Space>
      </Modal>
    </div>
  );
}

/* ================= 文案创作（写作/脚本/小红书/学习/洗稿） ================= */

function CopyTab() {
  const [sub, setSub] = useState('write');
  const [roles, setRoles] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ title: string; text: string; platform?: string; model?: string }>({ title: '', text: '' });

  const [writeForm, setWriteForm] = useState({ role_id: 0, theme: '', requirements: '' });
  const [scriptTheme, setScriptTheme] = useState('');
  const [xhsTheme, setXhsTheme] = useState('');
  const [learnForm, setLearnForm] = useState({ reference: '', theme: '' });
  const [xiegouText, setXiegouText] = useState('');

  useEffect(() => {
    api.listRoles().then(setRoles).catch(() => {});
  }, []);

  const run = async (title: string, fn: Promise<any>) => {
    setLoading(true);
    setResult({ title, text: '' });
    try {
      const d = await fn;
      setResult({ title, text: d.text, platform: d.platform, model: d.model });
    } catch (e: any) {
      setResult({ title, text: `[生成失败] ${e.message}` });
    } finally {
      setLoading(false);
    }
  };

  const copy = () => {
    if (!result.text) return;
    navigator.clipboard?.writeText(result.text).then(() => Message.success('已复制'));
  };

  const saveAsMaterial = async () => {
    if (!result.text) return;
    try {
      await api.saveMaterial({ title: result.title, kind: 'copy', content: result.text });
      Message.success('已保存到素材库');
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const innerTabs = [
    { key: 'write', title: '文案写作' },
    { key: 'script', title: '抖音热门脚本' },
    { key: 'xhs', title: '小红书热门文案' },
    { key: 'learn', title: '深度学习' },
    { key: 'xiegou', title: '洗稿改写' },
  ];

  return (
    <div>
      <Tabs activeTab={sub} onChange={setSub} type="rounded" style={{ marginBottom: 16 }}>
        {innerTabs.map((t) => (
          <Tabs.TabPane key={t.key} title={t.title} />
        ))}
      </Tabs>
      <Row gutter={16}>
        <Col span={10}>
          <Card size="small" title={innerTabs.find((t) => t.key === sub)?.title}>
            {sub === 'write' && (
              <Space direction="vertical" style={{ width: '100%' }}>
                <Select
                  placeholder="选择创作角色（可不选）" allowClear value={writeForm.role_id || undefined}
                  onChange={(v) => setWriteForm({ ...writeForm, role_id: v || 0 })} style={{ width: '100%' }}
                >
                  {(roles || []).map((r) => <Select.Option key={r.id} value={r.id}>{r.name}</Select.Option>)}
                </Select>
                <Input placeholder="创作主题（如：夏季防晒霜卖点文案）" value={writeForm.theme} onChange={(v) => setWriteForm({ ...writeForm, theme: v })} />
                <TextArea placeholder="补充要求（语气、字数、平台、受众等，可选）" value={writeForm.requirements} onChange={(v) => setWriteForm({ ...writeForm, requirements: v })} autoSize={{ minRows: 4 }} />
                <Button type="primary" long loading={loading} icon={<IconPen />}
                  onClick={() => run('文案创作', api.writeCopy(writeForm))}>
                  生成文案
                </Button>
              </Space>
            )}
            {sub === 'script' && (
              <Space direction="vertical" style={{ width: '100%' }}>
                <TextArea placeholder="输入视频主题（如：夏天如何在半小时内拍出日系氛围感 Vlog）" value={scriptTheme} onChange={setScriptTheme} autoSize={{ minRows: 4 }} />
                <Button type="primary" long loading={loading} icon={<IconPen />}
                  onClick={() => run('抖音热门视频脚本', api.genScript({ theme: scriptTheme }))}>
                  生成分镜脚本
                </Button>
              </Space>
            )}
            {sub === 'xhs' && (
              <Space direction="vertical" style={{ width: '100%' }}>
                <TextArea placeholder="输入笔记主题（如：平价护肤新手入门）" value={xhsTheme} onChange={setXhsTheme} autoSize={{ minRows: 4 }} />
                <Button type="primary" long loading={loading} icon={<IconPen />}
                  onClick={() => run('小红书热门文案', api.genXhsCopy({ theme: xhsTheme }))}>
                  生成种草文案
                </Button>
              </Space>
            )}
            {sub === 'learn' && (
              <Space direction="vertical" style={{ width: '100%' }}>
                <TextArea placeholder="参考文本 / 链接（AI 学习其风格）" value={learnForm.reference} onChange={(v) => setLearnForm({ ...learnForm, reference: v })} autoSize={{ minRows: 4 }} />
                <Input placeholder="待创作主题（按学习到的风格再创作）" value={learnForm.theme} onChange={(v) => setLearnForm({ ...learnForm, theme: v })} />
                <Button type="primary" long loading={loading} icon={<IconPen />}
                  onClick={() => run('深度学习再创作', api.learnCopy(learnForm))}>
                  深度学习再创作
                </Button>
              </Space>
            )}
            {sub === 'xiegou' && (
              <Space direction="vertical" style={{ width: '100%' }}>
                <TextArea placeholder="粘贴原文（AI 改写去重并保留原意）" value={xiegouText} onChange={setXiegouText} autoSize={{ minRows: 6 }} />
                <Button type="primary" long loading={loading} icon={<IconPen />}
                  onClick={() => run('洗稿改写', api.xiegou({ original: xiegouText }))}>
                  洗稿改写
                </Button>
                <Alert type="warning" content="洗稿仅作为创作辅助，请确保最终内容符合版权与平台规范。" />
              </Space>
            )}
          </Card>
        </Col>
        <Col span={14}>
          <Card
            size="small"
            title={result.title || '生成结果'}
            extra={
              result.text ? (
                <Space>
                  <Button size="mini" icon={<IconCopy />} onClick={copy}>复制</Button>
                  <Button size="mini" icon={<IconSave />} onClick={saveAsMaterial}>存素材库</Button>
                </Space>
              ) : null
            }
          >
            {result.platform && (
              <Space style={{ marginBottom: 8 }}>
                <Tag color="arcoblue" size="small">平台：{result.platform}</Tag>
                <Tag size="small">模型：{result.model}</Tag>
              </Space>
            )}
            <div style={{ whiteSpace: 'pre-wrap', minHeight: 260, color: 'var(--color-text-1)' }}>
              {result.text || (loading ? <Spin style={{ display: 'block', margin: '60px auto' }} /> : '填写左侧参数后点击生成')}
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
}

/* ================= 图片生成 ================= */

function ImageTab() {
  const [prompt, setPrompt] = useState('');
  const [size, setSize] = useState('1024x1024');
  const [loading, setLoading] = useState(false);
  const [img, setImg] = useState<{ b64?: string; url?: string; platform?: string; model?: string } | null>(null);
  const [error, setError] = useState('');

  const gen = async () => {
    if (!prompt.trim()) {
      Message.warning('请输入画面描述');
      return;
    }
    setLoading(true);
    setError('');
    setImg(null);
    try {
      const d = await api.genImage({ prompt, size });
      setImg({ b64: d.image_b64, url: d.image_url, platform: d.platform, model: d.model });
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  const download = () => {
    const target = img?.b64 || img?.url;
    if (!target) return;
    const a = document.createElement('a');
    if (img?.b64) {
      a.href = `data:image/png;base64,${img.b64}`;
    } else {
      a.href = target;
    }
    a.download = `creative_image_${Date.now()}.png`;
    a.click();
  };

  return (
    <Row gutter={16}>
      <Col span={10}>
        <Card size="small" title="文生图">
          <Space direction="vertical" style={{ width: '100%' }}>
            <TextArea placeholder="描述画面内容（主体/场景/风格/光线等，越具体效果越好）" value={prompt} onChange={setPrompt} autoSize={{ minRows: 5 }} />
            <Select value={size} onChange={setSize} style={{ width: '100%' }}>
              <Select.Option value="1024x1024">1024x1024（方图）</Select.Option>
              <Select.Option value="1024x1792">1024x1792（竖图）</Select.Option>
              <Select.Option value="1792x1024">1792x1024（横图）</Select.Option>
            </Select>
            <Button type="primary" long loading={loading} icon={<IconImage />} onClick={gen}>生成图片</Button>
            <Alert type="info" content="生成调用 OpenAI 兼容 images/generations 接口，基于左侧「AI 平台」配置的模型；生成结果自动保存到生成记录。" />
          </Space>
        </Card>
      </Col>
      <Col span={14}>
        <Card size="small" title="生成结果" extra={img && <Button size="mini" icon={<IconDownload />} onClick={download}>下载</Button>}>
          {error && <Alert type="error" content={error} />}
          {!img && !loading && !error && <Empty description="暂无生成的图片" style={{ padding: 60 }} />}
          {loading && <Spin style={{ display: 'block', margin: '60px auto' }} />}
          {img && !loading && (
            <div style={{ textAlign: 'center' }}>
              {img.b64 ? (
                <img src={`data:image/png;base64,${img.b64}`} alt="generated" style={{ maxWidth: '100%', maxHeight: 520, borderRadius: 8 }} />
              ) : (
                <img src={img.url} alt="generated" style={{ maxWidth: '100%', maxHeight: 520, borderRadius: 8 }} />
              )}
              <div style={{ marginTop: 8 }}>
                <Tag color="arcoblue" size="small">平台：{img.platform}</Tag>
                <Tag size="small">模型：{img.model}</Tag>
              </div>
            </div>
          )}
        </Card>
      </Col>
    </Row>
  );
}

/* ================= 素材库与生成记录 ================= */

function MaterialsTab() {
  const [tab, setTab] = useState('materials');
  const [materials, setMaterials] = useState<any[]>([]);
  const [records, setRecords] = useState<any[]>([]);
  const [recordKind, setRecordKind] = useState('');
  const [expanded, setExpanded] = useState<number | null>(null);

  const loadMaterials = useCallback(async () => {
    try {
      setMaterials(await api.listMaterials());
    } catch (e: any) {
      Message.error(e.message);
    }
  }, []);

  const loadRecords = useCallback(async () => {
    try {
      setRecords(await api.listRecords(recordKind));
    } catch (e: any) {
      Message.error(e.message);
    }
  }, [recordKind]);

  useEffect(() => { loadMaterials(); }, [loadMaterials]);
  useEffect(() => { if (tab === 'records') loadRecords(); }, [tab, loadRecords]);

  const delMaterial = async (id: number) => {
    try {
      await api.deleteMaterial(id);
      Message.success('已删除素材');
      await loadMaterials();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  const delRecord = async (id: number) => {
    try {
      await api.deleteRecord(id);
      Message.success('已删除记录');
      await loadRecords();
    } catch (e: any) {
      Message.error(e.message);
    }
  };

  return (
    <Tabs activeTab={tab} onChange={setTab} type="card" style={{ marginBottom: 16 }}>
      <Tabs.TabPane key="materials" title={`素材库（${materials.length}）`}>
        <Table
          rowKey="id" data={materials} pagination={false}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 60 },
            { title: '标题', dataIndex: 'title', width: 220, ellipsis: true },
            { title: '类型', dataIndex: 'kind', width: 100, render: (v: string) => kindTag(v) },
            { title: '内容', dataIndex: 'content', ellipsis: true },
            { title: '保存时间', dataIndex: 'created_at', width: 140, render: (v: string) => fmtTime(v) },
            {
              title: '操作', width: 90,
              render: (_: any, r: any) => (
                <Button size="mini" type="text" status="danger" icon={<IconDelete />} onClick={() => delMaterial(r.id)}>删除</Button>
              ),
            },
          ]}
        />
      </Tabs.TabPane>
      <Tabs.TabPane key="records" title={`生成记录（${records.length}）`}>
        <Space style={{ marginBottom: 12 }}>
          <Radio.Group value={recordKind} onChange={setRecordKind} type="button">
            <Radio value="">全部</Radio>
            <Radio value="copy">文案</Radio>
            <Radio value="script">抖音脚本</Radio>
            <Radio value="xhs">小红书</Radio>
            <Radio value="learn">深度学习</Radio>
            <Radio value="xiegou">洗稿</Radio>
            <Radio value="image">图片</Radio>
          </Radio.Group>
          <Button size="small" icon={<IconRefresh />} onClick={loadRecords}>刷新</Button>
        </Space>
        <Table
          rowKey="id" data={records} loading={tab === 'records' && records.length === 0 ? undefined : false} pagination={false}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 60 },
            { title: '类型', dataIndex: 'kind', width: 90, render: (v: string) => kindTag(v) },
            { title: '标题', dataIndex: 'title', width: 200, ellipsis: true },
            { title: '平台/模型', width: 160, render: (_: any, r: any) => r.platform ? `${r.platform} / ${r.model || '--'}` : '--' },
            {
              title: '内容预览 / 图片', render: (_: any, r: any) => {
                if (r.kind === 'image' && r.image_url) {
                  return <img src={r.image_url} alt="img" style={{ maxWidth: 96, borderRadius: 6 }} />;
                }
                return <Text style={{ wordBreak: 'break-all' }}>{String(r.output || '').slice(0, 90)}</Text>;
              },
            },
            { title: '状态', dataIndex: 'status', width: 90 },
            { title: '时间', dataIndex: 'created_at', width: 140, render: (v: string) => fmtTime(v) },
            {
              title: '操作', width: 90,
              render: (_: any, r: any) => (
                <Space>
                  {r.output && (
                    <Button size="mini" type="text" onClick={() => setExpanded(expanded === r.id ? null : r.id)}>
                      {expanded === r.id ? '收起' : '查看'}
                    </Button>
                  )}
                  <Button size="mini" type="text" status="danger" icon={<IconDelete />} onClick={() => delRecord(r.id)}>删除</Button>
                </Space>
              ),
            },
          ]}
          expandedRowRender={(r: any) =>
            (expanded === r.id && r.output) ? (
              <div style={{ whiteSpace: 'pre-wrap' }}>{r.output}</div>
            ) : null
          }
        />
      </Tabs.TabPane>
    </Tabs>
  );
}
