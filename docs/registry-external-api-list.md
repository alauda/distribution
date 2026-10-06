# Registry 对外暴露 API 清单

本文整理当前仓库中 `registry` 进程对外暴露的 HTTP API，包括：

- 每个 API 的完整路由
- 支持的请求方法
- API 的用途
- 具体实现代码位置
- 常见调用方式

说明：

- 本文以当前仓库代码实现为准
- 既包含 Registry V2 主数据面接口，也包含进程额外暴露的运维类接口

主要参考代码：

- `registry/handlers/app.go`
- `registry/api/v2/routes.go`
- `registry/api/v2/descriptors.go`
- `registry/registry.go`
- `health/health.go`

## 占位符说明

- `<registry>`：registry 地址，例如 `registry.example.com` 或 `127.0.0.1:5000`
- `<name>`：仓库名，例如 `library/busybox` 或 `team/app`
- `<reference>`：tag 或 digest，例如 `latest` 或 `sha256:...`
- `<digest>`：blob 或 manifest 的 digest，例如 `sha256:...`
- `<uuid>`：上传会话 ID，由上传初始化接口返回

如果 registry 开启了鉴权，实际调用时还需要带 `Authorization` 请求头。下面示例主要展示路由形态和调用方式。

## 1. Registry V2 API 总览

| 完整路由 | 方法 | 用途 | 主要实现代码 | OCI Conformance 是否覆盖 |
| --- | --- | --- | --- | --- |
| `/v2/` | `GET` | 检查 Registry V2 接口是否可用 | `registry/handlers/app.go:956` | 是 |
| `/v2/<name>/tags/list` | `GET` | 列出仓库下的 tag | `registry/handlers/tags.go:36` | 是 |
| `/v2/<name>/manifests/<reference>` | `GET`、`HEAD`、`PUT`、`DELETE` | 查询、上传、探测、删除 manifest 或 tag 引用 | `registry/handlers/manifests.go:47` | 是 |
| `/v2/<name>/referrers/<digest>` | `GET` | 查询某个 subject digest 的 referrers | `registry/handlers/referrers.go:42` | 是 |
| `/v2/<name>/blobs/<digest>` | `GET`、`HEAD`、`DELETE` | 查询、下载、探测、删除 blob | `registry/handlers/blob.go:14` | 是 |
| `/v2/<name>/blobs/uploads/` | `POST` | 初始化 blob 上传，或尝试跨仓库 mount | `registry/handlers/blobupload.go:67` | 是 |
| `/v2/<name>/blobs/uploads/<uuid>` | `GET`、`HEAD`、`PATCH`、`PUT`、`DELETE` | 查询上传状态、续传、完成上传、取消上传 | `registry/handlers/blobupload.go:107` | 是 |
| `/v2/_catalog` | `GET` | 列出 registry 中的仓库 | `registry/handlers/catalog.go:36` | 否；仓库内由 `registry/handlers/api_test.go:84` 和 `registry/storage/catalog_test.go:106` 等覆盖 |

路由注册入口：

- `registry/handlers/app.go:106`
- 路由名定义：`registry/api/v2/routes.go:11`
- 路由详细描述：`registry/api/v2/descriptors.go:398`

## 2. 各 API 详细说明

### 2.1 Base API

- 完整路由：`/v2/`
- 方法：`GET`
- 用途：
  - 判断目标服务是否实现了 Registry V2 / OCI Distribution API
  - 很多客户端在登录、push、pull 前会先访问这个接口
- 返回：
  - 返回一个简单的 JSON：`{}`
- 代码位置：
  - 路由注册：`registry/handlers/app.go:106`
  - 处理函数：`registry/handlers/app.go:956`
  - 描述定义：`registry/api/v2/descriptors.go:400`
- OCI Conformance 覆盖：
  - 是否覆盖：是
  - 对应 case：`ping`
  - 参考：`tests/oci-conformance-case-checklist.md:95`
- 调用示例：

```bash
curl -i http://<registry>/v2/
```

### 2.2 Tags API

- 完整路由：`/v2/<name>/tags/list`
- 方法：`GET`
- 用途：
  - 列出某个仓库下的所有 tag
  - 支持分页查询
