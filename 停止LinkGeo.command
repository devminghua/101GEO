#!/bin/bash
# 停止 LinkGeo 服务
pkill -f "bin/geotool" 2>/dev/null
echo "✅ LinkGeo 已停止"
