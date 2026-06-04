---
name: hn-digest
description: |
  Fetch and summarize Hacker News top stories. Use when the life wants to
  catch up on tech/startup news or find new things to be curious about.
allowed-tools:
  - web.fetch
  - script.python
runtime:
  python: ">=3.12"
  deps:
    python:
      - feedparser
lanes:
  - deliberative
seed_version: "0.1.0"
---

# Hacker News Digest

帮生命体快速浏览 Hacker News 热门，从中发现可能感兴趣的新话题。

## 步骤

1. 用 `web.fetch` 抓 `https://news.ycombinator.com/` 或用 `script.python` + feedparser
   解析 RSS：`https://hnrss.org/frontpage`
2. 取前 10 条标题 + 链接
3. 挑 2-3 条最契合自己好奇心的，用 `web.fetch` 读正文
4. 若发现真正想深入的主题，用 `record_learning` 或在反思中标记为新兴趣

## 参考

- HN RSS: https://hnrss.org/frontpage
- 解析脚本见 `scripts/parse_hn.py`
