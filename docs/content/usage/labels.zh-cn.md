---
date: "2023-05-23T09:00:00+08:00"
title: "标签"
slug: "labels"
sidebar_position: 13
toc: false
draft: false
aliases:
  - /zh-cn/labels
menu:
  sidebar:
    parent: "usage"
    name: "标签"
    sidebar_position: 13
    identifier: "labels"
---

# 标签

您可以使用标签对工单和合并请求进行分类，并提高对它们的概览。

## 创建标签

对于仓库，可以在 `工单（Issues）` 中点击 `标签（Labels）` 来创建标签。

对于组织，您可以定义组织级别的标签，这些标签与所有组织仓库共享，包括已存在的仓库和新创建的仓库。可以在组织的 `设置（Settings）` 中创建组织级别的标签。

标签具有必填的名称和颜色，可选的描述，以及必须是独占的或非独占的（见下面的“作用域标签”）。

当您创建一个仓库时，可以通过使用 `工单标签（Issue Labels）` 选项来选择标签集。该选项列出了一些在您的实例上 [全局配置的可用标签集](../administration/customizing-gitea/#labels)。在创建仓库时，这些标签也将被创建。

## 作用域标签

作用域标签用于确保将至多一个具有相同作用域的标签分配给工单或合并请求。例如，如果标签 `kind/bug` 和 `kind/enhancement` 的独占选项被设置，那么工单只能被分类为 bug 或 enhancement 中的一个。

作用域标签的名称必须包含 `/`（不能在名称的任一端）。标签的作用域是基于最后一个 `/` 决定的，因此例如标签 `scope/subscope/item` 的作用域是 `scope/subscope`。

## 按标签筛选

工单和合并请求列表可以按标签进行筛选。选择多个标签将显示具有所有选定标签的工单和合并请求。

通过按住 alt 键并单击标签，可以将具有所选标签的工单和合并请求从列表中排除。
