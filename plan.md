# DSMF / DPF / DSF Integration Plan

## Summary

- 目标是在现有 `free5gc` 工程中落一个可运行的最小数据服务闭环：`测试客户端/NEF -> DSMF -> DPF -> DSF -> DSMF -> 请求方`
- 采用“重建复用”方式处理现有 `NFs/dsmf/`：该目录原本是 `smf` 的拷贝壳子，改造成真正的 `DSMF`，并新增 `NFs/dpf/`、`NFs/dsf/`
- 与现有项目的集成范围锁定为：根级构建、启动脚本、统一配置目录、测试框架、可选 `NEF` 薄转发
- 本轮不做 `SOF`、`eNRF` 能力注册、动态发现，以及论文里其余扩展网元

## Key Changes

### 1. 协议与接口基线

- 新增共享协议模块 `NFs/dataapi`
- 在该模块下维护第二个 PDF 对应的 3 组协议定义：
  - `dsmf <-> dpf`
  - `dsmf <-> dsf`
  - `dpf <-> dsf`
- 保留论文和 PDF 中涉及的协议枚举、状态枚举和核心消息结构
- 当前工程实现采用手写 gRPC service descriptor + JSON codec 的方式落地最小可运行面，同时保留 `.proto` 文件作为接口基线
- DSMF 对外暴露最小 HTTP/JSON 入口：
  - `POST /ndsmf-data-service/v1/tasks`
  - `GET /ndsmf-data-service/v1/tasks/{taskId}`
- 在现有 `NEF` 中增加薄转发入口：
  - `POST /nnef-data-service/v1/tasks`
- v1 实现的协议子集：
  - 控制面：gRPC over HTTP/2
  - 结果载荷编码：`PROTOBUF`、`JSON`
  - `RDMA`、`QUIC`、`HTTP3` 只保留枚举，占位返回 `unsupported`

### 2. DSMF 重建

- 将 `NFs/dsmf/` 从原有 `SMF` 语义重建为 `DSMF`
- 移除 PFCP、PDU Session、UPF、SM context 等与本项目目标无关的遗留逻辑
- DSMF 的职责收敛为：
  - 请求接入
  - 请求解析与任务生成
  - DPF/DSF 静态选择
  - 任务下发、状态聚合、结果返回
- 内部维护内存态任务表，核心字段包括：
  - `request_id`
  - `orchestration_id`
  - `processing_task_id`
  - `storage_task_id`
  - `state`
  - `sync_or_async`
  - `result_uri`
  - `error`
- 编排顺序固定为：
  1. 接收请求并标准化
  2. 根据静态配置选择 1 个 DPF 和 1 个 DSF
  3. 先向 DSF 下发 `SubmitStorageTask`
  4. 再向 DPF 下发 `SubmitProcessingTask`
  5. 接收 DPF 状态回调与 DSF 存储回调
  6. 同步模式等待聚合完成后返回结果，异步模式返回查询接口
- 新增 DSMF 静态配置：
  - `dpfEndpoints`
  - `dsfEndpoints`
  - 默认协议组合
  - 超时
  - 存储策略默认项

### 3. 新增 DPF

- 新建独立 Go module：`NFs/dpf/`
- 目录结构对齐现有 NF 风格：
  - `cmd`
  - `pkg/{app,factory,service}`
  - `internal/{context,logger,rpc,processor}`
- DPF 提供：
  - 任务接收
  - 数据接入
  - 数据预处理
  - 数据处理
  - 向 DSF 结果投递
  - 向 DSMF 状态回调
- 数据源接入 v1 仅支持：
  - `file`
  - `http` / `https`
- 处理逻辑 v1 提供两档：
  - `BREATHING_CSI` 示例处理链路
  - 通用 `passthrough`/字段提取链路
- DPF 完成处理后，通过 `ResultTransferService` 向 DSF 推送结果，并回调 DSMF 处理状态

### 4. 新增 DSF

