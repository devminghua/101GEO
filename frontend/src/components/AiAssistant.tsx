import { useState, useEffect, useRef, useCallback } from 'react';
import { Message, Tooltip, Spin } from '@arco-design/web-react';
import {
  IconRobot,
  IconClose,
  IconSend,
  IconRefresh,
  IconDelete,
  IconThunderbolt,
  IconCopy,
  IconHistory,
  IconLoading,
} from '@arco-design/web-react/icon';
import { api } from '../api';

/*
右侧悬浮 AI 数据分析助手
=================================================
客户每天面对一堆指标（AIVS 评分、出现率、引用率、缺口清单），但真正的问题是
「这些数字说明什么、我下一步做什么」——本组件把这个问题交给 AI，且 AI 看到的是
**本租户的真实数据**（后端 /api/assistant/* 注入），而不是泛泛的行业套话。

设计要点：
  1) 悬浮入口与既有「客服悬浮球」纵向错开，避免两个球叠在一起（客服 bottom:24，本组件 bottom:92）。
  2) 打开即展示数据快照条：让客户立刻确认「AI 看到的数据和我仪表盘一致」，建立可信感。
  3) 首屏给 6 个快捷提问，解决"不知道能问什么"（GEO 客户常见困境）。
  4) 逐字打字机呈现，与「智能创作中心」体验一致。
  5) 移动端（<560px）自动全屏，避免小屏挤压。
  6) 历史会话本地暂存（localStorage 最近一次会话 id）：客户关掉面板/刷新页面后
     再打开，仍能看到上次问过什么、AI 答过什么，不用重问一遍。
*/

interface Snapshot {
  brand: string;
  period: string;
  score: number;
  grade: string;
  grade_label: string;
  grade_color: string;
  delta: number;
  confidence: string;
  sample_count: number;
  dims: { name: string; score: number; weight: number }[];
  brand_rate: number;
  top3_rate: number;
  citation_rate: number;
  has_data: boolean;
}

interface Msg {
  id: number;
  role: 'user' | 'assistant';
  content: string;
}

interface SessionItem {
  id: number;
  title: string;
  message_count: number;
  updated_at: string;
}

const GRADE_COLOR: Record<string, string> = {
  green: '#00B42A',
  arcoblue: '#165DFF',
  orange: '#FF7D00',
  red: '#F53F3F',
};

const isMobile = () => typeof window !== 'undefined' && window.innerWidth < 560;

// 最近一次会话 id 的缓存键：面板收起/刷新后仍能回到上次的对话，不用重问
const LAST_SESSION_KEY = 'geo_ai_assistant_session';