- 查询参数：
  - `n`：每页数量
  - `last`：分页游标
- 代码位置：
  - 路由注册：`registry/handlers/app.go:112`
  - dispatcher：`registry/handlers/tags.go:15`
  - 处理函数：`registry/handlers/tags.go:36`
  - 描述定义：`registry/api/v2/descriptors.go:434`
- OCI Conformance 覆盖：
  - 是否覆盖：是
  - 直接 case：`empty -> tag list`
  - 在大量镜像/索引/artifact case 中也会反复覆盖 `tag-list`
  - 典型分组：`image`、`image-uncompressed`、`large-manifest`、`index`、`nested-index`、`empty-index`、`artifact`、`artifact-index`、`artifact-without-layers`、`artifacts-with-subject`、`index-with-subject`、`missing-subject`、`data-field`、`non-distributable-layers`、`custom-fields`、`no-layers`、`sha512`、`bad-digest-image`
  - 参考：`tests/oci-conformance-case-checklist.md:101`、`tests/oci-conformance-case-checklist.md:304`
- 调用示例：

```bash
curl -i http://<registry>/v2/<name>/tags/list
curl -i "http://<registry>/v2/<name>/tags/list?n=20&last=v1.2.0"
```

### 2.3 Manifest API

- 完整路由：`/v2/<name>/manifests/<reference>`
- 方法：
  - `GET`
  - `HEAD`
  - `PUT`
  - `DELETE`
- 用途：
  - `GET`：获取 manifest 内容
  - `HEAD`：只看 manifest 元信息，不返回 body
  - `PUT`：上传 manifest
  - `DELETE`：删除 manifest，或者删除某个 tag 引用
- 承载的 OCI 语义：
  - image manifest
  - image index
  - artifact manifest
  - `subject`
  - `artifactType`
- 代码位置：
  - 路由注册：`registry/handlers/app.go:109`
  - dispatcher：`registry/handlers/manifests.go:47`
  - `GET` / `HEAD`：`registry/handlers/manifests.go:88`
  - `PUT`：`registry/handlers/manifests.go:257`
  - `DELETE`：`registry/handlers/manifests.go:464`
  - 描述定义：`registry/api/v2/descriptors.go:527`
- OCI Conformance 覆盖：
  - 是否覆盖：是
  - `GET` / `HEAD` 典型 case：`manifest-by-tag`、`manifest-by-digest`、`manifest-head-by-tag`、`manifest-head-by-digest`
  - `PUT` 典型 case：`manifest-by-tag`、`manifest-by-digest`、`manifest-put`
  - `DELETE` 典型 case：`manifest-delete`、`tag-delete`
  - 主要覆盖分组：`image`、`image-uncompressed`、`large-manifest`、`index`、`nested-index`、`empty-index`、`artifact`、`artifact-index`、`artifact-without-layers`、`artifacts-with-subject`、`index-with-subject`、`missing-subject`、`data-field`、`non-distributable-layers`、`custom-fields`、`no-layers`、`sha512`、`bad-digest-image`、`missing-manifest`、`invalid-digest-format`
  - 与 OCI 语义强相关的专项分组：`index`、`nested-index`、`empty-index`、`artifact`、`artifact-index`、`artifact-without-layers`、`artifacts-with-subject`、`index-with-subject`、`missing-subject`、`non-distributable-layers`、`sha512`、`invalid-digest-format`
  - 参考：`tests/oci-conformance-case-checklist.md:295`、`tests/oci-conformance-case-checklist.md:385`、`tests/oci-conformance-case-checklist.md:535`、`tests/oci-conformance-case-checklist.md:621`、`tests/oci-conformance-case-checklist.md:693`、`tests/oci-conformance-case-checklist.md:999`、`tests/oci-conformance-case-checklist.md:1007`
- 调用示例：

```bash
curl -i -H "Accept: application/vnd.oci.image.manifest.v1+json" \
  http://<registry>/v2/<name>/manifests/latest

curl -I http://<registry>/v2/<name>/manifests/sha256:<digest>

curl -i -X PUT \
  -H "Content-Type: application/vnd.oci.image.manifest.v1+json" \
  --data-binary @manifest.json \
  http://<registry>/v2/<name>/manifests/latest

curl -i -X DELETE http://<registry>/v2/<name>/manifests/sha256:<digest>
```

