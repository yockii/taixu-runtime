"""解析 Hacker News frontpage RSS，输出前 N 条标题 + 链接。

用法（生命体经 script.python 调用，或参考其逻辑）：
    python3 scripts/parse_hn.py
依赖：feedparser（baseline 白名单内）。
"""
import sys
import feedparser

FEED = "https://hnrss.org/frontpage"


def main(n: int = 10) -> None:
    d = feedparser.parse(FEED)
    for i, e in enumerate(d.entries[:n], 1):
        print(f"{i}. {e.title}\n   {e.link}")


if __name__ == "__main__":
    n = int(sys.argv[1]) if len(sys.argv) > 1 else 10
    main(n)