export default function AiAssistant({ bottomOffset = 92 }: { bottomOffset?: number }) {
  const [open, setOpen] = useState(false);
  const [msgs, setMsgs] = useState<Msg[]>([]);
  const [input, setInput] = useState('');
  const [sessionId, setSessionId] = useState<number>(0);
  const [snap, setSnap] = useState<Snapshot | null>(null);
  const [quicks, setQuicks] = useState<any[]>([]);
  const [sending, setSending] = useState(false);
  const [loadingSnap, setLoadingSnap] = useState(false);
  const [typing, setTyping] = useState('');
  const [mobile, setMobile] = useState(isMobile());
  const [sessions, setSessions] = useState<SessionItem[]>([]);
  const [showHistory, setShowHistory] = useState(false);
  const [loadingHistory, setLoadingHistory] = useState(false);

  const boxRef = useRef<HTMLDivElement>(null);
  const typingRef = useRef<number>(0);
  const inputRef = useRef<any>(null);

  // 响应式：小屏全屏
  useEffect(() => {
    const onResize = () => setMobile(isMobile());
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  useEffect(() => () => window.clearTimeout(typingRef.current), []);

  // 打开面板时恢复上次会话：客户关掉再打开不用重问一遍
  const restoreLastSession = useCallback(async () => {
    let sid = 0;
    try {
      sid = Number(localStorage.getItem(LAST_SESSION_KEY) || 0);
    } catch {
      sid = 0;
    }
    if (!sid) return;
    try {
      const d: any = await api.assistantMessages(sid);
      const list: Msg[] = (d?.messages || []).map((m: any) => ({
        id: m.id,
        role: m.role === 'user' ? 'user' : 'assistant',
        content: m.content,
      }));
      if (list.length === 0) return;
      setSessionId(sid);
      setMsgs(list);
    } catch {
      // 会话可能已被删除，清掉本地缓存即可，不打扰用户
      try {
        localStorage.removeItem(LAST_SESSION_KEY);
      } catch {
        /* ignore */
      }
    }
  }, []);

  const loadSessions = useCallback(async () => {
    setLoadingHistory(true);
    try {
      const s: any = await api.assistantSessions();
      setSessions(s || []);
    } catch {
      setSessions([]);
    } finally {
      setLoadingHistory(false);
    }
  }, []);

  // 打开历史列表：显示并拉取最新列表
  const toggleHistory = () => {
    const next = !showHistory;
    setShowHistory(next);
    if (next) loadSessions();
  };

  // 点开某条历史会话
  const openSession = async (id: number) => {
    window.clearTimeout(typingRef.current);
    setTyping('');
    setShowHistory(false);
    try {
      const d: any = await api.assistantMessages(id);
      const list: Msg[] = (d?.messages || []).map((m: any) => ({
        id: m.id,
        role: m.role === 'user' ? 'user' : 'assistant',
        content: m.content,
      }));
      setSessionId(id);
      setMsgs(list);
      try {
        localStorage.setItem(LAST_SESSION_KEY, String(id));
      } catch {
        /* ignore */
      }
    } catch (e: any) {
      Message.error(e?.message || '读取会话失败');
    }
  };

  const removeSession = async (id: number) => {
    try {
      await api.assistantDeleteSession(id);
      setSessions((prev) => prev.filter((s) => s.id !== id));
      if (id === sessionId) {
        setSessionId(0);
        setMsgs([]);
      }
      try {
        if (Number(localStorage.getItem(LAST_SESSION_KEY) || 0) === id) {
          localStorage.removeItem(LAST_SESSION_KEY);
        }
      } catch {
        /* ignore */
      }
      Message.success('已删除');
    } catch (e: any) {
      Message.error(e?.message || '删除失败');
    }
  };

  // 首次打开时加载快照与快捷提问（缓存，避免每次开合重复请求）
  const loadSnapshot = useCallback(async () => {
    setLoadingSnap(true);
    try {
      const s: any = await api.assistantSnapshot();
      if (s) setSnap(s);
    } catch {
      /* 快照失败不阻塞对话，AI 侧仍会重新取数 */
    } finally {
      setLoadingSnap(false);
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    if (!snap) loadSnapshot();
    if (quicks.length === 0) {
      api.assistantQuickAsks().then((q) => setQuicks(q || [])).catch(() => {});
    }
    // 首次打开时恢复上次会话（有新消息说明已在对话中，不再覆盖）
    if (msgs.length === 0) restoreLastSession();
    // 聚焦输入框（延迟到动画结束）
    const t = window.setTimeout(() => inputRef.current?.focus?.(), 260);
    return () => window.clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, snap, quicks.length, loadSnapshot, restoreLastSession]);

  // 消息区自动滚底
  useEffect(() => {
    if (boxRef.current) boxRef.current.scrollTop = boxRef.current.scrollHeight;
  }, [msgs, typing, sending]);

  // 逐字输出：标点处停顿稍长，与创作中心一致
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
      const pause = /[。！？；，、.,;!?]/.test(ch) ? 80 : 14;
      typingRef.current = window.setTimeout(step, pause);
    };
    step();
  };

  const ask = async (question: string) => {
    const content = question.trim();
    if (!content || sending) return;
    window.clearTimeout(typingRef.current);
    setTyping('');
    setInput('');
    setShowHistory(false);
    const um: Msg = { id: Date.now(), role: 'user', content };
    const base = [...msgs, um];
    setMsgs(base);
    setSending(true);
    try {
      const data: any = await api.assistantChat({ session_id: sessionId, content });
      const sid = data?.session_id || sessionId;
      if (data?.session_id) setSessionId(data.session_id);
      if (data?.snapshot) setSnap(data.snapshot);
      const reply: string = data?.reply ? String(data.reply) : '';
      await new Promise<void>((resolve) => {
        typewriter(reply, (s) => setTyping(s), resolve);
      });
      setMsgs([...base, { id: Date.now() + 1, role: 'assistant', content: reply }]);
      setTyping('');
      // 记住本次会话，下次打开可回看
      try {
        if (sid) localStorage.setItem(LAST_SESSION_KEY, String(sid));
      } catch {
        /* ignore */
      }
    } catch (e: any) {
      setTyping('');
      setMsgs([...base, { id: Date.now() + 1, role: 'assistant', content: `⚠️ ${e.message || '分析失败，请重试'}` }]);
    } finally {
      setSending(false);
      window.setTimeout(() => inputRef.current?.focus?.(), 60);
    }
  };

  const resetChat = () => {
    window.clearTimeout(typingRef.current);
    setTyping('');
    setMsgs([]);
    setSessionId(0);
    setShowHistory(false);
    // 清掉本地会话指针，否则重开面板会又跳回刚才那条
    try {
      localStorage.removeItem(LAST_SESSION_KEY);
    } catch {
      /* ignore */
    }
    Message.success('已开始新对话');
  };

  const copyMsg = (t: string) => {
    navigator.clipboard?.writeText(t).then(() => Message.success('已复制'));
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      ask(input);
    }
  };

  const gradeColor = snap ? GRADE_COLOR[snap.grade_color] || '#165DFF' : '#4F46E5';
  const empty = msgs.length === 0 && !typing && !sending;

  return (
    <>
      {/* 悬浮入口：位于客服悬浮球（bottom:24 高度52）上方，纵向错开不重叠 */}
      {!open && (
        <div
          onClick={() => setOpen(true)}
          title="AI 数据分析助手"
          style={{
            position: 'fixed',
            right: 24,
            bottom: bottomOffset,
            width: 52,
            height: 52,
            borderRadius: '50%',
            background: 'linear-gradient(135deg,#4F46E5,#7B61FF)',
            color: '#fff',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 24,
            cursor: 'pointer',
            boxShadow: '0 6px 20px rgba(79,70,229,0.42)',
            userSelect: 'none',
            zIndex: 1001,
            transition: 'transform .18s',
          }}
          onMouseEnter={(e) => ((e.currentTarget as HTMLDivElement).style.transform = 'scale(1.06)')}
          onMouseLeave={(e) => ((e.currentTarget as HTMLDivElement).style.transform = 'scale(1)')}
        >
          <IconRobot />
        </div>
      )}

      {/* 对话面板 */}
      {open && (
        <div
          style={{
            position: 'fixed',
            right: mobile ? 0 : 24,
            bottom: mobile ? 0 : 24,
            top: mobile ? 0 : undefined,
            left: mobile ? 0 : undefined,
            width: mobile ? '100%' : 420,
            height: mobile ? '100%' : 'min(660px, calc(100vh - 48px))',
            background: 'var(--geo-surface, #fff)',
            borderRadius: mobile ? 0 : 16,
            boxShadow: '0 12px 48px rgba(0,0,0,0.22)',
            display: 'flex',
            flexDirection: 'column',
            overflow: 'hidden',
            zIndex: 1002,
            border: '1px solid var(--color-border-2)',
          }}
        >
          {/* 头部 */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '12px 14px',
              background: 'linear-gradient(135deg,#4F46E5,#7B61FF)',
              color: '#fff',
              flexShrink: 0,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 9, minWidth: 0 }}>
              <span
                style={{
                  width: 30,
                  height: 30,
                  borderRadius: 9,
                  background: 'rgba(255,255,255,0.2)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: 17,
                  flexShrink: 0,
                }}
              >
                <IconRobot />
              </span>
              <div style={{ minWidth: 0 }}>
                <div style={{ fontSize: 14.5, fontWeight: 600, lineHeight: 1.25 }}>AI 数据分析助手</div>
                <div style={{ fontSize: 11.5, opacity: 0.86, lineHeight: 1.35, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  基于你的真实数据解读优化效果
                </div>
              </div>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 4, flexShrink: 0 }}>
              <Tooltip content="历史会话">
                <span
                  onClick={toggleHistory}
                  style={{ ...iconBtn, background: showHistory ? 'rgba(255,255,255,0.28)' : 'transparent' }}
                >
                  <IconHistory />
                </span>
              </Tooltip>
              <Tooltip content="刷新数据">
                <span onClick={loadSnapshot} style={iconBtn}>
                  <IconRefresh />
                </span>
              </Tooltip>
              <Tooltip content="新对话">
                <span onClick={resetChat} style={iconBtn}>
                  <IconDelete />
                </span>
              </Tooltip>
              <Tooltip content="收起">
                <span onClick={() => setOpen(false)} style={iconBtn}>
                  <IconClose />
                </span>
              </Tooltip>
            </div>
          </div>

          {/* 数据快照条：让客户确认 AI 看到的数据与自己一致 */}
          <div
            style={{
              flexShrink: 0,
              padding: '9px 14px',
              background: 'var(--color-fill-1, #FAFAFA)',
              borderBottom: '1px solid var(--color-border-2)',
              fontSize: 12,
              color: 'var(--color-text-2)',
            }}
          >
            {loadingSnap && !snap ? (
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                <Spin size={12} /> 正在读取你的数据…
              </span>
            ) : snap && !snap.has_data ? (
              <span style={{ color: '#FF7D00', wordBreak: 'break-word' }}>
                ⚠ 近 7 天还没有巡检数据，先完成「添加关键词 → 配置 AI 平台 → 执行巡检」，再来分析效果。
              </span>
            ) : snap ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', minWidth: 0 }}>
                <span
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 4,
                    padding: '2px 8px',
                    borderRadius: 20,
                    background: gradeColor,
                    color: '#fff',
                    fontWeight: 600,
                    fontSize: 12,
                    flexShrink: 0,
                  }}
                >
                  AIVS {snap.score} · {snap.grade} {snap.grade_label}
                </span>
                <span style={{ flexShrink: 0 }}>
                  出现率 {snap.brand_rate}% · 引用率 {snap.citation_rate}%
                </span>
                {snap.delta !== 0 && (
                  <span style={{ color: snap.delta > 0 ? '#F53F3F' : '#00B42A', flexShrink: 0 }}>
                    {snap.delta > 0 ? '↑' : '↓'} {Math.abs(snap.delta)} 环比
                  </span>
                )}
                <span style={{ color: 'var(--color-text-3)', flexShrink: 0 }}>
                  {snap.sample_count} 样本 · 置信度{snap.confidence}
                </span>
              </div>
            ) : (
              <span>数据读取失败，可直接提问（AI 会重新取数）</span>
            )}
          </div>

          {/* 消息区（历史模式下替换为会话列表） */}
          <div ref={boxRef} style={{ flex: 1, overflowY: 'auto', padding: '14px 14px 6px', minHeight: 0 }}>
            {showHistory ? (
              <div>
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 10 }}>
                  历史会话（{sessions.length}）
                </div>
                {loadingHistory ? (
                  <div style={{ textAlign: 'center', padding: '26px 0', color: 'var(--color-text-3)', fontSize: 13 }}>
                    <IconLoading /> 加载中…
                  </div>
                ) : sessions.length === 0 ? (
                  <div style={{ textAlign: 'center', padding: '26px 0', color: 'var(--color-text-3)', fontSize: 13 }}>
                    还没有历史会话，先问一个问题吧
                  </div>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    {sessions.map((s) => (
                      <div
                        key={s.id}
                        onClick={() => openSession(s.id)}
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 8,
                          padding: '10px 12px',
                          borderRadius: 10,
                          border: `1px solid ${s.id === sessionId ? '#4F46E5' : 'var(--color-border-2)'}`,
                          background: 'var(--color-fill-1, #FAFAFA)',
                          cursor: 'pointer',
                        }}
                      >
                        <div style={{ flex: 1, minWidth: 0 }}>
                          <div
                            style={{
                              fontSize: 13,
                              color: 'var(--color-text-1)',
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {s.title || '未命名会话'}
                          </div>
                          <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 2 }}>
                            {s.message_count} 条 · {String(s.updated_at || '').slice(0, 16).replace('T', ' ')}
                          </div>
                        </div>
                        <Tooltip content="删除">
                          <span
                            onClick={(e) => {
                              e.stopPropagation();
                              removeSession(s.id);
                            }}
                            style={{ color: 'var(--color-text-3)', cursor: 'pointer', flexShrink: 0, fontSize: 13 }}
                          >
                            <IconDelete />
                          </span>
                        </Tooltip>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ) : empty ? (
              <div style={{ paddingTop: 4 }}>
                <div style={{ fontSize: 13, color: 'var(--color-text-2)', marginBottom: 12, lineHeight: 1.7 }}>
                  我可以帮你分析优化效果、找出问题原因、给出下一步建议。<br />
                  试试这些问题：
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {(quicks.length > 0 ? quicks : FALLBACK_QUICKS).map((q: any) => (
                    <div
                      key={q.key}
                      onClick={() => ask(q.question)}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 8,
                        padding: '10px 12px',
                        borderRadius: 10,
                        border: '1px solid var(--color-border-2)',
                        background: 'var(--color-fill-1, #FAFAFA)',
                        cursor: 'pointer',
                        fontSize: 13,
                        transition: 'all .15s',
                      }}
                      onMouseEnter={(e) => {
                        (e.currentTarget as HTMLDivElement).style.borderColor = '#4F46E5';
                        (e.currentTarget as HTMLDivElement).style.background = 'rgba(79,70,229,0.05)';
                      }}
                      onMouseLeave={(e) => {
                        (e.currentTarget as HTMLDivElement).style.borderColor = 'var(--color-border-2)';
                        (e.currentTarget as HTMLDivElement).style.background = 'var(--color-fill-1, #FAFAFA)';
                      }}
                    >
                      <IconThunderbolt style={{ color: '#4F46E5', flexShrink: 0 }} />
                      <span style={{ minWidth: 0, wordBreak: 'break-word' }}>{q.title}</span>
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <>
                {msgs.map((m) => (
                  <Bubble key={m.id} role={m.role} content={m.content} onCopy={copyMsg} />
                ))}
                {sending && !typing && (
                  <Bubble role="assistant" content="" thinking />
                )}
                {!!typing && <Bubble role="assistant" content={typing} typing />}
              </>
            )}
          </div>

          {/* 输入区 */}
          <div style={{ flexShrink: 0, borderTop: '1px solid var(--color-border-2)', padding: '10px 12px', background: 'var(--geo-surface, #fff)' }}>
            <div
              style={{
                display: 'flex',
                alignItems: 'flex-end',
                gap: 8,
                border: '1px solid var(--color-border-2)',
                borderRadius: 10,
                padding: '7px 9px',
                background: 'var(--color-fill-1, #FAFAFA)',
              }}
            >
              <textarea
                ref={inputRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={onKeyDown}
                placeholder={sending ? '正在分析…' : '问我任何关于优化效果的问题…'}
                disabled={sending}
                rows={1}
                style={{
                  flex: 1,
                  minWidth: 0,
                  border: 'none',
                  outline: 'none',
                  resize: 'none',
                  background: 'transparent',
                  fontSize: 13.5,
                  lineHeight: 1.6,
                  color: 'var(--color-text-1)',
                  maxHeight: 96,
                  fontFamily: 'inherit',
                }}
              />
              <button
                onClick={() => ask(input)}
                disabled={sending || !input.trim()}
                style={{
                  flexShrink: 0,
                  width: 32,
                  height: 32,
                  borderRadius: 8,
                  border: 'none',
                  cursor: sending || !input.trim() ? 'not-allowed' : 'pointer',
                  background: sending || !input.trim() ? 'var(--color-fill-3, #E5E6EB)' : '#4F46E5',
                  color: '#fff',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: 15,
                  transition: 'background .15s',
                }}
              >
                <IconSend />
              </button>
            </div>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 6, textAlign: 'center' }}>
              Enter 发送 · Shift+Enter 换行 · AI 分析仅供参考
            </div>
          </div>
        </div>
      )}
    </>
  );
}

