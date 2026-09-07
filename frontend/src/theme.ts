// 主题模式：浅色 / 深色
// ---------------------------------------------------------------
// 唯一权威入口：切 body 上的 arco-theme 属性 + html 的 color-scheme，
// 并把选择写进 localStorage（刷新后保持）。
// Arco 自带 body[arco-theme='dark'] 变量块，theme.css 补齐了它没覆盖的
// border 系列和 LinkGeo 自己的 --geo-* 语义变量。
//
// 业务组件不要再自己维护 themeMode state —— 用 getThemeMode() 读、
// setThemeMode() 写、subscribeTheme() 订阅。

export type ThemeMode = 'light' | 'dark';

const STORAGE_KEY = 'geotool-theme-mode';

/** 读取当前主题（优先 localStorage，缺省浅色） */
export function getThemeMode(): ThemeMode {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === 'dark' || v === 'light') return v;
  } catch {
    /* localStorage 不可用（隐私模式等）时忽略 */
  }
  return 'light';
}

type Listener = (mode: ThemeMode) => void;
const listeners = new Set<Listener>();

/** 订阅主题变化，返回取消订阅函数 */
export function subscribeTheme(fn: Listener): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

/** 应用主题：写 DOM 属性 + localStorage + 通知订阅者 */
export function setThemeMode(mode: ThemeMode): void {
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    /* 忽略 */
  }
  const body = document.body;
  const html = document.documentElement;
  if (mode === 'dark') {
    body.setAttribute('arco-theme', 'dark');
    html.setAttribute('arco-theme', 'dark');
  } else {
    body.removeAttribute('arco-theme');
    html.removeAttribute('arco-theme');
  }
  listeners.forEach((fn) => fn(mode));
}

/** 启动时恢复上次选择的主题（在 React 渲染前调用，避免闪白） */
export function initTheme(): ThemeMode {
  const mode = getThemeMode();
  // 直接写 DOM，不触发订阅（此时还没有订阅者）
  if (mode === 'dark') {
    document.body.setAttribute('arco-theme', 'dark');
    document.documentElement.setAttribute('arco-theme', 'dark');
  }
  return mode;
}

/** 在浅色 / 深色之间来回切 */
export function toggleTheme(): ThemeMode {
  const next: ThemeMode = getThemeMode() === 'dark' ? 'light' : 'dark';
  setThemeMode(next);
  return next;
}
