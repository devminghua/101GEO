import React from 'react';
import ReactDOM from 'react-dom/client';
import { ConfigProvider } from '@arco-design/web-react';
import { HashRouter } from 'react-router-dom';
import zhCN from '@arco-design/web-react/es/locale/zh-CN';
import '@arco-design/web-react/dist/css/arco.css';
// 主题层：必须在 arco.css 之后加载（要覆盖它的 border 系列变量）
import './theme.css';
import { initTheme } from './theme';
import App from './App';

// 渲染前先把 arco-theme 属性落到 <body>/<html>，避免深色模式下首屏闪白
initTheme();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider locale={zhCN}>
      <HashRouter>
        <App />
      </HashRouter>
    </ConfigProvider>
  </React.StrictMode>
);
