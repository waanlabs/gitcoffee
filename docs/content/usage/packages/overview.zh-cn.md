---
date: "2021-07-20T00:00:00+00:00"
title: "软件包注册表"
slug: "overview"
sidebar_position: 1
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Overview"
    sidebar_position: 1
    identifier: "packages-overview"
---

# 软件包注册表

从Gitea **1.17**版本开始，软件包注册表可以用作常见软件包管理器的公共或私有注册表。

## 支持的软件包管理器

目前支持以下软件包管理器：

| Name                                                                | Language   | Package client            |
| ------------------------------------------------------------------- | ---------- | ------------------------- |
| [Alpine](usage/packages/alpine.md)       | -          | `apk`                     |
| [Cargo](usage/packages/cargo.md)         | Rust       | `cargo`                   |
| [Chef](usage/packages/chef.md)           | -          | `knife`                   |
| [Composer](usage/packages/composer.md)   | PHP        | `composer`                |
| [Conan](usage/packages/conan.md)         | C++        | `conan`                   |
| [Conda](usage/packages/conda.md)         | -          | `conda`                   |
| [Container](usage/packages/container.md) | -          | 任何符合OCI规范的客户端   |
| [CRAN](usage/packages/cran.md)           | R          | -                         |
| [Debian](usage/packages/debian.md)       | -          | `apt`                     |
| [Generic](usage/packages/generic.md)     | -          | 任何HTTP客户端            |
| [Go](usage/packages/go.md)               | Go         | `go`                      |
| [Helm](usage/packages/helm.md)           | -          | 任何HTTP客户端, `cm-push` |
| [Maven](usage/packages/maven.md)         | Java       | `mvn`, `gradle`           |
| [npm](usage/packages/npm.md)             | JavaScript | `npm`, `yarn`, `pnpm`     |
| [NuGet](usage/packages/nuget.md)         | .NET       | `nuget`                   |
| [Pub](usage/packages/pub.md)             | Dart       | `dart`, `flutter`         |
| [PyPI](usage/packages/pypi.md)           | Python     | `pip`, `twine`            |
| [RPM](usage/packages/rpm.md)             | -          | `yum`, `dnf`, `zypper`    |
| [RubyGems](usage/packages/rubygems.md)   | Ruby       | `gem`, `Bundler`          |
| [Swift](usage/packages/rubygems.md)      | Swift      | `swift`                   |
| [Vagrant](usage/packages/vagrant.md)     | -          | `vagrant`                 |

**以下段落仅适用于未全局禁用软件包的情况！**

## 仓库 x 软件包

软件包始终属于所有者（用户或组织），而不是仓库。
要将（已上传的）软件包链接到仓库，请打开该软件包的设置页面，并选择要将此软件包链接到的仓库。
将链接到整个软件包，而不仅是单个版本。

链接软件包将导致在仓库的软件包列表中显示该软件包，并在软件包页面上显示到仓库的链接（以及到仓库工单的链接）。

## 访问限制

| 软件包所有者类型 | 用户                                     | 组织                                       |
| ---------------- | ---------------------------------------- | ------------------------------------------ |
| **读取** 访问    | 公开，如果用户也是公开的；否则仅限此用户 | 公开，如果组织是公开的，否则仅限组织成员   |
| **写入** 访问    | 仅软件包所有者                           | 具有组织中的管理员或写入访问权限的组织成员 |

注意：这些访问限制可能会[变化](https://github.com/go-gitea/gitea/issues/19270)，将通过专门的组织团队权限添加更细粒度的控制。

## 创建或上传软件包

根据软件包类型，使用相应的软件包管理器。请查看特定软件包管理器的子页面以获取说明。

## 查看软件包

您可以在仓库页面上查看仓库的软件包。

1. 转到仓库主页。
2. 在导航栏中选择**软件包**

要查看有关软件包的更多详细信息，请选择软件包的名称。

## 下载软件包

要从仓库下载软件包：

1. 在导航栏中选择**软件包**
2. 选择软件包的名称以查看详细信息。
3. 在 **Assets** 部分，选择要下载的软件包文件的名称。

## 删除软件包

在将软件包发布到软件包注册表后，您无法编辑软件包。相反，您必须删除并重新创建它。

要从仓库中删除软件包：

1. 在导航栏中选择**软件包**
2. 选择软件包的名称以查看详细信息。
3. 单击**删除软件包**以永久删除软件包。

## 禁用软件包注册表

包注册表已自动启用。要在单个存储库中禁用它：

1. 在导航栏中选择**设置**。
2. 禁用**启用仓库软件包注册表**.

禁用软件包注册表不会删除先前发布的软件包。
