import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import zh from './zh';
import en from './en';
import ko from './ko';
import ja from './ja';

/* ================================================================
 * 多语言（中文 / English / 한국어 / 日本語）
 *  - 默认中文；用户切换后存 localStorage（geo_lang），下次进入自动恢复
 *  - 覆盖范围逐步扩展：框架层（菜单/顶栏/登录/用户菜单/渠道客户端）已全量翻译，
 *    业务页面文案随迭代补充（当前未翻译页面回落中文，不会白屏）
 * ================================================================ */

const LANG_KEY = 'geo_lang';

export const LANGS = [
  { code: 'zh', label: '中文', flag: '🇨🇳' },
  { code: 'en', label: 'English', flag: '🇺🇸' },
  { code: 'ko', label: '한국어', flag: '🇰🇷' },
  { code: 'ja', label: '日本語', flag: '🇯🇵' },
] as const;

export type LangCode = (typeof LANGS)[number]['code'];

export function currentLang(): LangCode {
  const v = localStorage.getItem(LANG_KEY);
  return (LANGS.some((l) => l.code === v) ? v : 'zh') as LangCode;
}

export function setLang(code: LangCode) {
  localStorage.setItem(LANG_KEY, code);
  i18n.changeLanguage(code);
}

i18n.use(initReactI18next).init({
  resources: {
    zh: { translation: zh },
    en: { translation: en },
    ko: { translation: ko },
    ja: { translation: ja },
  },
  lng: currentLang(),
  fallbackLng: 'zh',
  interpolation: { escapeValue: false },
});

export default i18n;