- 新建独立 Go module：`NFs/dsf/`
- 结构与 DPF 对齐
- DSF 暴露：
  - `SubmitStorageTask`
  - `ResultTransferService`
  - `ReportStorageStatus` 回调客户端
- 存储后端 v1 仅实现本地文件系统
- 支持配置：
  - 根目录
  - 对象前缀
  - 覆盖策略
  - 保留天数元数据
- DSF 在 `OpenTransfer` 时建立接收上下文
- 在 `PushResult` 阶段按流累计写入
- 在 `CloseTransfer` 后生成：
  - `result_uri`
  - `result_id`
  - `checksum`
  - `stored_bytes`
- DSF 最终通过 `ReportStorageStatus` 异步回传 DSMF

### 5. 仓库级集成

- 根目录新增配置文件：
  - `config/dsmfcfg.yaml`
  - `config/dpfcfg.yaml`
  - `config/dsfcfg.yaml`
- 更新根级构建和运行入口：
  - `Makefile` 将 `dsmf`、`dpf`、`dsf` 加入 `GO_NF`
  - `run.sh` 将 3 个新 NF 纳入启动列表
- 测试工程同步接入：
  - `test/go.mod` 增加新 module 依赖与本地 `replace`
  - `test/nfs_setup.go` 增加 `Dsmf`、`Dpf`、`Dsf` 开关和初始化函数
- `NFs/nef` 增加最小 northbound 数据服务转发接口，不改现有 PFD/TI 主逻辑

## Public APIs / Types

### DSMF 对外 HTTP API

- `POST /ndsmf-data-service/v1/tasks`
- `GET /ndsmf-data-service/v1/tasks/{taskId}`

### 共享 gRPC API

- `DpfControlService.SubmitProcessingTask`
- `DsmfProcessingCallbackService.ReportProcessingStatus`
- `DsfControlService.SubmitStorageTask`
- `DsmfStorageCallbackService.ReportStorageStatus`
- `ResultTransferService.OpenTransfer`
- `ResultTransferService.PushResult`
- `ResultTransferService.CloseTransfer`

### 关键配置类型

- `DsmfConfig`
- `DpfConfig`
- `DsfConfig`

### 共享协议类型

- `ProtocolType`
- `ProcessingState`
- `StorageState`
- `TransferState`
- `DataSourceSpec`
- `DeliveryBinding`
- `ReceiveContract`

## Test Plan

- 单元测试：
  - DSMF 请求解析、静态选择、任务状态迁移、同步/异步返回
  - DPF 处理链路、状态回调、协议参数校验
  - DSF 传输会话、分块接收、落盘、结果地址生成
- 协议测试：
  - 3 组 gRPC service 的 request/response 与错误路径
  - `PROTOBUF` 与 `JSON` 两种 payload 编码正确性
  - 不支持协议返回 `unsupported`
- 延迟分布测试：
  - 当前正式测试目标切换为 `UE -> AMF -> DSMF -> DPF -> DSF -> DSMF` 的整链路延迟，而不是此前的假 JSON 文件直连 DSMF
  - 测试输入统一来自远端 `/data/chensb-data/data-framework/free5gc/prepared_csi_streams/`
  - 当前按 3 类顶层场景执行：
    - `gesture` -> `sourceScenario=GESTURE_RECOGNITION_CSI`
    - `localization` -> `sourceScenario=POSITIONING_CSI`
    - `vehicle` -> `sourceScenario=VEHICLE_CSI`
  - 每个场景每次只发送 1 个时刻的 CSI 包：
    - 输入文件格式为 prepared 生成的逐行 TSV
    - 每轮测试把 1 行 CSI 数据连同表头写成临时单包文件，再由 AMF 的 DSMF trigger 作为 `file` 数据源下发
  - 每个场景固定执行 `30` 次，得到端到端 latency 原始样本和分布统计
  - 本轮分布统计的主指标为：
    - `ue_trigger_total_from_registration`
    - `amf_to_dsmf_sync_total`
    - `dsmf_sync_task_total`
    - `dpf_delivery_total`
  - 每轮测试至少记录：
    - 场景名
    - 输入 prepared 文件
    - 输入行号
    - UE 触发序号
    - DSMF `task_id`
    - `orchestration_id`
    - 各阶段耗时
    - DSMF 最终状态
  - 延迟统计结果至少输出：
    - 样本数 `N`
    - `min`
    - `max`
    - `avg`
    - `p50`
    - `p90`
    - `p95`
    - `p99`
    - `stddev`
  - 结果按场景分别保存为 CSV，便于后续画直方图、CDF 或箱线图
  - 远端执行完成后，结果必须同步回本地 `remote-test-results/`
  - 若中途出现失败请求，需要单独记录失败次数和失败原因，不能混入成功样本的延迟统计
