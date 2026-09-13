// 客户端功能模块清单：key 与后端 FeatureKeys 保持一致
export interface FeatureDef {
  key: string;
  label: string;
  route: string;
}

export const FEATURES: FeatureDef[] = [
  { key: 'dashboard', label: '仪表盘', route: '/dashboard' },
  { key: 'keywords', label: '关键词监控', route: '/keywords' },
  { key: 'platforms', label: 'AI 平台', route: '/platforms' },
  { key: 'tasks', label: '巡检任务', route: '/tasks' },
  { key: 'report', label: '生成报告', route: '/report' },
  { key: 'content', label: '内容投放', route: '/content' },
  { key: 'baidu', label: '百度优化', route: '/baidu-keywords' },
  { key: 'douyin', label: '抖音获客', route: '/douyin' },
  { key: 'xhs', label: '小红书获客', route: '/xhs' },
  { key: 'creation', label: '智能创作中心', route: '/creation' },
  { key: 'tools', label: '获客工具', route: '/tools/watermark' },
  { key: 'geo_intel', label: 'GEO 智能', route: '/geo-intel' },
  { key: 'settings', label: '系统设置', route: '/settings' },
];

export const ALL_FEATURE_KEYS = FEATURES.map((f) => f.key);

// 判断功能是否授权：features 为空/undefined 表示全部开放
export function hasFeature(features: string[] | undefined | null, key: string): boolean {
  if (!features || features.length === 0) return true;
  return features.includes(key);
}

export function featureLabel(key: string): string {
  return FEATURES.find((f) => f.key === key)?.label || key;
}