const iconBtn: React.CSSProperties = {
  width: 28,
  height: 28,
  borderRadius: 8,
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  cursor: 'pointer',
  fontSize: 15,
  color: '#fff',
  transition: 'background .15s',
};

const FALLBACK_QUICKS = [
  { key: 'effect', title: '优化效果如何', question: '我最近的 GEO 优化效果怎么样？和前期相比是变好还是变差了？' },
  { key: 'reason', title: '为什么上不去', question: '我的品牌出现率和引用率为什么不高？最根本的原因是什么？' },
  { key: 'next', title: '先做哪三件事', question: '如果只让我做三件事来提升可见度，应该优先做哪三件？请给出具体操作步骤。' },
];

// Bubble 对话气泡：用户消息右对齐紫底，助手消息左对齐浅底
function Bubble({
  role,
  content,
  thinking,
  typing,
  onCopy,
}: {
  role: 'user' | 'assistant';
  content: string;
  thinking?: boolean;
  typing?: boolean;
  onCopy?: (t: string) => void;
}) {
  const isUser = role === 'user';
  return (
    <div style={{ display: 'flex', justifyContent: isUser ? 'flex-end' : 'flex-start', marginBottom: 12 }}>
      <div style={{ maxWidth: '86%', minWidth: 0 }}>
        <div
          style={{
            padding: '9px 12px',
            borderRadius: isUser ? '12px 12px 2px 12px' : '12px 12px 12px 2px',
            background: isUser ? '#4F46E5' : 'var(--color-fill-2, #F2F3F5)',
            color: isUser ? '#fff' : 'var(--color-text-1)',
            fontSize: 13.5,
            lineHeight: 1.72,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            overflowWrap: 'anywhere',
          }}
        >
          {thinking ? (
            <span style={{ color: 'var(--color-text-3)' }}>
              正在分析你的数据<span className="ai-assistant-caret" />
            </span>
          ) : (
            <>
              {content}
              {typing && <span className="ai-assistant-caret" />}
            </>
          )}
        </div>
        {!isUser && !thinking && !typing && content && (
          <div style={{ marginTop: 4, display: 'flex', gap: 10 }}>
            <span
              onClick={() => onCopy?.(content)}
              style={{ fontSize: 11.5, color: 'var(--color-text-3)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center', gap: 3 }}
            >
              <IconCopy /> 复制
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