- 集成与回归测试：
  - `NEF/客户端 -> DSMF -> DPF -> DSF -> DSMF` 同步成功链路
  - 异步查询链路
  - DPF 处理中失败
  - DSF 落盘失败
  - 静态配置错误时 DSMF 拒绝任务
  - 结果文件 URI、checksum、bytes 生成正确
- 启动验证：
  - `make` 能构建 3 个新 binary
  - `run.sh` 能拉起 3 个新 NF 且不影响现有 NF 启动

## Remote Test Environment

- 远端测试机：
  - IP: `172.27.33.78`
  - User: `chensb`
  - Remote path: `/data/chensb-data/data-framework/free5gc`
- 远端主机名在登录后显示为：`chensb@a100-1-caizhp`
- 后续需要真实运行验证时，优先在该远端目录执行，不在本地直接跑集成测试
- 远端测试时保持长连接，修改后需要同步本地与远端代码，避免两边代码漂移
- `prepared_csi_streams/` 存放用于协议、编码、分包和流式回放实验的 prepared 测试数据
- `prepared_csi_streams/` 不通过 Git 管理；该目录内容有更新时，需要单独同步到远端 `/data/chensb-data/data-framework/free5gc/prepared_csi_streams`
- 远端已有环境相关改动需要保留，至少包括：
  - `cert/nrf.pem`
  - `config/amfcfg.yaml`
  - `config/smfcfg.yaml`
  - `config/upfcfg.yaml`

## srsRAN Integration Plan

- 后续 RAN 方向选择 `srsRAN Project`，先支持无 USRP 的 ZMQ/software RF 调试，之后再切到 USRP
- srsRAN 源码放在当前仓库的 `external/srsRAN/`
- `external/srsRAN/` 采用 vendored source 方式管理：
  - 不作为 Git submodule
  - 不保留 srsRAN 自己的嵌套 `.git`
  - srsRAN 源码修改直接由当前 free5GC 仓库记录
- 集成配置和脚本放在：
  - `ran/srsran/`
  - `scripts/bootstrap_srsran_local.sh`
  - `scripts/sync_srsran_to_remote.sh`
  - `scripts/sync_srsran_from_remote.sh`
  - `scripts/build_srsran_remote.sh`
  - `scripts/run_srsran_gnb_remote.sh`
- 本地/远端同步约定：
  - 本地是主要编辑位置
  - 远端是主要编译和运行位置
  - 本地改 srsRAN 后执行 `scripts/sync_srsran_to_remote.sh`
  - 如果临时在远端改了 srsRAN，继续本地开发前必须执行 `scripts/sync_srsran_from_remote.sh`
  - 同一次编辑会话只选择一个同步方向，避免覆盖对方修改
- free5GC 对接 srsRAN 时的关键配置：
  - AMF N2 地址继续使用 `10.0.0.197`
  - UPF N3 地址不能保持 `127.0.0.8`，需要改成 gNB 可达地址，例如 `10.0.0.197`
  - SMF `userplaneInformation.upNodes.UPF.interfaces[N3].endpoints` 要与 UPF N3 地址一致
