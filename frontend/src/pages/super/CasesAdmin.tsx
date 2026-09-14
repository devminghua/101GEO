import { useState, useEffect, useRef } from 'react';
import { Card, Button, Message, Input, Popconfirm, Space, Tag, Table, Modal, Switch, InputNumber, Upload } from '@arco-design/web-react';
import { IconPlus, IconEdit, IconDelete, IconUpload } from '@arco-design/web-react/icon';
import '@wangeditor/editor/dist/css/style.css';
import { Editor, Toolbar } from '@wangeditor/editor-for-react';
import { IDomEditor, IEditorConfig } from '@wangeditor/editor';
import { api } from '../../api';

/* ================================================================
 * 成功案例管理（SaaS 后台 super）：上传/编辑/删除平台级案例，
 * 发布后客户端「成功案例」页即时可见。
 * 正文富文本（图文，支持本地上传插图）；封面图支持本地上传或填 URL。
 * ================================================================ */

// 富文本编辑器配置：图片走自定义上传（api.uploadImage 带鉴权）
const editorConfig: Partial<IEditorConfig> = {
  MENU_CONF: {
    uploadImage: {
      async customUpload(file: File, insertFn: (url: string, alt: string, href: string) => void) {
        try {
          const d = await api.uploadImage(file, 'case');
          insertFn(d.url, '', '');
        } catch (e: any) {
          Message.error(e?.message || '图片上传失败');
        }
      },
    },
  },
};
export default function CasesAdmin() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [visible, setVisible] = useState(false);
  const [editing, setEditing] = useState<any>(null); // null=新建
  const [saving, setSaving] = useState(false);

  // 表单字段
  const [title, setTitle] = useState('');
  const [summary, setSummary] = useState('');
  const [coverURL, setCoverURL] = useState('');
  const [tags, setTags] = useState('');
  const [status, setStatus] = useState(1);
  const [sortOrder, setSortOrder] = useState(0);

  const [editor, setEditor] = useState<IDomEditor | null>(null);
  const [html, setHtml] = useState('');
  const editorKeyRef = useRef(0);

  const load = async () => {
    setLoading(true);
    try {
      const d: any = await api.superCases();
      setList(d || []);
    } catch {
      Message.error('加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { load(); }, []);
  useEffect(() => () => { editor?.destroy(); }, [editor]);

  const openNew = () => {
    setEditing(null);
    setTitle('');
    setSummary('');
    setCoverURL('');
    setTags('');
    setStatus(1);
    setSortOrder(0);
    setHtml('');
    editorKeyRef.current += 1;
    setVisible(true);
  };

  const openEdit = (c: any) => {
    setEditing(c);
    setTitle(c.title || '');
    setSummary(c.summary || '');
    setCoverURL(c.cover_url || '');
    setTags(c.tags || '');
    setStatus(c.status === 0 ? 0 : 1);
    setSortOrder(c.sort_order || 0);
    setHtml(c.content || '');
    editorKeyRef.current += 1;
    setVisible(true);
  };

  const save = async () => {
    const t = title.trim();
    if (!t) { Message.warning('请填写案例标题'); return; }
    setSaving(true);
    try {
      const body = { title: t, summary: summary.trim(), content: html, cover_url: coverURL.trim(), tags: tags.trim(), status, sort_order: sortOrder };
      if (editing?.id) {
        await api.superCaseUpdate(editing.id, body);
        Message.success('已更新');
      } else {
        await api.superCaseSave(body);
        Message.success('已发布/保存');
      }
      setVisible(false);
      load();
    } catch (e: any) {
      Message.error(e?.message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const remove = async (id: number) => {
    try {
      await api.superCaseDelete(id);
      Message.success('已删除');
      load();
    } catch (e: any) {
      Message.error(e?.message || '删除失败');
    }
  };

  return (
    <div style={{ padding: '20px 24px 40px', maxWidth: 1100, margin: '0 auto' }}>
      <Card
        title="成功案例管理"
        bordered={false}
        style={{ borderRadius: 16, boxShadow: '0 2px 14px rgba(0,0,0,.05)' }}
        extra={
          <Button type="primary" icon={<IconPlus />} onClick={openNew}>
            上传案例
          </Button>
        }
      >
        <Table
          rowKey="id"
          loading={loading}
          data={list}
          pagination={false}
          columns={[
            { title: '标题', dataIndex: 'title', ellipsis: true, render: (v) => <b>{v}</b> },
            {
              title: '状态', dataIndex: 'status', width: 100,
              render: (v) => <Tag color={v === 0 ? 'gray' : 'green'}>{v === 0 ? '草稿' : '已发布'}</Tag>,
            },
            { title: '标签', dataIndex: 'tags', width: 160, ellipsis: true, render: (v) => v || '—' },
            { title: '排序', dataIndex: 'sort_order', width: 70, render: (v) => v || 0 },
            { title: '更新时间', dataIndex: 'updated_at', width: 170, render: (v) => (v || '').replace('T', ' ').slice(0, 16) },
            {
              title: '操作', width: 150,
              render: (_, r) => (
                <Space>
                  <Button size="mini" type="text" icon={<IconEdit />} onClick={() => openEdit(r)}>编辑</Button>
                  <Popconfirm title="确认删除该案例？" onOk={() => remove(r.id)}>
                    <Button size="mini" type="text" status="danger" icon={<IconDelete />}>删除</Button>
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
        />
        <div style={{ marginTop: 12, fontSize: 12, color: '#86909c' }}>
          已发布（绿色）的案例客户端「成功案例」页立即可见；草稿仅此处可见。排序数字越大越靠前。
        </div>
      </Card>

      {/* 编辑弹窗 */}
      <Modal
        title={editing?.id ? '编辑成功案例' : '上传成功案例'}
        visible={visible}
        onCancel={() => setVisible(false)}
        onOk={save}
        okText={saving ? '保存中…' : '保存'}
        okButtonProps={{ loading: saving }}
        style={{ width: 720, maxWidth: '94vw' }}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="medium">
          <div>
            <div style={{ marginBottom: 6, fontSize: 13, color: '#4e5969' }}>案例标题（必填）</div>
            <Input value={title} onChange={setTitle} placeholder="如：深圳某婚恋机构 GEO 优化 3 个月关键词霸屏" maxLength={128} />
          </div>
          <div>
            <div style={{ marginBottom: 6, fontSize: 13, color: '#4e5969' }}>摘要（卡片列表展示，可选）</div>
            <Input.TextArea value={summary} onChange={setSummary} placeholder="一句话概括案例成果" maxLength={512} rows={2} />
          </div>
          <div>
            <div style={{ marginBottom: 6, fontSize: 13, color: '#4e5969' }}>封面图（本地上传或填 URL）</div>
            <Space style={{ width: '100%' }}>
              <Input value={coverURL} onChange={setCoverURL} placeholder="https://… 或点击右侧上传本地图片" style={{ flex: 1 }} />
              <Upload
                showUploadList={false}
                accept="image/*"
                customRequest={async (option: any) => {
                  try {
                    const d = await api.uploadImage(option.file as File, 'cover');
                    setCoverURL(d.url);
                    Message.success('封面上传成功');
                  } catch (e: any) {
                    Message.error(e?.message || '上传失败');
                  } finally {
                    option.onProgress?.(100);
                    option.onSuccess?.({});
                  }
                }}
              >
                <Button icon={<IconUpload />}>上传封面</Button>
              </Upload>
            </Space>
            {coverURL && (
              <div
                style={{
                  marginTop: 8,
                  height: 100,
                  borderRadius: 8,
                  background: `url(${coverURL}) center/cover no-repeat`,
                  backgroundColor: '#f7f8fa',
                  border: '1px solid var(--color-border-2)',
                }}
              />
            )}
          </div>
          <div>
            <div style={{ marginBottom: 6, fontSize: 13, color: '#4e5969' }}>标签（逗号分隔，可选）</div>
            <Input value={tags} onChange={setTags} placeholder="如：婚恋,GEO,深圳" maxLength={255} />
          </div>
          <Space size="large">
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 13, color: '#4e5969' }}>发布到客户端</span>
              <Switch checked={status === 1} onChange={(v) => setStatus(v ? 1 : 0)} />
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 13, color: '#4e5969' }}>排序权重</span>
              <InputNumber value={sortOrder} onChange={(v) => setSortOrder(Number(v) || 0)} min={0} max={999} style={{ width: 90 }} />
            </div>
          </Space>
          <div>
            <div style={{ marginBottom: 6, fontSize: 13, color: '#4e5969' }}>案例正文</div>
            <div style={{ border: '1px solid var(--color-border-2)', borderRadius: 8, overflow: 'hidden' }}>
              <Toolbar
                key={`tb-${editorKeyRef.current}`}
                editor={editor}
                defaultConfig={{}}
                mode="default"
                style={{ borderBottom: '1px solid var(--color-border-2)' }}
              />
              <Editor
                key={`ed-${editorKeyRef.current}`}
                defaultConfig={{ placeholder: '填写案例正文：客户背景、使用过程、效果数据…（支持插入图片）', ...editorConfig }}
                value={html}
                onCreated={setEditor}
                onChange={(e) => setHtml(e.getHtml())}
                mode="default"
                style={{ height: 260, overflowY: 'hidden' }}
              />
            </div>
          </div>
        </Space>
      </Modal>
    </div>
  );
}