### 2.4 Referrers API

- 完整路由：`/v2/<name>/referrers/<digest>`
- 方法：`GET`
- 用途：
  - 查询哪些 manifest 或 index 引用了指定的 subject digest
  - OCI artifact / subject / referrers 能力主要依赖这个接口
- 查询参数：
  - `artifactType`：可选过滤条件
- 代码位置：
  - 路由注册：`registry/handlers/app.go:110`
  - dispatcher：`registry/handlers/referrers.go:18`
  - 处理函数：`registry/handlers/referrers.go:42`
  - 描述定义：`registry/api/v2/descriptors.go:732`
- OCI Conformance 覆盖：
  - 是否覆盖：是
  - 直接 case：`empty -> referrers`
  - subject/referrers 相关 case：`artifacts-with-subject -> referrers`、`index-with-subject -> referrers`、`missing-subject -> referrers`
  - 参考：`tests/oci-conformance-case-checklist.md:101`、`tests/oci-conformance-case-checklist.md:674`、`tests/oci-conformance-case-checklist.md:752`、`tests/oci-conformance-case-checklist.md:793`
- 调用示例：

```bash
curl -i http://<registry>/v2/<name>/referrers/sha256:<digest>
curl -i "http://<registry>/v2/<name>/referrers/sha256:<digest>?artifactType=application/vnd.cncf.notary.signature"
```

### 2.5 Blob API

- 完整路由：`/v2/<name>/blobs/<digest>`
- 方法：
  - `GET`
  - `HEAD`
  - `DELETE`
- 用途：
  - `GET`：下载 blob 内容
  - `HEAD`：探测 blob 是否存在、查看元信息
  - `DELETE`：删除 blob
- 说明：
  - `GET` 可能直接返回内容，也可能重定向到后端存储地址
  - 支持 range 请求时可以分段拉取
- 代码位置：
  - 路由注册：`registry/handlers/app.go:113`
  - dispatcher：`registry/handlers/blob.go:14`
  - `GET` / `HEAD`：`registry/handlers/blob.go:55`
  - `DELETE`：`registry/handlers/blob.go:76`
  - 描述定义：`registry/api/v2/descriptors.go:784`
- OCI Conformance 覆盖：
  - 是否覆盖：是
  - 直接 blob 能力分组：`sha256 blobs`、`sha512 blobs`
  - 典型 case：`blob-head`、`blob-get`、`blob-delete`、`range 500-1499`、`range 500-`、`range -500`、`range 2000-5000`、`range 500-0`、`range 5000-10000`
  - 在镜像/索引/artifact 分组中也会反复覆盖 blob 拉取、探测、删除
  - 参考：`tests/oci-conformance-case-checklist.md:109`、`tests/oci-conformance-case-checklist.md:202`、`tests/oci-conformance-case-checklist.md:265`
- 调用示例：

```bash
curl -i http://<registry>/v2/<name>/blobs/sha256:<digest>
curl -I http://<registry>/v2/<name>/blobs/sha256:<digest>
curl -i -H "Range: bytes=0-1023" http://<registry>/v2/<name>/blobs/sha256:<digest>
curl -i -X DELETE http://<registry>/v2/<name>/blobs/sha256:<digest>
```

### 2.6 Blob Upload Start API

- 完整路由：`/v2/<name>/blobs/uploads/`
- 方法：`POST`
- 用途：
  - 初始化一个可续传的 blob 上传会话
  - 支持单次上传整个 blob
  - 支持跨仓库 mount 已存在 blob
- 支持方式：
  - 普通初始化上传
  - `?digest=<digest>`：单请求上传完成
  - `?mount=<digest>&from=<repo>`：跨仓库 mount
- 代码位置：
  - 路由注册：`registry/handlers/app.go:114`
  - dispatcher：`registry/handlers/blobupload.go:21`
  - 处理函数：`registry/handlers/blobupload.go:67`
  - 描述定义：`registry/api/v2/descriptors.go:1018`
