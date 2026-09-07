import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Card, Spin, Empty, Typography } from '@arco-design/web-react';
import { api } from '../api';

// 客户端「使用指南」：只展示文档内容（导航由侧栏「使用指南」下拉菜单承载）。
// B 站视频链接自动识别渲染为可播放的 iframe。
export default function HelpDoc() {
  const [params] = useSearchParams();
  const docParam = params.get('doc');
  const catParam = params.get('cat');
  const [curDoc, setCurDoc] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let alive = true;
    const load = async () => {
      setLoading(true);
      try {
        if (docParam) {
          const d = await api.helpDocDetail(Number(docParam));
          if (alive) setCurDoc(d);
        } else {
          // 未指定文档：定位到分类（cat 参数或第一个分类）下的第一篇文档
          const tree = await api.helpTree();
          const c = (catParam ? tree.find((x) => String(x.id) === catParam) : tree[0]) || tree[0];
          const first = c?.docs?.[0];
          if (first) {
            const d = await api.helpDocDetail(first.id);
            if (alive) setCurDoc(d);
          } else if (alive) {
            setCurDoc(null);
          }
        }
      } catch {
        if (alive) setCurDoc(null);
      } finally {
        if (alive) setLoading(false);
      }
    };
    load();
    return () => { alive = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [docParam, catParam]);

  const rendered = curDoc ? injectBilibili(curDoc.content || '') : '';

  return (
    <Card style={{ borderRadius: 12 }} bodyStyle={{ padding: '24px 32px' }}>
      {loading ? (
        <div style={{ textAlign: 'center', padding: '60px 0' }}><Spin /></div>
      ) : curDoc ? (
        <div>
          <Typography.Title heading={4} style={{ marginTop: 0 }}>{curDoc.title}</Typography.Title>
          <div className="help-doc-content" dangerouslySetInnerHTML={{ __html: rendered }} />
        </div>
      ) : (
        <Empty description={<span><Typography.Text type="secondary">该分类暂无文档，请在左侧菜单选择其他分类</Typography.Text></span>} />
      )}
    </Card>
  );
}

// B 站视频链接 → iframe
function biliIframe(bvid: string): string {
  return `<div class="bili-video" style="margin:12px 0;max-width:640px;">
    <iframe src="https://player.bilibili.com/player.html?bvid=${bvid}&autoplay=0&danmaku=0"
      scrolling="no" border="0" frameborder="no" framespacing="0" allowfullscreen="true"
      style="width:100%;aspect-ratio:16/9;border-radius:8px;border:none;"></iframe>
  </div>`;
}

// 把文档里的 B 站视频链接替换成 iframe
function injectBilibili(html: string): string {
  let out = html;
  out = out.replace(
    /<a[^>]*?href="[^"]*?bilibili\.com\/video\/(BV[0-9A-Za-z]+)[^"]*?"[^>]*>.*?<\/a>/g,
    (_m, bvid: string) => biliIframe(bvid),
  );
  out = out.replace(
    /https?:\/\/(?:www\.)?bilibili\.com\/video\/(BV[0-9A-Za-z]+)/g,
    (_m, bvid: string) => biliIframe(bvid),
  );
  return out;
}
