# Redis Token 存储方案对比分析

## 概述

本文档分析多设备 Token 管理场景下，Redis String 和 Hash 两种存储方案的技术对比，以及 OpenIM 选择 Hash 方案的原因。

---

## 一、基本概念

### String 存储

**模型**：一个 Key 存储一个值

```
存储结构:
├─ Key: "token:user123:ios:token_aaa"  → Value: "0"
├─ Key: "token:user123:ios:token_bbb"  → Value: "0"
└─ Key: "token:user123:ios:token_ccc"  → Value: "2"

每个 token 独立存储，独立过期时间
```

### Hash 存储

**模型**：一个 Key 包含多个 Field-Value 对

```
存储结构:
Key: "token:user123:ios"
├─ Field: "token_aaa"  → Value: "0"
├─ Field: "token_bbb"  → Value: "0"
└─ Field: "token_ccc"  → Value: "2"

所有 token 共享一个 Key，共享过期时间
```

---

## 二、核心对比

### 2.1 查询性能

**场景**：获取用户在 iOS 平台的所有 token

| 方案 | 操作 | 复杂度 |
|------|------|--------|
| String | `KEYS` + N 次 `GET` | O(N) + N 次网络往返 |
| Hash | 1 次 `HGETALL` | O(1) 次网络往返 |

**性能差异**（查询 1000 用户，每用户 3 个 token）：
- String: 4000 次 Redis 操作
- Hash: 1000 次 Redis 操作
- **Hash 快 4 倍**

### 2.2 内存占用

**String 方案**：每个 Key 有独立元数据开销（~100 字节）

```
3 个 token: 3 × 141 = 423 字节
```

**Hash 方案**：多个 Field 共享元数据

```
3 个 token: 168 字节
```

**内存节省**：
| Token 数量 | 节省比例 |
|------------|----------|
| 3 个 | 60% |
| 10 个 | 83% |
| 100 个 | 91% |

**规模估算**（10,000 用户，每用户 2 设备）：
- String: 2.82 MB
- Hash: 1.57 MB
- **节省 1.25 MB (44%)**

### 2.3 批量操作

**场景**：强制登出用户（标记所有 token 为"被踢出"）

| 方案 | 操作数 |
|------|--------|
| String | 1 次 KEYS + 2N 次操作 |
| Hash | 2 次操作（HGETALL + HMSET）|

### 2.4 过期时间管理

| 方案 | 特点 |
|------|------|
| String | 每个 Key 独立 TTL，容易忘记设置导致内存泄漏 |
| Hash | 整个 Hash 共享 TTL，统一管理，不会漏设 |

---

## 三、Hash 的 TTL 刷新问题

### 3.1 问题场景

多设备登录时，活跃设备会刷新 TTL，可能导致不活跃设备的过期 token 一直保留在 Redis 中。

```
Day 1: 3 个设备登录，TTL = 90 天
Day 30: iPhone 活跃，创建新 token，TTL 刷新为 90 天
Day 91: iPad/MacBook 的 JWT 已过期，但 Redis Hash 还在
```

### 3.2 OpenIM 的解决方案

**双层过期机制**：

```
第一层: JWT ExpiresAt（主控）
├─ 创建时设置，永不改变
├─ 验证时首先检查
└─ 过期直接拒绝（不再查 Redis）

第二层: Redis TTL（兜底）
├─ 自动清理过期数据
├─ 防止内存泄漏
└─ 用户长期不活跃时清理
```

**验证流程**：
1. 解析 JWT，检查签名和 ExpiresAt
2. JWT 过期直接拒绝（不查 Redis）
3. JWT 有效才检查 Redis 状态

**关键结论**：
- 即使 Redis TTL 被刷新，过期的 JWT 仍然无法使用
- 但 Redis 中会积累过期的 JWT（需要清理）

### 3.3 过期 Token 的清理

OpenIM 在创建新 token 时清理同平台的过期 JWT：

```
创建新 token 流程:
1. 获取现有所有 token
2. 验证每个 JWT 是否过期
3. 删除过期的 JWT
4. 根据多设备策略决定踢出哪些
5. 创建新 token
```

---

## 四、TTL 刷新的必要性

### 4.1 如果不刷新 TTL

```
Day 1: iPhone 登录，TTL = 90 天
Day 50: iPad 登录（不刷新 TTL）
Day 91: Redis TTL 到期，整个 Hash 被删除
├─ iPhone 的 JWT 也过期了 ✓
└─ iPad 的 JWT 还有 49 天才过期 ✗
    └─ 用户必须重新登录（虽然 JWT 还有效）
```

**问题**：新创建的 token 可能因为旧的 TTL 而提前失效

### 4.2 OpenIM 刷新 TTL 的原因

1. **避免新 token 提前失效**
2. **活跃用户的便利性**
3. **简化实现**（统一规则：每次操作都刷新为 90 天）

---

## 五、优化方案

### 方案 1：智能 TTL 更新（推荐）

只在新 token 过期时间更晚时更新 TTL：

```
逻辑:
1. 创建新 token
2. 获取当前 Hash 的 TTL
3. 如果新 token 过期时间 > 当前 TTL：
   └─ 更新 TTL
4. 否则不更新
```

### 方案 2：完全不依赖 Redis TTL

依赖定期任务扫描和清理过期 JWT。

### 方案 3：混合方案（平衡）

TTL 作为兜底，定期清理过期 JWT，设置 TTL = 最晚 JWT 过期时间 + 缓冲。

---

## 六、适用场景总结

### String 存储适合

- 简单的 Key-Value 存储
- 每个值独立管理
- 不需要批量查询同组数据

### Hash 存储适合

- 一个对象有多个属性
- 需要频繁查询/更新部分字段
- 需要批量操作一组相关数据
- **多设备 Token 管理（OpenIM 场景）**

---

## 七、OpenIM Token 存储方案总结

### 为什么选择 Hash

1. **多设备登录特性**：一次性获取所有 token
2. **多设备策略需要批量查询**：Hash 比 String 快 4 倍
3. **批量操作频繁**：强制登出、清理过期 token
4. **内存优化**：规模越大，节省越明显

### 双层过期机制

| 层级 | 机制 | 作用 |
|------|------|------|
| 第一层 | JWT ExpiresAt | 主控，永不改变 |
| 第二层 | Redis TTL | 兜底清理，防止内存泄漏 |

### 关键结论

1. **Hash 比 String 更适合多设备 Token 管理**
2. **JWT 过期时间是主控，Redis TTL 是兜底**
3. **即使 TTL 刷新，过期 JWT 仍无法使用**
4. **过期 Token 需要清理**（创建新 token 时或定期任务）
