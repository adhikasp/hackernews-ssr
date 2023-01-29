# Hackernews SSR

A Server Side Rendered (SSR) mirror of [Hacker News](https://news.ycombinator.com/).

Hosted in https://hn.adhikasp.my.id.

Database is fetched with [adhikasp/hackernews-scrape](https://github.com/adhikasp/hackernews-scrape).

## Analytics from access logs

Use [angle-grinder](https://github.com/rcoh/angle-grinder/) to analyze logs.

Get most active IP

```sh
cat access.log | agrind '* | json | count by ip'
```

Get most popular story/item

```sh
cat access.log | agrind '* | json | where item_id != "" | count by item_id'
```

Get latency 

```sh
cat access.log | agrind '* | json | where !isNull(latency) | avg(latency), p50(latency), p95(latency) by path'
```