- 当前已加入的配置模板：
  - `ran/srsran/gnb_zmq.yaml`
  - `ran/srsran/gnb_usrp.yaml`
  - `ran/srsran/free5gc-upfcfg.srsran.patch`
  - `ran/srsran/free5gc-smfcfg.srsran.patch`
- 当前状态：
  - 已由用户将 srsRAN Project 源码放入 `external/srsRAN/`
  - 已同步到远端 `/data/chensb-data/data-framework/free5gc/external/srsRAN`
  - 已在远端完成 ZMQ/no-USRP 构建，`gnb` 版本为 `srsRAN 5G gNB version 25.10.0 (fa995e6)`
  - 已验证 `gnb_zmq.yaml` 可以连接 free5GC AMF，并在 AMF 日志中看到：
    - `SCTP Accept from: 10.0.0.197:50159`
    - `Handle NGSetupRequest`
    - `Send NG-Setup response`
  - 当前远端暂不安装 UHD/USRP 依赖，避免拉取 GNURadio/UHD 大依赖导致根分区空间不足
  - 远端根分区空间很紧，USRP 构建前需要先清理或扩容 `/`

## Remote Test Record

### 2026-04-23

- 已完成本地到远端 `/data/chensb-data/data-framework/free5gc` 的代码同步
- 已把远端生成的依赖校验文件同步回本地，当前两边以下文件保持同步：

### 2026-04-24

- 已补充 `prepared_csi_streams/` 的远端同步约定
- 已为 `chensb@172.27.33.78` 配置本机 SSH 公钥登录，当前可免密连接
- 已将本地 `prepared_csi_streams/` 同步到远端 `/data/chensb-data/data-framework/free5gc/prepared_csi_streams`
- 后续只要重新 prepare 数据或更新该目录内容，都需要再次执行远端同步，避免本地实验数据与远端测试数据不一致
- 已将 `external/srsRAN/` 源码、`ran/srsran/` 集成配置和 srsRAN 相关脚本同步到远端
- 已在远端安装 ZMQ 构建最小依赖，不安装 UHD/USRP 依赖
- 已远端构建通过 `external/srsRAN/build/apps/gnb/gnb`
- 已完成 srsRAN gNB 到 free5GC AMF 的 N2 smoke test，AMF 收到 `NGSetupRequest` 并返回 `NG-Setup response`
  - `NFs/dataapi/go.sum`
  - `NFs/dsmf/go.sum`
  - `NFs/dpf/go.sum`
  - `NFs/dsf/go.sum`
- 远端构建结果：
  - `make dsmf dpf dsf` 通过
  - `make nef` 通过
- 远端运行方式：
  - 单独启动 `DSF`、`DPF`、`DSMF`
  - 使用 `POST http://127.0.0.31:8010/ndsmf-data-service/v1/tasks` 做闭环烟测
- 远端真实修复记录：
  - 修复 `NFs/dsmf/internal/processor/processor.go` 中未使用变量 `dpfEndpoint`
  - 修复 `NFs/dsf/internal/processor/processor.go` 中 DSF 仅按存储任务 ID 建索引，导致 `OpenTransfer` 找不到存储任务的问题
  - 修复 `NFs/dpf/internal/processor/processor.go` 中 `PROTOBUF` 路径对自定义字符串枚举的编码问题，避免 `structpb.Struct` 报 `proto: invalid type`

### 2026-04-23 Protocol Smoke Test

- 测试输入：
  - `transportProtocol=HTTP2`
  - `payloadProtocol=JSON`
  - `payloadProtocol=PROTOBUF`
  - 数据源为远端本地文件 `file:///tmp/data-framework-test/source.json`
  - 场景为 `BREATHING_CSI`