- OCI Conformance 覆盖：
  - 是否覆盖：是
  - 直接 case：`blob-post-only`、`blob-post-put`、`blob-mount`、`blob-mount-anonymous`、`blob-post-cancel`
  - 主要分组：`sha256 blobs`、`sha512 blobs`
  - 同时在 `image`、`index`、`artifact` 等分组的 push 流程里也会隐式覆盖 `blob-post-put`
  - 说明：`blob-post-only` 与 `blob-mount-anonymous` 在当前已知结果里可能走 fallback/skip，不算失败
  - 参考：`tests/oci-conformance-case-checklist.md:109`、`tests/oci-conformance-case-checklist.md:202`、`tests/oci-conformance-case-checklist.md:298`
- 调用示例：

```bash
curl -i -X POST http://<registry>/v2/<name>/blobs/uploads/

curl -i -X POST \
  "http://<registry>/v2/<name>/blobs/uploads/?mount=sha256:<digest>&from=<source-repo>"

curl -i -X POST \
  -H "Content-Type: application/octet-stream" \
  --data-binary @layer.tar \
  "http://<registry>/v2/<name>/blobs/uploads/?digest=sha256:<digest>"
```

### 2.7 Blob Upload Lifecycle API

- 完整路由：`/v2/<name>/blobs/uploads/<uuid>`
- 方法：
  - `GET`
  - `HEAD`
  - `PATCH`
  - `PUT`
  - `DELETE`
- 用途：
  - `GET` / `HEAD`：查看上传进度
  - `PATCH`：追加上传 chunk
  - `PUT`：带 `?digest=<digest>` 完成上传
  - `DELETE`：取消上传
- 代码位置：
  - 路由注册：`registry/handlers/app.go:115`
  - dispatcher：`registry/handlers/blobupload.go:21`
  - 查询状态：`registry/handlers/blobupload.go:107`
  - 追加上传：`registry/handlers/blobupload.go:132`
  - 完成上传：`registry/handlers/blobupload.go:168`
  - 取消上传：`registry/handlers/blobupload.go:269`
  - 描述定义：`registry/api/v2/descriptors.go:1211`
- OCI Conformance 覆盖：
  - 是否覆盖：是
  - `GET` / `HEAD`：通过上传状态/续传流程间接覆盖
  - `PATCH`：`blob-patch-stream`、`blob-patch-chunked`
  - `PUT`：`blob-post-put`，以及 chunked/stream 最终提交流程
  - `DELETE`：`blob-post-cancel`（当前配置下该项可 skip）
  - 主要分组：`sha256 blobs`、`sha512 blobs`、`invalid-digest-format`
  - 边界行为覆盖：`chunked out-of-order`、`chunked out-of-order and put chunk`、`bad digest chunked`、`bad digest stream`
  - 参考：`tests/oci-conformance-case-checklist.md:117`、`tests/oci-conformance-case-checklist.md:122`、`tests/oci-conformance-case-checklist.md:127`、`tests/oci-conformance-case-checklist.md:149`、`tests/oci-conformance-case-checklist.md:245`、`tests/oci-conformance-case-checklist.md:1009`
- 典型调用流程：
  1. `POST /v2/<name>/blobs/uploads/`
  2. 从响应头中拿到 `Location`
  3. 对返回的 upload URL 执行 `PATCH`
  4. 最后对该 upload URL 执行 `PUT ...?digest=sha256:...`
- 调用示例：

```bash
curl -i http://<registry>/v2/<name>/blobs/uploads/<uuid>

curl -i -X PATCH \
  -H "Content-Type: application/octet-stream" \
  --data-binary @chunk.bin \
  http://<registry>/v2/<name>/blobs/uploads/<uuid>

curl -i -X PUT \
  --data-binary @final-chunk.bin \
  "http://<registry>/v2/<name>/blobs/uploads/<uuid>?digest=sha256:<digest>"

curl -i -X DELETE http://<registry>/v2/<name>/blobs/uploads/<uuid>
```

### 2.8 Catalog API

- 完整路由：`/v2/_catalog`
- 方法：`GET`
- 用途：
  - 列出当前 registry 中已有的 repository
  - 支持分页
- 查询参数：
  - `n`：每页数量
  - `last`：分页游标
