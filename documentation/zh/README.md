---
description: 什么是Curio，它与Lotus-Miner有何不同？
---

# What is Curio?
# Curio是什么？

## Overview
## 概述

Curio是Filecoin存储协议的新实现。它旨在简化存储提供商的设置和操作。

{% hint style="danger" %}
请注意，Curio集群不能在不同的网络之间共享。

示例：单个Curio集群不能同时托管来自主网和校准网的矿工ID。
{% endhint %}

## Key Features
## 主要特性

#### High Availability
#### 高可用性

Curio设计用于高可用性。您可以运行多个Curio节点实例来处理类似类型的任务。分布式调度器和贪婪工作器设计将确保即使在大多数部分故障的情况下，任务也能按时完成。您可以安全地更新其中一台Curio机器，而不会中断其他机器的运行。

#### Node Heartbeat
#### 节点心跳

集群中的每个Curio节点必须每10分钟在HarmonyDB中发布一次心跳消息，更新其状态。如果错过心跳，该节点将被视为丢失，所有任务现在可以在剩余节点上调度。

#### Task Retry
#### 任务重试

Curio中的每个任务都有一个限制，即在被声明为丢失之前应该尝试多少次。这确保了Curio不会无限期地重试坏任务。这可以防止计算时间和存储的浪费。

#### Polling
#### 轮询

Curio通过轮询系统避免节点过载。节点检查它们可以处理的任务，优先考虑空闲节点以实现均衡的工作负载分配。

#### Simple Configuration Management
#### 简单的配置管理

配置以层的形式存储在数据库中。这些层可以堆叠在一起创建最终配置。用户可以重用这些层来控制多台机器的行为，而无需维护每个节点的配置。使用适当的标志启动二进制文件以连接YugabyteDB并指定使用哪些配置层以获得所需的行为。

#### Running Curio with Multiple GPUs
#### 使用多个GPU运行Curio

Curio可以同时处理多个GPU，而无需运行多个Curio进程实例。因此，Curio可以作为单个systemd服务进行管理，而无需担心GPU分配问题。

## Curio vs Lotus Miner
## Curio与Lotus Miner对比

| 特性                                 | Curio                                                          | Lotus-Miner                                         |
| ------------------------------------ | -------------------------------------------------------------- | --------------------------------------------------- |
| 调度                                 | 协作式（优先贪婪）                                             | 单点故障                                            |
| 高可用性                             | 可用                                                           | 单一控制进程                                        |
| 冗余Post                             | 可用                                                           | 不可用                                              |
| 任务重试控制                         | 任务重试有截止限制（每个任务）                                 | 无限重试导致资源耗尽                                |
| 多个矿工ID                           | Curio集群可以支持多个矿工ID                                    | 每个Lotus-Miner只有单个矿工ID                       |
| 共享任务节点                         | Curio节点可以处理多个矿工ID的任务                              | 附加的工作器只处理单个矿工ID的任务                  |
| 分布式配置管理                       | 配置存储在高可用的Yugabyte数据库中                             | 所有配置都在单个文件中                              |

## Future of Curio
## Curio的未来

Curio的长期愿景是最终取代当前的lotus-miner和lotus-worker进程。这是简化和精简存储提供商设置和操作的持续努力的一部分。

\
