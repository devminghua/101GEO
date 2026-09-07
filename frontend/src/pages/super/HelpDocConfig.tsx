import { useState, useEffect, useRef } from 'react';
import { Card, Button, Message, Typography, Input, Popconfirm, Space, Tag, Empty } from '@arco-design/web-react';
import { IconPlus, IconEdit, IconDelete, IconFolder } from '@arco-design/web-react/icon';
import '@wangeditor/editor/dist/css/style.css';
import { Editor, Toolbar } from '@wangeditor/editor-for-react';
import { IDomEditor, IEditorConfig } from '@wangeditor/editor';
import { api } from '../../api';

// 帮助文档管理（SaaS 后台 super）：
// 左侧分类 + 文档目录，右侧富文本编辑（图文 + 粘贴 B 站视频链接自动嵌入）。
export default function HelpDocConfig() {
  const [cats, setCats] = useState<any[]>([]);
  const [docs, setDocs] = useState<any[]>([]);
  const [curCat, setCurCat] = useState<number | null>(null);
  const [curDoc, setCurDoc] = useState<any>(null); // 当前编辑的文档
  const [title, setTitle] = useState('');
  const [html, setHtml] = useState('');
  const [saving, setSaving] = useState(false);

  const [editingCat, setEditingCat] = useState<number | null>(null);
  const [editingCatName, setEditingCatName] = useState('');
  const [addingCat, setAddingCat] = useState(false);
  const [newCatName, setNewCatName] = useState('');

  const [editor, setEditor] = useState<IDomEditor | null>(null);
  const editorKeyRef = useRef(0);

  const loadCats = async () => {
    const c = await api.superHelpCategories();
    setCats(c || []);
  };
  const loadDocs = async (cid?: number) => {
    const d = await api.superHelpDocs(cid);
    setDocs(d || []);
  };
  useEffect(() => { loadCats(); loadDocs(); }, []);
  useEffect(() => () => { editor?.destroy(); }, [editor]);

  const selectCat = (id: number) => {
    setCurCat(id);
    setCurDoc(null);
    loadDocs(id);
  };

  const openDoc = async (doc: any) => {
    try {
      const d = await api.superHelpDoc(doc.id);
      setCurDoc(d);
      setTitle(d.title || '');
      setHtml(d.content || '');
      editorKeyRef.current += 1;
    } catch (e: any) { Message.error(e.message); }
  };
  const newDoc = () => {
    if (!curCat) { Message.warning('请先选择分类'); return; }
    setCurDoc({ id: 0, category_id: curCat, title: '', content: '' });
    setTitle('');
    setHtml('');
    editorKeyRef.current += 1;
  };

  const saveDoc = async () => {
    const t = title.trim();
    if (!t) { Message.warning('请输入文档标题'); return; }
    if (!curCat) { Message.warning('请选择分类'); return; }
    setSaving(true);
    try {
      await api.superHelpSaveDoc({ id: curDoc?.id || 0, category_id: curCat, title: t, content: html });
      Message.success('已保存');
      loadCats(); loadDocs(curCat);
      if (curDoc?.id) openDoc({ id: curDoc.id });
    } catch (e: any) { Message.error(e.message || '保存失败'); }
    finally { setSaving(false); }
  };

  const delDoc = async (id: number) => {
    await api.superHelpDeleteDoc(id);
    Message.success('已删除');
    setCurDoc(null);
    loadCats(); loadDocs(curCat);
  };

  const addCat = async () => {
    const n = newCatName.trim();
    if (!n) { Message.warning('请输入分类名'); return; }
    await api.superHelpCreateCategory(n);
    Message.success('已新增分类');
    setNewCatName(''); setAddingCat(false);
    loadCats();
  };
  const renameCat = async (id: number) => {
    const n = editingCatName.trim();
    if (!n) { Message.warning('请输入分类名'); return; }
    await api.superHelpUpdateCategory(id, n);
    Message.success('已改名');
    setEditingCat(null); setEditingCatName('');
    loadCats();
  };
  const delCat = async (id: number) => {
    await api.superHelpDeleteCategory(id);
    Message.success('已删除（含其下文档）');
    if (curCat === id) { setCurCat(null); setCurDoc(null); }
    loadCats(); loadDocs();
  };

  const editorConfig: Partial<IEditorConfig> = {
    placeholder: '请输入文档内容（支持图文 + 粘贴 B 站视频链接自动嵌入）...',
    MENU_CONF: {
      uploadImage: {
        async customUpload(file: File, insertFn: (url: string, alt: string, href: string) => void) {
          try {
            const d = await api.uploadImage(file, 'help');
            insertFn(d.url, '', '');
          } catch (e: any) { Message.error(e.message || '图片上传失败'); }
        },
      },
    },
  };

  return (
    <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
      {/* 左侧：分类 + 文档目录 */}
      <Card title="文档目录" style={{ borderRadius: 12, width: 300, flexShrink: 0 }} bodyStyle={{ padding: 12 }}>
        <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
          <Button size="mini" type="primary" icon={<IconPlus />} onClick={() => setAddingCat(true)}>新增分类</Button>
        </div>
        {addingCat && (
          <div style={{ display: 'flex', gap: 6, marginBottom: 8 }}>
            <Input size="small" value={newCatName} onChange={setNewCatName} placeholder="分类名" onPressEnter={addCat} />
            <Button size="mini" type="primary" onClick={addCat}>确定</Button>
            <Button size="mini" onClick={() => setAddingCat(false)}>取消</Button>
          </div>
        )}
        {cats.length === 0 && <Empty description="暂无分类，请先新增" />}
        {cats.map((c) => (
          <div key={c.id} style={{ marginBottom: 4 }}>
            <div
              style={{
                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                padding: '6px 8px', borderRadius: 6, cursor: 'pointer',
                background: curCat === c.id ? 'var(--color-primary-light-1)' : 'transparent',
              }}
              onClick={() => selectCat(c.id)}
            >
              {editingCat === c.id ? (
                <div style={{ display: 'flex', gap: 6, flex: 1 }}>
                  <Input size="mini" value={editingCatName} onChange={setEditingCatName} onPressEnter={() => renameCat(c.id)} />
                  <Button size="mini" type="primary" onClick={() => renameCat(c.id)}>√</Button>
                  <Button size="mini" onClick={() => setEditingCat(null)}>×</Button>
                </div>
              ) : (
                <>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontWeight: 600 }}>
                    <IconFolder /> {c.name} <Tag size="small">{c.doc_count || 0}</Tag>
                  </span>
                  <Space size={4}>
                    <Button size="mini" type="text" icon={<IconEdit />} onClick={(e) => { e.stopPropagation(); setEditingCat(c.id); setEditingCatName(c.name); }} />
                    <Popconfirm content="删除分类将同时删除其下所有文档，确认？" onOk={() => delCat(c.id)}>
                      <Button size="mini" type="text" status="danger" icon={<IconDelete />} onClick={(e) => e.stopPropagation()} />
                    </Popconfirm>
                  </Space>
                </>
              )}
            </div>
            {curCat === c.id && (
              <div style={{ paddingLeft: 16, marginTop: 2 }}>
                {docs.filter((d) => d.category_id === c.id).map((d) => (
                  <div
                    key={d.id}
                    style={{
                      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                      padding: '4px 8px', borderRadius: 4, cursor: 'pointer', fontSize: 13,
                      background: curDoc?.id === d.id ? 'var(--color-fill-2)' : 'transparent',
                    }}
                    onClick={() => openDoc(d)}
                  >
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{d.title}</span>
                    <Button size="mini" type="text" status="danger" icon={<IconDelete />} onClick={(e) => { e.stopPropagation(); delDoc(d.id); }} />
                  </div>
                ))}
                <Button size="mini" type="text" icon={<IconPlus />} style={{ marginTop: 4 }} onClick={newDoc}>新增文档</Button>
              </div>
            )}
          </div>
        ))}
      </Card>

      {/* 右侧：文档编辑 */}
      <Card
        title={curDoc ? '编辑文档' : '请选择左侧文档'}
        style={{ borderRadius: 12, flex: 1 }}
        extra={curDoc && <Button type="primary" loading={saving} onClick={saveDoc}>保存文档</Button>}
      >
        {!curDoc && (
          <Empty description={<span><Typography.Text type="secondary">请先在左侧选择分类与文档，或「新增文档」开始编写</Typography.Text></span>} />
        )}
        {curDoc && (
          <div>
            <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
              支持图文排版、图片上传；粘贴 B 站视频链接（如 https://www.bilibili.com/video/BV...）保存后，客户端会自动渲染成可播放的视频。
            </Typography.Text>
            <div style={{ marginBottom: 12 }}>
              <Typography.Text style={{ marginRight: 8 }}>标题</Typography.Text>
              <Input value={title} onChange={setTitle} placeholder="文档标题" style={{ maxWidth: 480 }} />
            </div>
            <div style={{ border: '1px solid var(--color-border-2)', borderRadius: 8, overflow: 'hidden' }}>
              <Toolbar editor={editor} defaultConfig={{}} mode="default" style={{ borderBottom: '1px solid var(--color-border-2)' }} />
              <Editor
                key={editorKeyRef.current}
                defaultConfig={editorConfig}
                value={html}
                onCreated={setEditor}
                onChange={(e) => setHtml(e.getHtml())}
                mode="default"
                style={{ height: 520, overflowY: 'hidden' }}
              />
            </div>
          </div>
        )}
      </Card>
    </div>
  );
}