- 代码位置：
  - 路由注册：`registry/handlers/app.go:111`
  - dispatcher：`registry/handlers/catalog.go:18`
  - 处理函数：`registry/handlers/catalog.go:36`
  - 描述定义：`registry/api/v2/descriptors.go:1592`
- 仓库内测试覆盖：
  - HTTP API 级测试：`registry/handlers/api_test.go:84` `TestCatalogAPI`
  - 存储层 catalog 能力测试：`registry/storage/catalog_test.go:106`、`registry/storage/catalog_test.go:125`、`registry/storage/catalog_test.go:178`
  - 这部分主要覆盖 repository 枚举、分页参数 `n` / `last`、`Link` 响应头和非法分页参数处理
- OCI Conformance 覆盖：
  - 是否覆盖：否
  - 说明：当前 `tests/oci-conformance-case-checklist.md` 中没有 `_catalog` 对应 case
- `tests/official-registry-e2e-execution-record.md` 覆盖情况：
  - 未看到 `_catalog` 被显式执行或验证
  - 说明：当前官方社区 E2E 执行记录重点在 `push/pull`、compose 环境、OCI conformance、storage driver 和 `registry/**/_test.go`，没有单独记录 `_catalog` 验证
- 调用示例：

```bash
curl -i http://<registry>/v2/_catalog
curl -i "http://<registry>/v2/_catalog?n=100&last=<repo-name>"
```

## 3. 额外暴露的运维类接口

这些接口也是对外可访问的，但不属于 Registry V2 数据面协议本身。

| 完整路由 | 方法 | 用途 | 主要实现代码 | OCI Conformance 是否覆盖 |
| --- | --- | --- | --- | --- |
| `/` | `GET` | 进程级存活检查 | `registry/registry.go:461` | 否；未看到单独 endpoint 测试，相关服务级行为见 `registry/registry_test.go:93` |
| `/debug/health` | `GET` | 健康检查 | `health/health.go:16` | 否；仓库内由 `health/health_test.go:15`、`health/health_test.go:32`、`health/health_test.go:54`、`registry/handlers/health_test.go:17` 等覆盖 |
| `/metrics` | `GET` | Prometheus 指标 | `registry/registry.go:376` | 否；未看到 `/metrics` endpoint 直测，但有指标采集逻辑测试如 `registry/proxy/proxyblobstore_test.go:335` |

### 3.1 Alive 接口

- 完整路由：`/`
- 方法：`GET`
- 用途：
  - 进程级别存活检查
  - 只要服务起来了，这个接口通常就返回 `200`
- 代码位置：
  - 注册包装：`registry/registry.go:155`
  - 实现：`registry/registry.go:461`
- 仓库内测试覆盖：
  - 未看到单独针对 `GET /` 的专门 endpoint 测试
  - 相关服务级行为测试可参考：`registry/registry_test.go:93` `TestGracefulShutdown`
- OCI Conformance 覆盖：
  - 否
  - 说明：这不是 OCI Distribution 标准接口
- `tests/official-registry-e2e-execution-record.md` 覆盖情况：
  - 未看到对 `/` 的显式验证记录
  - 当前记录中主要使用的是 `/v2/` 和 `/debug/health` 来判断服务可用性
- 调用示例：

```bash
curl -i http://<registry>/
```

### 3.2 Health 接口

- 完整路由：`/debug/health`
- 方法：`GET`
- 用途：
  - 查看当前健康检查结果
- 代码位置：
  - 注册：`health/health.go:16`
- 仓库内测试覆盖：
  - endpoint/handler 测试：`health/health_test.go:15` `TestReturns200IfThereAreNoChecks`
  - endpoint/handler 测试：`health/health_test.go:32` `TestReturns503IfThereAreErrorChecks`
  - health wrapper 测试：`health/health_test.go:54` `TestHealthHandler`
  - checker 注册与轮询测试：`registry/handlers/health_test.go:17`、`registry/handlers/health_test.go:68`、`registry/handlers/health_test.go:133`
- OCI Conformance 覆盖：
  - 否
  - 说明：这是进程健康检查接口，不属于 OCI Distribution API
