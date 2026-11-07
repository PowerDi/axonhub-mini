# beta12 同步复核修复报告（2026-11-07）

## 结论与范围

- 在本地 `sync/beta12` 上，从 `7c82daab` 开始追加提交；完成必做 **1–11** 和可选 **12**。可选 **13** 留待后续。
- 保留 mini 多端点、模型 Allow/Exclude（Allow 非空优先、非优先级列表）、过滤后严格跳过渠道的设计；未引入 `ModelProtocols`。
- 未改 GraphQL/Ent schema，因此未运行 `make generate`；未安装依赖、改依赖版本、运行 lint/build、启停长驻服务、改 mini 版本号、推送或发版。

## 修复与回归对应

以下前端通用回归文件简称 **FE 回归**：`frontend/src/features/system/data/beta12-review-regressions.test.mjs`；浏览器文件简称 **UI 回归**：`frontend/tests/ui-regression/beta12.spec.ts`。

| 项 | 修复要点 | 回归测试 / 验证 |
|---|---|---|
| 1 | 挤模式预算独立于全局重试开关；流式预提交探测同样识别该预算。普通渠道关闭全局重试后仍不重试、不切渠道，普通渠道级次数覆盖也不重新启用重试。 | `llm/pipeline/hammer_disabled_retry_test.go`：非流式/流式持续 429，各对比挤模式 4 次、普通渠道 1 次，检查间隔和不切渠道；另测多条元数据后的流错误重试。原 `same_channel_retry_limit_test.go` 继续通过。 |
| 2 | 按模型比对存量和提交策略；原样回传（包括列表重排、nil/空列表等价）的策略按旧身份解释，真正新增/修改的策略才按新身份解释。必要时保存规范 WS 端点，避免保留下来的 HTTP 策略在下次读取时被重新解释；仅 settings 编辑不会额外展开端点能力。 | `internal/server/biz/channel_endpoint_responses_ent_test.go`：A（停用 openai 拆 HTTP/WS、Exclude 仍禁 WS）、B（WS→HTTP）、C（HTTP→WS）原样携带 settings 的回归，B/C 拒绝且存储不变；另测停用渠道编辑后重读、真正改动/新增策略和 settings-only 能力不变。`channel_endpoint_responses_update_test.go` 验证真正显式改动；原 `TestResponsesPolicyRoutabilityAcrossEndpointEdits`、`TestResponsesEndpointLegacyNormalizationEnt` 通过。 |
| 3 | 增加 `xai_subscription` 配额解析分支，保留套餐、周/月账单数据及类型判别。 | FE 回归执行真实解析器和电量百分比函数（35%/65% 得到 65%）；遍历类型联合及 quota-badges 使用的判别类型，确认支持的提供商不落入 unknown。 |
| 4 | 删除提交时针对派生类型的第二次运行时 parse，恢复类型断言；显式保留凭据数组默认值，未放松类型检查。 | FE 回归守护创建/复制共用提交路径不再二次 parse；UI 回归实际复制 `anthropic_gcp` 渠道，确认发出 `DuplicateChannel` 并关闭弹窗，而非静默停留。 |
| 5 | 改为有界固定列宽 + `table-fixed`，恢复 `min-h-48`；默认列合计 1104px，表格最小宽度 1120px；保留窄屏横向滚动，不恢复响应式隐藏列。 | `frontend/src/features/channels/data/channels-columns-quota.test.mjs`：等价列宽守护，禁止混用百分比，默认宽度不超过 1120px。UI 回归用真实 ChannelsTable：1440px、232px 默认侧栏和页面边距下无横向溢出、可点行菜单；390px 下可以横向滚动到行操作。 |
| 6 | 旧策略翻译推广至所有渠道类型，重命名为 `translateLegacyResponsesModelPolicies`；普通端点弹窗也提交翻译后的策略，保留非 Codex 的原 Base URL。 | `frontend/src/features/channels/data/protocol-options.test.mjs` 及两份 Codex 弹窗守护测试；UI 回归：`openai_responses`、`wss://api.openai.com/v1#`、Allow Responses，新增聊天端点后正常保存，提交 Allow Responses WS。 |
| 7 | 在 Requests 后、Cost 前补回 tok/s、TTFT 表头，使用已有翻译 key。 | FE 回归核对 11 个表头、11 个 col、11 个数据单元及顺序；UI 回归检查实际表头和数据列。 |
| 8 | 模型策略 settings PATCH 成功后显示 `channels.messages.updateSuccess` toast，再关闭弹窗。 | FE 回归检查调用顺序；UI 回归检查真实 PATCH 和成功 toast。 |
| 9 | 默认 Playwright 排除 ui-regression；临时 Vite 使用 `vite.ui.config.ts` 和独立 `.vite-ui-regression` 缓存；两个交互登录 suite 使用空 storageState。 | FE 回归守护配置；`playwright test --list` 列出 117 个测试、23 个文件，无 ui-regression。Codex 的 `test.use` 放在调用 helper 的 `system-codex-compatibility.spec.ts`，而不是工具模块中。 |
| 10 | 错误横幅恢复 mini warning 色变量并保留移动端换行；恢复全局 `components/quota-badges.tsx` 电量图标 `shrink-0`。 | FE 回归检查 warning 变量、无硬编码 orange、移动端布局和图标不收缩。 |
| 11 | 原样恢复与 ModelProtocols 无关的 4 个上游测试及其必要 import。 | `internal/server/orchestrator/outbound_test.go`：`TestFinalizeTransportRequestUsesSwitchedCandidate`；`select_endpoints_test.go`：`TestSelectAPIFormat_DecisionsRequiresExplicitEndpoint`、`TestSelectAPIFormat_SystemOneRequiresExplicitEndpoint`、`TestSpecifiedChannelSelector_DecisionsWithoutEndpointReturnsNoCandidate`。 |
| 12（可选） | 模型列表单独更新不再加载并写回 settings 快照。这里没有端点身份变更，不需要归一化；不产生 settings 写入，也就无需为这种不存在的写回新增版本检查。身份编辑仍使用已有版本保护。 | `internal/server/biz/channel_settings_test.go`：`TestModelOnlyUpdatePreservesInterleavedSettingsPatch` 在模型更新落库前插入真实 settings PATCH，检查两个新设置和旧无关字段保留。修复前该测试确实失败（后写 PATCH 被覆盖），修复后通过。 |

