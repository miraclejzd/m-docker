# m-docker

这是我对容器技术基础的一些学习，并自己实现的一个简易版容器运行时，取名为 `m-docker`（my docker）.

不知道最后我会实现到哪个地步，也不知道会不会因为一些原因无法坚持下去。

但无论如何，做好记录，便是对学习的过程所表示的尊重。

# 1. QuickStart

克隆仓库之后，进入项目根目录，执行以下命令：

```bash
make demo
```

这将构建 m-docker 二进制文件，并运行一个简单的容器。

# 2. Contents

## 2.1. 底层技术

- [namespace](./docs/basics/namespace/readme.md)

- [cgroup](./docs/basics/cgroup/readme.md)

- [UnionFS](./docs/basics/UnionFS/readme.md)

## 2.2. 具体实现

### 2.2.1. 构造简单容器

构造一个简单的容器，具有最基本的隔离与资源限制。

- [chapter1 - 实现 run 命令](./docs/source-analysis/chapter1-run命令实现.md)
  
  tag：**feat-run**

- [chapter2 - 优化：匿名管道传递参数](./docs/source-analysis/chapter2-匿名管道传递参数.md)

  tag：**perf-pipe**

- [chapter3 - 基于 cgroup 实现资源限制](./docs/source-analysis/chapter3-基于cgroup实现资源限制.md)
  
  tag：**feat-cgroup**

- [chapter4 - 使用 pivot_root 切换根文件系统](./docs/source-analysis/chapter4-使用pivot_root切换根文件系统.md)
  
  tag：**feat-rootfs**

- [chapter5 - 基于 overlay 联合挂载根文件系统](./docs/source-analysis/chapter5-基于overlay联合挂载根文件系统.md)

  tag: **feat-overlay**

### 2.2. 容器进阶

- [chapter6 - 重构！添加容器 Config](./docs/source-analysis/chapter6-重构！添加容器Config.md)
  
  tag: **refactor-config**

- [chapter7 - 重构！添加容器生命周期](./docs/source-analysis/chapter7-重构！添加容器生命周期.md)

  tag: **refactor-lifecycle**

- [chapter8 - 实现 -v 数据卷挂载](./docs/source-analysis/chapter8-实现-v数据卷挂载.md)

  tag: **feat-volume**

- [chapter9 - 实现 -d 后台运行](./docs/source-analysis/chapter9-实现-d后台运行.md)
  
  tag: **feat-detach**

- [chapter10 - ps 命令实现](./docs/source-analysis/chapter10-ps命令实现.md)
  
  tag: **feat-containers-list**

- [chapter11 - logs 命令实现](./docs/source-analysis/chapter11-logs命令实现.md)
  
  tag: **feat-logs**