- `tests/official-registry-e2e-execution-record.md` 覆盖情况：
  - 有显式验证
  - 典型记录：`tests/official-registry-e2e-execution-record.md:40`、`tests/official-registry-e2e-execution-record.md:48`、`tests/official-registry-e2e-execution-record.md:163`、`tests/official-registry-e2e-execution-record.md:174`
  - 说明：compose E2E 复验时，明确使用 `curl http://127.0.0.1:5001/debug/health` 作为服务可用性判断的一部分
- 调用示例：

```bash
curl -i http://<registry>/debug/health
```

### 3.3 Metrics 接口

- 完整路由：`/metrics`
- 方法：`GET`
- 用途：
  - 暴露 Prometheus 指标
- 说明：
  - 仅在 debug prometheus 开启时暴露
  - 实际路径可配置，默认是 `/metrics`
- 代码位置：
  - 配置逻辑：`registry/registry.go:376`
  - 默认路径：`registry/registry.go:380`
- 仓库内测试覆盖：
  - 目前未看到针对 `/metrics` HTTP 暴露路径的专门 endpoint 测试
  - 但有指标采集逻辑相关测试，例如：`registry/proxy/proxyblobstore_test.go:335`、`registry/proxy/proxymanifeststore_test.go:294`
- OCI Conformance 覆盖：
  - 否
  - 说明：这是 Prometheus 监控接口，不属于 OCI Distribution API
- `tests/official-registry-e2e-execution-record.md` 覆盖情况：
  - 未看到 `/metrics` 被显式执行或验证
  - 说明：当前执行记录没有把 Prometheus 指标暴露接口作为官方社区 E2E 验证项
- 调用示例：

```bash
curl -i http://<registry>/metrics
```

## 4. 关于 Artifact API 的补充说明

当前这个 registry 并没有单独提供业务风格的 artifact API，例如：

- `/artifacts/list`
- `/artifacts/tag`
- `/artifacts/delete`

artifact 相关能力是通过标准 registry 路由暴露出来的：

- `/v2/<name>/manifests/<reference>`
- `/v2/<name>/referrers/<digest>`
- `/v2/<name>/tags/list`
- `/v2/<name>/blobs/<digest>`

也就是说，OCI 里的 manifest、index、artifact、subject、referrers 这些语义，都是通过标准 `/v2/...` 路径以及对应的请求体 / 响应体语义对外暴露的，而不是通过另一套单独的 artifact 管理 API。

## 5. 路由注册汇总

当前应用注册的 V2 路由名包括：

- `base`
- `manifest`
- `referrers`
- `catalog`
- `tags`
- `blob`
- `blob-upload`
- `blob-upload-chunk`

代码参考：

- `registry/handlers/app.go:106`
- `registry/api/v2/routes.go:11`

## 6. API 与覆盖关系总表

