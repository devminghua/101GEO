#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
把 LinkGeo 前端里「浅色模式下写死的表面色 / 文字色」替换成语义 CSS 变量，
让 body[arco-theme='dark'] 生效后整站能真正变深色。

只动「背景 / 边框 / 文字」三类属性上的白底与深字，
状态色与图表配色（#165DFF / #00B42A / #FF7D00 / #86909C ...）一律不动 ——
它们在深色背景下本来就是可读的。
"""
import os
import re
import sys

SRC = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'src')

# (属性名正则, 旧值正则, 新值)
RULES = [
    # --- 主表面：卡片 / 弹层 / 侧栏 / 顶栏 / 图表扇形描边 ---
    (r'(background|backgroundColor|bg|borderColor)', r'#fff|#FFF|#ffffff|#FFFFFF', 'var(--geo-surface)'),

    # --- 主文字 / 次级文字（深色下必须反转，否则看不见）---
    (r'color', r'#1d2129|#1D2129', 'var(--geo-text)'),
    (r'color', r'#4e5969|#4E5969', 'var(--color-text-2)'),

    # --- 浅灰填充块（标签底 / 代码块底 / 头像占位）---
    (r'background|bg', r'#f2f3f5|#F2F3F5', 'var(--color-fill-2)'),
    (r'background|bg', r'#f7f8fa|#F7F8FA', 'var(--geo-surface-2)'),

    # --- 进度条轨道 ---
    (r'background', r'#e5e6eb|#E5E6EB', 'var(--geo-track-strong)'),

    # --- 淡色告警块（深色下换成对应深色调）---
    (r'background|bg', r'#fff7f7|#FFF7F7', 'var(--geo-tint-red)'),
    (r'background|bg', r'#fffbf4|#FFFBF4', 'var(--geo-tint-orange)'),
    (r'background|bg', r'#fff7e8|#FFF7E8', 'var(--geo-tint-amber)'),
    (r'background|bg', r'#f0f5ff|#F0F5FF', 'var(--geo-tint-blue)'),
    (r'background|bg', r'#e8ffea|#E8FFEA', 'var(--geo-tint-green)'),
]

# 表格分隔线：'1px solid #F2F3F5' -> '1px solid var(--color-border-1)'
BORDER_RULE = (re.compile(r"(borderTop|borderBottom|border)\s*:\s*'(1px solid )#(f2f3f5|F2F3F5)'", re.I),
               r"\1: '\2var(--color-border-1)'")


def build(prop_pat, val_pat, new_val):
    # 匹配  prop: '#旧值'  ——  只匹配单引号包裹的整值（避免误伤 '#00B42A14' 这类带透明度的写法）
    return re.compile(r"(" + prop_pat + r")(\s*:\s*)'" + val_pat + r"'", re.I)


COMPILED = [(build(p, v, n), n, p) for p, v, n in RULES]


def process(path):
    with open(path, 'r', encoding='utf-8') as f:
        src = f.read()
    orig = src

    for rx, new_val, prop in COMPILED:
        src = rx.sub(lambda m: "%s%s'var(%s)'" % (m.group(1), m.group(2), new_val)
                     if new_val.startswith('--geo-') or new_val.startswith('--color-')
                     else "%s%s'%s'" % (m.group(1), m.group(2), new_val), src)

    src = BORDER_RULE[0].sub(BORDER_RULE[1], src)

    if src != orig:
        with open(path, 'w', encoding='utf-8') as f:
            f.write(src)
        return True
    return False


def main():
    changed = []
    for root, dirs, files in os.walk(SRC):
        dirs[:] = [d for d in dirs if d not in ('node_modules', '.git', 'dist')]
        for fn in files:
            if not fn.endswith(('.tsx', '.ts')):
                continue
            p = os.path.join(root, fn)
            if process(p):
                changed.append(os.path.relpath(p, SRC))

    print('已改写 %d 个文件：' % len(changed))
    for c in sorted(changed):
        print('  - ' + c)


if __name__ == '__main__':
    main()
