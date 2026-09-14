import { Dropdown, Menu, Button } from '@arco-design/web-react';
import { IconLanguage } from '@arco-design/web-react/icon';
import { useTranslation } from 'react-i18next';
import { LANGS, setLang, currentLang, type LangCode } from '../i18n';

/* ================================================================
 * 语言切换器（顶栏）：中文 / English / 한국어 / 日本語
 * 切换即生效 + localStorage 持久化，下次进入自动恢复
 * ================================================================ */
export default function LanguageSwitcher() {
  const { i18n } = useTranslation();
  const cur = currentLang();

  const droplist = (
    <Menu
      onClickMenuItem={(key) => {
        setLang(key as LangCode);
        i18n.changeLanguage(key as LangCode);
      }}
    >
      {LANGS.map((l) => (
        <Menu.Item key={l.code}>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
            <span>{l.flag}</span>
            <span style={{ fontWeight: cur === l.code ? 700 : 400 }}>{l.label}</span>
            {cur === l.code && <span style={{ marginLeft: 'auto', color: '#165DFF' }}>✓</span>}
          </span>
        </Menu.Item>
      ))}
    </Menu>
  );

  const curMeta = LANGS.find((l) => l.code === cur) || LANGS[0];

  return (
    <Dropdown droplist={droplist} trigger="click" position="br">
      <Button
        size="small"
        icon={<IconLanguage />}
        style={{ color: 'var(--geo-text)', borderColor: 'transparent', background: 'transparent' }}
        title="Language / 语言"
      >
        {curMeta.flag} {curMeta.label}
      </Button>
    </Dropdown>
  );
}
