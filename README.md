# m-docker

这是我对容器技术基础的一些学习，并自己实现的一个简易版容器运行时，取名为 `m-docker`（my docker）.

不知道最后我会实现到哪个地步，也不知道会不会因为一些原因无法坚持下去。

但无论如何，做好记录，便是对学习的过程所表示的尊重。

# Contents

## 底层技术

- [namespace](./docs/basics/namespace/readme.md)

- [cgroup](./docs/basics/cgroup/readme.md)

- [UnionFS](./docs/basics/UnionFS/readme.md)

## 具体实现

### 1. 构造容器

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