### 明确保留的行为

- 未修改 `internal/server/orchestrator/hammer_retry.go` 的 `OverridesSameChannelLimit()` 或错误分类。
- 新增 `TestHammerKeepsRetrying5xxAndEmptyResponses`（`internal/server/orchestrator/hammer_retry_test.go`）：在挤模式预算仅 2 次的配置下，对普通 500、空响应分别连续检查 20 次仍允许同渠道重试，且不消耗 429 预算。结合 pipeline 回归和全量测试，确认没有为这些错误加次数上限或强制切渠道。
- 保持 beta12 的 `retryableStatusCodes:[429]` 显式开关；未处理 ReasoningScrubMode 接入、深档 conversation/item_reference 清理、同渠道次数为 0 时清洗不触发等已排除事项。

## 测试结果

| 命令 | 结果 |
|---|---|
| `/usr/local/go/bin/go test ./...`（根模块） | 通过 |
| `cd llm && /usr/local/go/bin/go test ./...` | 通过 |
| `cd frontend && pnpm run typecheck && pnpm run typecheck:ui && pnpm run test:unit` | 类型检查全部通过；单测 413 通过、0 失败、1 跳过（既有 DST 测试） |
| `cd frontend && pnpm run test:ui:regression` | 15/15 通过；只使用 9530 临时服务、模拟数据和独立缓存 |
| `cd frontend && pnpm exec playwright test --list` | 117 个测试 / 23 个文件；默认 suite 不包含 UI 回归 |
| `git diff --check` | 通过 |

验证期间修正了测试夹具问题：补齐 GeneralSettings/ChannelNames mock 和表格所需的内存 Router；动画弹窗查询限定可访问弹窗，确认弹窗覆盖时显式允许隐藏父弹窗；模板选择使用当前焦点按 Enter，避免 placeholder 消失后的定位超时。这些是测试稳定性修正，不绕过业务断言。

## 本地提交

- `d8b7f3d6` — `fix(retry): preserve hammer budgets when global retries are disabled`
- `9c754c8e` — `fix(channels): preserve legacy model policy identity across endpoint edits`
- `8bcb17a1` — `test(orchestrator): restore beta12 transport and endpoint regressions`
- `bb0a8dd2` — `fix(channels): avoid expanding endpoints during settings-only edits`
- `8f4afaa1` — `fix(frontend): restore beta12 channel interactions and isolate UI tests`
- `460aad66` — `fix(channels): preserve settings patches during model-only updates`
- 本报告另作 `docs: record beta12 review fixes and validation` 文档提交。

## 未做事项与说明

- **可选 13 未做**：删除模型后的存量策略目前仍保留，也未新增“仅校验实际提供模型”的过滤。此项需要覆盖模型别名/前缀、创建、自动同步及前端端点校验等路径；本轮不把必做修复扩大成跨路径策略清理，避免误删仍通过映射提供的模型策略。
- 列宽采用任务允许的“等价固定宽度守护”，不是恢复百分比之和为 100 的断言；真实浏览器同时验证桌面和窄屏。
- 原显式策略更新测试的初始数据由 Allow 改为 Exclude，使提交的 Allow 确实属于“改动过的策略”；原样回传的行为由新增 A/B/C 测试单独守护，未保留旧测试中“原样回传即重新解释”的错误假设。
- 无其他设计边界偏离。所有提交仅存在于本地 `sync/beta12`；无 push、amend、rebase 或历史改写，最终工作区干净。