| API | 方法 | 用途 | 代码位置 | curl 示例 | OCI Conformance 是否覆盖 | 主要覆盖 case / 分组 | 仓库内测试覆盖 | `official-registry-e2e-execution-record.md` 覆盖情况 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `/v2/` | `GET` | Registry V2 基础探测 | `registry/handlers/app.go:956` | `curl -i http://<registry>/v2/` | 是 | `ping` | 有，见 `registry/registry_test.go:93` 等服务级测试 | 有，显式用 `/v2/` 判断服务可用 |
| `/v2/<name>/tags/list` | `GET` | 列 tag | `registry/handlers/tags.go:36` | `curl -i http://<registry>/v2/<name>/tags/list` | 是 | `empty -> tag list`，以及各镜像/索引/artifact 分组里的 `tag-list` | 有，见 `registry/handlers/api_test.go:458` | 无单独显式记录，主要随 conformance 覆盖 |
| `/v2/<name>/manifests/<reference>` | `GET` `HEAD` `PUT` `DELETE` | 查询/上传/删除 manifest | `registry/handlers/manifests.go:88` `registry/handlers/manifests.go:257` `registry/handlers/manifests.go:464` | `curl -i http://<registry>/v2/<name>/manifests/latest` | 是 | `manifest-by-tag`、`manifest-by-digest`、`manifest-head-by-tag`、`manifest-head-by-digest`、`manifest-delete`、`tag-delete`、`manifest-put`；分组覆盖见 `image`、`index`、`artifact`、`artifacts-with-subject`、`sha512`、`missing-manifest`、`invalid-digest-format` | 有，见 `registry/handlers/api_test.go:1691` 等 | 有，随 `push.sh`、conformance、`go test ./registry/...` 覆盖 |
| `/v2/<name>/referrers/<digest>` | `GET` | 查 subject 的 referrers | `registry/handlers/referrers.go:42` | `curl -i http://<registry>/v2/<name>/referrers/sha256:<digest>` | 是 | `empty -> referrers`、`artifacts-with-subject -> referrers`、`index-with-subject -> referrers`、`missing-subject -> referrers` | 有，见 `registry/handlers/api_test.go` 中 referrers 相关测试 | 有，主要随 conformance 覆盖，记录里也提到曾修复该路由 |
| `/v2/<name>/blobs/<digest>` | `GET` `HEAD` `DELETE` | 查/拉/删 blob | `registry/handlers/blob.go:55` `registry/handlers/blob.go:76` | `curl -i http://<registry>/v2/<name>/blobs/sha256:<digest>` | 是 | `blob-head`、`blob-get`、`blob-delete`、`range ...`；分组覆盖见 `sha256 blobs`、`sha512 blobs` 和各 image/index/artifact 分组 | 有，见 `registry/handlers/api_test.go` 中 blob 相关测试 | 有，随 `push.sh`、conformance、`go test ./registry/...` 覆盖 |
| `/v2/<name>/blobs/uploads/` | `POST` | 初始化上传 / mount | `registry/handlers/blobupload.go:67` | `curl -i -X POST http://<registry>/v2/<name>/blobs/uploads/` | 是 | `blob-post-only`、`blob-post-put`、`blob-mount`、`blob-mount-anonymous`、`blob-post-cancel` | 有，见 `registry/storage/blob_test.go:55` 与 `registry/handlers/api_test.go` 相关测试 | 有，随 `push.sh`、conformance、`go test ./registry/...` 覆盖 |
| `/v2/<name>/blobs/uploads/<uuid>` | `GET` `HEAD` `PATCH` `PUT` `DELETE` | 查询上传状态 / 续传 / 完成 / 取消 | `registry/handlers/blobupload.go:107` `registry/handlers/blobupload.go:132` `registry/handlers/blobupload.go:168` `registry/handlers/blobupload.go:269` | `curl -i http://<registry>/v2/<name>/blobs/uploads/<uuid>` | 是 | `blob-patch-stream`、`blob-patch-chunked`、`blob-post-put`、`blob-post-cancel`、`chunked out-of-order`、`bad digest chunked`、`bad digest stream` | 有，见 `registry/handlers/api_test.go:883` 等 | 有，随 `push.sh`、conformance、`go test ./registry/...` 覆盖 |
| `/v2/_catalog` | `GET` | 列 repository | `registry/handlers/catalog.go:36` | `curl -i http://<registry>/v2/_catalog` | 否 | 当前 checklist 无对应 case | 有，见 `registry/handlers/api_test.go:84` `TestCatalogAPI`，以及 `registry/storage/catalog_test.go:106` 等 | 无显式覆盖记录 |
| `/` | `GET` | 进程 alive | `registry/registry.go:461` | `curl -i http://<registry>/` | 否 | 非 OCI 标准接口 | 未看到单独 endpoint 测试；有服务级测试如 `registry/registry_test.go:93` | 无显式覆盖记录 |
| `/debug/health` | `GET` | 健康检查 | `health/health.go:16` | `curl -i http://<registry>/debug/health` | 否 | 非 OCI 标准接口 | 有，见 `health/health_test.go:15`、`health/health_test.go:32`、`health/health_test.go:54`、`registry/handlers/health_test.go:17` 等 | 有显式覆盖，compose E2E 复验时多次用它判断服务可用 |
| `/metrics` | `GET` | Prometheus 指标 | `registry/registry.go:376` | `curl -i http://<registry>/metrics` | 否 | 非 OCI 标准接口 | 未看到 `/metrics` endpoint 直测；有指标采集逻辑测试如 `registry/proxy/proxyblobstore_test.go:335` | 无显式覆盖记录 |
