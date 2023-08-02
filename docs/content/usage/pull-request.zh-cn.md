---
date: "2018-06-01T19:00:00+02:00"
title: "合并请求"
slug: "pull-request"
sidebar_position: 13
toc: false
draft: false
aliases:
  - /zh-cn/pull-request
menu:
  sidebar:
    parent: "usage"
    name: "Pull Request"
    sidebar_position: 13
    identifier: "pull-request"
---

# 合并请求

## 在`合并请求`中使用“Work In Progress”标记

您可以通过在一个进行中的 pull request 的标题上添加前缀 `WIP:` 或者 `[WIP]`（此处大小写敏感）来防止它被意外合并，具体的前缀设置可以在配置文件 `app.ini` 中找到：

```
[repository.pull-request]
WORK_IN_PROGRESS_PREFIXES=WIP:,[WIP]
```

列表的第一个值将用于 helpers 程序。

## 合并请求模板

有关合并请求模板的更多信息请您移步 : [工单与合并请求模板](issue-pull-request-templates)
