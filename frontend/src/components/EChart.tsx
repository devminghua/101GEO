import { useEffect, useRef, useState } from 'react';
import * as echarts from 'echarts';

// 通用 ECharts 封装：懒加载（滚动到图表进入视口才 init + setOption 触发展开动画）+ 自动 resize。
export default function EChart({ option, height = 320, style }: { option: any; height?: number; style?: React.CSSProperties }) {
  const ref = useRef<HTMLDivElement>(null);
  const chartRef = useRef<echarts.ECharts | null>(null);
  const [inView, setInView] = useState(false);
  const firedRef = useRef(false);

  // 滚动监听：图表顶部进入视口（或初始就在视口内）才标记 inView
  useEffect(() => {
    const check = () => {
      const el = ref.current;
      if (!el || firedRef.current) return;
      const r = el.getBoundingClientRect();
      const vh = window.innerHeight || document.documentElement.clientHeight;
      // 图表顶部进入视口下方 80px 以内，且底部仍在视口上方（即图表可见）
      if (r.top < vh - 60 && r.bottom > 0) {
        firedRef.current = true;
        setInView(true);
        window.removeEventListener('scroll', check);
        window.removeEventListener('resize', check);
      }
    };
    check();
    window.addEventListener('scroll', check);
    window.addEventListener('resize', check);
    return () => {
      window.removeEventListener('scroll', check);
      window.removeEventListener('resize', check);
    };
  }, []);

  // inView 后：init + 首次 setOption（触发动画），合并同一 effect 保证时序
  useEffect(() => {
    if (!inView || !ref.current) return;
    const chart = echarts.init(ref.current);
    chartRef.current = chart;
    chart.setOption(option); // 默认 merge，触发展开动画
    const ro = new ResizeObserver(() => chart.resize());
    ro.observe(ref.current);
    return () => {
      ro.disconnect();
      chart.dispose();
      chartRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [inView]);

  // 数据变化时更新
  useEffect(() => {
    if (inView) chartRef.current?.setOption(option);
  }, [option, inView]);

  return <div ref={ref} style={{ width: '100%', height, ...style }} />;
}
