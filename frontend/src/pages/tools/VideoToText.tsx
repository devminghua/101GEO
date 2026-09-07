import { useEffect, useRef, useState } from 'react';
import { Card, Button, Message, Alert, Space, Tag, Typography, Upload, Spin } from '@arco-design/web-react';
import { IconFile, IconCopy } from '@arco-design/web-react/icon';
import { api } from '../../api';

// 视频转文案：前端本地提取视频内嵌字幕（字幕轨道），不动用服务器流量。
// 扣费规则：Token > 2000 可用，每次解析扣 100 Token。
export default function VideoToText() {
  const [status, setStatus] = useState<any>(null);
  const [file, setFile] = useState<File | null>(null);
  const [videoUrl, setVideoUrl] = useState('');
  const [loading, setLoading] = useState(false);
  const [text, setText] = useState('');
  const videoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    api.toolStatus().then((s: any) => setStatus(s)).catch(() => {});
  }, []);

  const available = status?.available ?? false;
  const cost = status?.cost ?? 100;

  const onFile = (f: File) => {
    if (videoUrl) URL.revokeObjectURL(videoUrl);
    setFile(f);
    setVideoUrl(URL.createObjectURL(f));
    setText('');
  };

  const extract = async () => {
    if (!videoUrl) { Message.warning('请先上传视频文件'); return; }
    if (!available) { Message.warning(`获客工具需 Token > ${status?.min_balance ?? 2000}，请先充值`); return; }
    setLoading(true);
    try {
      await api.toolConsume('video2text');
      // 前端提取字幕轨道
      await new Promise((r) => setTimeout(r, 300)); // 等 video 元数据加载
      const video = videoRef.current;
      let collected = '';
      const tracks = video?.textTracks;
      if (tracks && tracks.length > 0) {
        // 遍历所有字幕轨道，拼接 cue 文本
        for (let i = 0; i < tracks.length; i++) {
          const track = tracks[i];
          for (let j = 0; j < track.cues?.length; j++) {
            const cue = track.cues[j] as any;
            const t = (cue?.text || '').trim();
            if (t && !collected.includes(t)) collected += (collected ? '\n' : '') + t;
          }
        }
      }
      if (collected) {
        setText(collected);
        Message.success('文案提取成功');
      } else {
        setText('');
        Message.warning('该视频未检测到内嵌字幕，无法提取文案。请上传带字幕的视频，或使用第三方语音转文字工具。');
      }
    } catch (e: any) {
      Message.error(e.message || '提取失败');
    } finally {
      setLoading(false);
    }
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      Message.success('文案已复制');
    } catch {
      Message.error('复制失败');
    }
  };

  return (
    <div>
      <Card style={{ marginBottom: 16, borderRadius: 12 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
          <div>
            <div style={{ fontSize: 16, fontWeight: 700 }}>视频转文案</div>
            <Typography.Text type="secondary" style={{ fontSize: 13 }}>
              上传本地视频，前端提取内嵌字幕文案，不占用服务器流量。
            </Typography.Text>
          </div>
          <Space>
            <Tag color={available ? 'green' : 'orange'}>
              {available ? `Token ${status?.balance}` : `Token ${status?.balance}（需 > ${status?.min_balance ?? 2000}）`}
            </Tag>
            <Tag color="arcoblue">每次 {cost} Token</Tag>
          </Space>
        </div>
        {!available && (
          <Alert style={{ marginTop: 12 }} type="warning" content={`获客工具需 Token 余额高于 ${status?.min_balance ?? 2000}，请先到「充值」页面充值。`} />
        )}
      </Card>

      <Card title="上传视频" style={{ marginBottom: 16, borderRadius: 12 }}>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Upload
            accept="video/*"
            showUploadList={false}
            beforeUpload={(f: File) => { onFile(f); return false; }}
          >
            <Button icon={<IconFile />} size="large">选择视频文件</Button>
          </Upload>
          {file && <Typography.Text style={{ fontSize: 13 }}>已选择：{file.name}（{(file.size / 1024 / 1024).toFixed(1)} MB）</Typography.Text>}
          {videoUrl && <video ref={videoRef} src={videoUrl} crossOrigin="anonymous" style={{ width: '100%', maxHeight: 280, borderRadius: 8 }} controls />}
          <Button type="primary" size="large" loading={loading} onClick={extract} disabled={!available || !videoUrl}>
            提取文案（扣 {cost} Token）
          </Button>
        </Space>
      </Card>

      {text && (
        <Card title="提取的文案" style={{ borderRadius: 12 }} extra={<Button size="small" icon={<IconCopy />} onClick={copy}>复制</Button>}>
          <div style={{ whiteSpace: 'pre-wrap', fontSize: 14, lineHeight: 1.8, background: 'var(--color-fill-2)', padding: 16, borderRadius: 8, maxHeight: 400, overflow: 'auto' }}>
            {text}
          </div>
        </Card>
      )}
    </div>
  );
}