- 测试结果：
  - `JSON`: `HTTP 200`, `state=COMPLETED`, `processingState=COMPLETED`, `storageState=STORED`
  - `PROTOBUF`: `HTTP 200`, `state=COMPLETED`, `processingState=COMPLETED`, `storageState=STORED`
- 结果文件已在远端 DSF 落盘，并已拷回本地
- 本地回传目录：
  - `remote-test-results/20260423-protocol/`
- 本地结果文件：
  - `remote-test-results/20260423-protocol/protocol_test_results.csv`
  - `remote-test-results/20260423-protocol/results/json-proc-9ecf1818-f98b-4751-a024-5a0f9413c33a-3939ffcb-b1f1-49ac-afd9-5f4853869aa4.json`
  - `remote-test-results/20260423-protocol/results/protobuf-proc-d58b7dc7-eb01-4bd7-ab38-b3fe66a61554-cbf43688-6232-4d43-8f83-50d1d48ab7aa.pb`
- CSV 记录字段包括：
  - `payload_protocol`
  - `http_status`
  - `task_id`
  - `orchestration_id`
  - `state`
  - `processing_state`
  - `storage_state`
  - `result_uri`
  - `result_file_exists`
  - `result_file_size`
  - `result_file_sha256`
  - `error`
- 当前记录只说明两种协议链路均已跑通，不能替代后续“多次调用下的延迟分布测试”

### Next Benchmark Target

- 下一阶段正式测试目标：
  - 在远端 `172.27.33.78` 上使用 `prepared_csi_streams/` 进行 UE 触发整链路延迟测试
  - 对 `gesture`、`localization`、`vehicle` 3 个场景分别执行 `30` 次
  - 每次测试只发送 1 个时刻的 CSI 包
  - 生成按请求粒度展开的原始延迟 CSV
  - 输出每个场景的延迟分布统计摘要
  - 将统计结果、原始 CSV 和关键日志回传本地，但保存在 `remote-test-results/` 下并保持不提交到 GitHub
  - 测试结果数据文件继续按日期或场景落在 `remote-test-results/<case>/...`
  - MATLAB 绘图脚本 `.m` 统一放在 `remote-test-results/` 根目录，不放在其子目录中

### 2026-04-23 Latency Distribution Benchmark

- 使用脚本：
  - `scripts/protocol_latency_benchmark.py`
- 远端执行参数：
  - `--iterations 30`
  - `--warmup 3`
  - `--task-timeout-seconds 60`
- 远端输出目录：
  - `/tmp/data-framework-benchmark/20260423-latency-timeout60/results/`
- 本地回传目录：
  - `remote-test-results/20260423-latency-timeout60/`
- 结果摘要：
  - `JSON`: `30/30` 成功，`avg=10.361 ms`，`p50=10.184 ms`，`p90=12.683 ms`，`p95=12.866 ms`，`p99=13.746 ms`
  - `PROTOBUF`: `30/30` 成功，`avg=9.090 ms`，`p50=9.204 ms`，`p90=10.025 ms`，`p95=10.322 ms`，`p99=10.954 ms`
- 结论：
  - 在当前远端环境和当前实现下，`PROTOBUF` 的平均延迟和尾延迟都优于 `JSON`
  - 当 `taskTimeoutSeconds=60` 时，本轮基准未观察到失败样本
- 本地结果文件：
  - `remote-test-results/20260423-latency-timeout60/results/protocol_latency_raw.csv`
  - `remote-test-results/20260423-latency-timeout60/results/protocol_latency_summary.csv`
  - `remote-test-results/20260423-latency-timeout60/results/protocol_latency_summary.json`

### 2026-04-23 AMF Triggered DSMF Validation

- 目标：
  - 验证调用路径由 `模拟 UE -> AMF -> DSMF` 触发，而不是直接向 DSMF 发 HTTP 请求
