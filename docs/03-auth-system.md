# OpenIM 用户鉴权体系分析

## 概述

OpenIM 采用 **JWT Token + Redis 状态管理** 的混合鉴权架构，实现了灵活可控的 Token 管理。

```
JWT Token（无状态） + Redis 状态管理（有状态） = 灵活可控的 Token 管理
```

---

## 一、核心架构

### 1.1 主要模块位置

```
openim-server/
├── pkg/authverify/           # 权限验证工具包
│   └── token.go              # Token 验证和权限检查函数
├── pkg/common/storage/
│   ├── controller/auth.go    # Auth 核心业务逻辑
│   └── cache/redis/token.go  # Redis Token 缓存实现
├── internal/
│   ├── api/router.go         # 路由和 middleware
│   └── rpc/auth/auth.go      # Auth gRPC 服务实现
└── config/openim-rpc-auth.yml
```

### 1.2 Token 状态常量

```go
const (
    NormalToken  = 0      // 正常 token
    InValidToken = 1      // 无效 token
    KickedToken  = 2      // 被踢出 token
    ExpiredToken = 3      // 过期 token
)
```

---

## 二、Token 生成流程

### 2.1 核心流程

```go
func (a *authDatabase) CreateToken(ctx context.Context, userID string, platformID int) (string, error) {
    isAdmin := authverify.IsManagerUserID(userID, a.adminUserIDs)

    // 1. 非管理员需要检查多设备登录策略
    if !isAdmin {
        tokens, _ := a.cache.GetAllTokensWithoutError(ctx, userID)
        deleteTokenKey, kickedTokenKey, _ := a.checkToken(ctx, tokens, platformID)

        // 删除过期 token
        if len(deleteTokenKey) != 0 {
            a.cache.DeleteTokenByUidPid(ctx, userID, platformID, deleteTokenKey)
        }
        // 标记踢出 token
        for _, k := range kickedTokenKey {
            a.cache.SetTokenFlagEx(ctx, userID, platformID, k, constant.KickedToken)
        }
    }

    // 2. 构建 JWT Claims
    claims := tokenverify.BuildClaims(userID, platformID, a.accessExpire)

    // 3. 使用 HS256 签名
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, _ := token.SignedString([]byte(a.accessSecret))

    // 4. 存储到 Redis（管理员除外）
    if !isAdmin {
        a.cache.SetTokenFlagEx(ctx, userID, platformID, tokenString, constant.NormalToken)
    }

    return tokenString, nil
}
```

### 2.2 多设备登录策略

| 策略 | 说明 |
|------|------|
| `DefalutNotKick` | 不踢出，但限制最大数量 |
| `AllLoginButSameTermKick` | 相同设备类型只保留最新一个 |
| `PCAndOther` | PC 端和其他端分离，各保留一个 |
| `AllLoginButSameClassKick` | 相同平台类只保留一个 |

---

## 三、Token 验证体系

### 3.1 HTTP API 验证（Middleware）

```go
func GinParseToken(authClient *rpcli.AuthClient) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. 白名单检查
        if strings.HasPrefix(c.Request.URL.Path, wApi) {
            c.Next()
            return
        }

        // 2. 获取 token
        token := c.Request.Header.Get(constant.Token)
        if token == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, ...)
            return
        }

        // 3. 调用 Auth RPC 验证
        resp, err := authClient.ParseToken(c, token)
        if err != nil {
            c.AbortWithStatusJSON(http.StatusUnauthorized, ...)
            return
        }

        // 4. 设置 context
        c.Set(constant.OpUserID, resp.UserID)
        c.Set(constant.OpUserPlatform, constant.PlatformIDToName(int(resp.PlatformID)))
        c.Next()
    }
}
```

### 3.2 RPC 层验证

```go
func (s *authServer) parseToken(ctx context.Context, tokensString string) (*tokenverify.Claims, error) {
    // 1. 解析 JWT
    claims, err := tokenverify.GetClaimFromToken(tokensString, authverify.Secret(s.config.Share.Secret))
    if err != nil {
        return nil, err
    }

    // 2. 管理员 token 直接返回（不检查 Redis）
    if authverify.IsManagerUserID(claims.UserID, s.config.Share.IMAdminUserID) {
        return claims, nil
    }

    // 3. 普通用户检查 Redis 状态
    m, _ := s.authDatabase.GetTokensWithoutError(ctx, claims.UserID, claims.PlatformID)
    if len(m) == 0 {
        return nil, servererrs.ErrTokenNotExist.Wrap()
    }

    if v, ok := m[tokensString]; ok {
        switch v {
        case constant.NormalToken:
            return claims, nil
        case constant.KickedToken:
            return nil, servererrs.ErrTokenKicked.Wrap()
        default:
            return nil, errs.Wrap(errs.ErrTokenUnknown)
        }
    }

    return nil, servererrs.ErrTokenNotExist.Wrap()
}
```

---

## 四、Redis Token 存储

### 4.1 存储结构

```
类型: Hash
Key:  "UID_PID_TOKEN_STATUS:user_123:ios"
Field-Value 对:
{
    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...": 0,  // NormalToken
    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ8...": 2,  // KickedToken
}
TTL: 90 天
```

**为什么用 Hash**：
- 一个用户在同一平台可能有多个 token（多设备登录）
- Hash 可以方便地存储多个 token 及其状态
- 可以单独更新某个 token 的状态

### 4.2 核心操作

