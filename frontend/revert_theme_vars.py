#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
回滚 apply_theme_vars.py 造成的语法损坏。

apply_theme_vars.py 的 lambda 取错了正则分组（部分规则的 prop 模式自带捕获组，
导致 group(1)/group(2) 错位），产出三类坏文本：
  A) 三元表达式 / 渐变里：'NoneNone'var(--xxx)'      —— 吞掉了开头的引号
  B) 普通属性：PropProp'var(--xxx)''                  —— 属性名重复 + 多一个引号
  C) 普通属性：prop: 'var(--xxx)''                    —— 多一个引号
本脚本按这三类逆变换，把文件还原成运行 apply_theme_vars.py 之前的样子。
"""
import os
import re
import sys

SRC = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'src')

# 变量 -> 原始十六进制（大小写无所谓，颜色一致）
HEX = {
    '--geo-surface': '#fff',
    '--geo-text': '#1d2129',
    '--color-text-2': '#4e5969',
    '--color-fill-2': '#f2f3f5',
    '--geo-surface-2': '#f7f8fa',
    '--geo-track-strong': '#e5e6eb',
    '--geo-tint-red': '#fff7f7',
    '--geo-tint-orange': '#fffbf4',
    '--geo-tint-amber': '#fff7e8',
    '--geo-tint-blue': '#f0f5ff',
    '--geo-tint-green': '#e8ffea',
}

PROPS = ['backgroundColor', 'borderColor', 'background', 'border', 'color', 'bg']

# A1) 渐变 / 值拼接处：前面没有引号 —— 直接换回原 hex
RE_A1 = re.compile(r"(?<!')NoneNone'var\((--[a-z0-9-]+)\)'")
# A2) 三元表达式：'NoneNone'var(--x)'  —— 连同前面那个被吞掉的引号一起还原成 '#hex'
RE_A2 = re.compile(r"'NoneNone'var\((--[a-z0-9-]+)\)'")
# B) 属性名重复 + 多余引号：backgroundbackground'var(--x)''  ->  background: 'var(--x)'
RE_B = re.compile(r"\b(" + '|'.join(PROPS) + r")\1'var\((--[a-z0-9-]+)\)''")
# C) 多余引号：'var(--x)''  ->  'var(--x)'
RE_C = re.compile(r"'var\((--[a-z0-9-]+)\)''")


def process(path):
    with open(path, 'r', encoding='utf-8') as f:
        src = f.read()
    orig = src

    src = RE_A1.sub(lambda m: HEX.get(m.group(1), m.group(0)), src)
    src = RE_A2.sub(lambda m: "'" + HEX.get(m.group(1), m.group(0)) + "'", src)
    src = RE_B.sub(lambda m: "%s: 'var(%s)'" % (m.group(1), m.group(2)), src)
    src = RE_C.sub(lambda m: "'var(%s)'" % m.group(1), src)

    if src != orig:
        with open(path, 'w', encoding='utf-8') as f:
            f.write(src)
        return True
    return False


def main():
    fixed = []
    for root, dirs, files in os.walk(SRC):
        dirs[:] = [d for d in dirs if d not in ('node_modules', '.git', 'dist')]
        for fn in files:
            if not fn.endswith(('.tsx', '.ts')):
                continue
            p = os.path.join(root, fn)
            if process(p):
                fixed.append(os.path.relpath(p, SRC))
    print('已回滚 %d 个文件：' % len(fixed))
    for f in sorted(fixed):
        print('  - ' + f)


if __name__ == '__main__':
    main()