- 本轮新增与调整：
  - `NFs/amf/internal/gmm/dsmf_trigger.go`
    - 增加 `dsmfTrigger` 配置驱动的 DSMF HTTP 触发逻辑
    - 增加 `Dispatch DSMF trigger` 和 `Triggered DSMF data transfer` 显式日志
  - `NFs/amf/internal/gmm/handler.go`
    - 在 `RegistrationType: Initial Registration` 分支触发 DSMF
    - 保留 `CreatePDUSession` 触发入口，后续可切回 `pdu_session_establishment`
  - `NFs/amf/pkg/factory/config.go`
    - 增加 `dsmfTrigger` 配置结构与 `triggerEvent` 校验
  - `config/amfcfg.dsmf-trigger.yaml`
    - 当前远端验证使用 `triggerEvent: registration_request`
  - `test/ueRanEmulator/config/ueranem.amf-dsmf.yaml`
    - 改为使用远端可达的 `10.0.0.197` 作为 `n2Amf.addr` 与 `n2Ran.addr`
  - `test/cmd/seed_ue_subscriber/main.go`
    - 增加按 UE 配置自动灌入测试订阅数据的工具
- 远端执行方式：
  - 远端目录：`/data/chensb-data/data-framework/free5gc`
  - 执行 `make -B amf`
  - 用 `AMF_CONFIG_PATH=./config/amfcfg.dsmf-trigger.yaml ./run.sh -p /tmp/amf-dsmf-ue-trigger` 启动 core
  - 用 `go run ./cmd/seed_ue_subscriber -c ./ueRanEmulator/config/ueranem.amf-dsmf.yaml` 灌入测试 UE
  - 用 `go run ./ueRanEmulator -c ./ueRanEmulator/config/ueranem.amf-dsmf.yaml` 发送注册消息
- 远端实测结论：
  - `UE -> AMF` 已成功打通，UE 日志确认：
    - `Connect to AMF successfully`
    - `Connect to UPF successfully`
    - `NGSetup successfully`
  - `AMF -> DSMF` 已成功打通，AMF 日志确认：
    - `Handle Registration Request`
    - `RegistrationType: Initial Registration`
    - `Dispatch DSMF trigger`
    - `Triggered DSMF data transfer: taskId=c3784e18-d573-4822-9804-a0b60dac766b state=COMPLETED`
  - 本次验证点是 `registration_request`，DSMF 同步任务成功完成并返回 `resultUri`
- 当前残留问题：
  - UE 后续流程在 AMF 侧停在 `ContextSetup Fail`
  - 该问题不影响本次目标“由模拟 UE 经 AMF 成功触发 DSMF”，但会阻塞后续把触发点切回 `pdu_session_establishment`
  - 后续若要验证“UE 建立 PDU Session 后再触发 DSMF”，需要继续排查 `InitialContextSetupResponse` 之后的上下文建立问题

### Result File Convention

- `remote-test-results/` 仅作为本地回传结果根目录，不提交到 GitHub
- 原始结果数据按测试批次保存在对应子目录，例如：
  - `remote-test-results/20260423-ue-trigger-latency/results/*.csv`
  - `remote-test-results/20260423-latency-timeout60/results/*.json`
- 所有 MATLAB 绘图脚本 `.m` 统一放在 `remote-test-results/` 根目录
- `.m` 脚本内部通过相对路径读取对应测试子目录里的 CSV/JSON，不在测试子目录中重复放置脚本

## Assumptions

- `plan.md` 放在仓库根目录
- 现有 `NFs/dsmf/` 被视为可重构资产，不保留原有 SMF 兼容逻辑
- 本轮不实现 `SOF`、`eNRF` 能力注册、动态 DPF/DSF 发现，也不修改上游 `openapi` 依赖
- 与现有 NF 的集成定义为：接入根级构建、启动、测试框架，并提供 `NEF` 薄转发，不做全量 5G SBA 能力注册
- v1 的目标是落一个论文思路与第二个 PDF 接口草案对应的最小工程闭环，不追求真实 CSI 算法或 RDMA/QUIC 全实现