```go
// 设置 token 标记（带过期时间）
func (c *tokenCache) SetTokenFlagEx(ctx context.Context, userID string, platformID int, token string, flag int) error {
    key := cachekey.GetTokenKey(userID, platformID)
    c.rdb.HSet(ctx, key, token, flag)
    c.rdb.Expire(ctx, key, c.accessExpire)  // 刷新 TTL
    return nil
}

// 获取用户某平台的所有 token
func (c *tokenCache) GetTokensWithoutError(ctx context.Context, userID string, platformID int) (map[string]int, error) {
    m, _ := c.rdb.HGetAll(ctx, cachekey.GetTokenKey(userID, platformID)).Result()
    // 转换为 map[string]int
    return mm, nil
}
```

---

## 五、管理员 Token 特殊处理

### 5.1 与普通 Token 的区别

| 特性 | 普通用户 Token | 管理员 Token |
|------|---------------|-------------|
| 存储 | 存入 Redis | 不存 Redis |
| 验证 | JWT + Redis 检查 | 仅 JWT 验证 |
| 多设备 | 受策略限制 | 无限制 |
| 踢出 | 可通过 Redis 踢出 | 无法强制失效 |

### 5.2 设计原因

1. **管理员不受多设备登录限制**
2. **Redis 故障不影响管理员操作**
3. **减少 Redis 查询压力**
4. **管理员数量少，并发可控**

### 5.3 安全风险

- 管理员 token 泄露后无法通过 Redis 强制失效
- 只能等待 JWT 自然过期（90 天）
- **建议**：缩短管理员 token 有效期或添加黑名单机制

---

## 六、强制登出机制

### 6.1 强制登出流程

```go
func (s *authServer) forceKickOff(ctx context.Context, userID string, platformID int32) error {
    // 1. 断开 WebSocket 连接
    for _, conn := range s.RegisterCenter.GetConns(ctx, s.config.Share.RpcRegisterName.MessageGateway) {
        client := msggateway.NewMsgGatewayClient(conn)
        client.KickUserOffline(ctx, &msggateway.KickUserOfflineReq{
            KickUserIDList: []string{userID},
            PlatformID:     platformID,
        })
    }

    // 2. 标记所有 token 为 KickedToken（不删除）
    m, _ := s.authDatabase.GetTokensWithoutError(ctx, userID, int(platformID))
    for k := range m {
        m[k] = constant.KickedToken
    }
    s.authDatabase.SetTokenMapByUidPid(ctx, userID, int(platformID), m)

    return nil
}
```

### 6.2 为什么不删除而是标记

```
❌ 删除方案: "Token 不存在，请重新登录"
   → 用户不知道是被踢出还是 token 过期

✅ 标记方案: "您的账号已被强制登出"
   → 用户明确知道是被踢出，体验更好
```

---

## 七、双层过期机制

### 7.1 两层过期

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

### 7.2 Token 生命周期

```
1. 创建 Token
   ├─ 生成 JWT (有效期 90 天)
   └─ 存入 Redis: status = NormalToken, TTL = 90 天

2. 正常使用
   ├─ 每次请求验证 JWT 签名
   └─ 检查 Redis 状态

3. 被踢出
   ├─ 断开 WebSocket
   └─ Redis: status = KickedToken (不删除)

4. Token 过期
   ├─ JWT 过期: 验证直接拒绝
   └─ Redis TTL 到期: Key 自动删除
```

---

## 八、权限验证工具

```go
// 检查用户是否有权限访问资源（用户本人或管理员）
func CheckAccessV3(ctx context.Context, ownerUserID string, imAdminUserID []string) error {
    opUserID := mcontext.GetOpUserID(ctx)

    // 管理员有所有权限
    if datautil.Contain(opUserID, imAdminUserID...) {
        return nil
    }

    // 用户本人有权限
    if opUserID == ownerUserID {
        return nil
    }

    return servererrs.ErrNoPermission.WrapMsg("ownerUserID", ownerUserID)
}

// 检查是否是管理员
func CheckAdmin(ctx context.Context, imAdminUserID []string) error {
    if datautil.Contain(mcontext.GetOpUserID(ctx), imAdminUserID...) {
        return nil
    }
    return servererrs.ErrNoPermission.WrapMsg("user is not admin")
}
```

---

## 九、配置

### 9.1 Auth 服务配置

```yaml
# config/openim-rpc-auth.yml
tokenPolicy:
  expire: 90  # Token 有效期（天）
```

### 9.2 多设备登录配置

```yaml
share:
  secret: "your-jwt-secret-key-here"
  imAdminUserID:
    - "admin_001"
    - "admin_002"
  multiLogin:
    policy: 1  # AllLoginButSameTermKick
    maxNumOneEnd: 3
```

---

## 十、关键文件

| 文件 | 功能 |
|------|------|
| `pkg/common/storage/controller/auth.go` | Token 创建和验证核心逻辑 |
| `internal/rpc/auth/auth.go` | Auth gRPC 服务实现 |
| `pkg/authverify/token.go` | 权限验证工具函数 |
| `internal/api/router.go` | 路由配置和 GinParseToken 中间件 |
| `pkg/common/storage/cache/redis/token.go` | Redis Token 缓存实现 |

---

## 十一、总结

### 优点

- **双层验证机制**：JWT 保证性能，Redis 保证可控性
- **灵活的多设备登录策略**：支持 4 种策略
- **完善的强制登出机制**：WebSocket 断连 + Token 失效
- **状态标记设计**：被踢出的 token 标记而非删除，提供更好的用户体验

### 安全建议

1. **管理员 Token**：也应该可控（存 Redis），或缩短有效期
2. **GetAdminToken API**：应从白名单中移除或限制内网访问
3. **Token 刷新**：实现 Refresh Token 机制
4. **Redis 高可用**：哨兵模式或集群，避免单点故障
