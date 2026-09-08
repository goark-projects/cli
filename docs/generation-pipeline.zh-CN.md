# 分阶段代码生成

生成遵循确定顺序的阶段链：发现输入 → 完整扫描 → 校验 → 绑定模型 → 生成计划 → 渲染全部产物 → 写入。

扫描、绑定和渲染都不写文件。任一包扫描、校验、规划或渲染失败，本批次不会进入写入或清理旧文件阶段。写入阶段使用单文件原子替换；不承诺跨文件、跨 CLI 的事务回滚。Goark CLI 调用独立 ORM 工具时，两者各自完成自己的生成批次。

`internal/genpipeline` 只负责校验阶段名称及处理器、按声明顺序执行和包装阶段错误。各领域模块持有本次执行的局部状态，不使用全局可变注册表或运行时反射代理。新增阶段必须明确前置输入与输出，放在依赖阶段之后；错误必须向上传递，不能忽略后继续渲染。

## Goark CLI

`PrepareAnnotationPlans` 一次接收全部包的扫描配置，先采集所有注解节点，再统一校验，最后绑定和完成生成规划。返回的计划通过 `RenderFiles` 按扩展注册顺序生成文件。项目入口收齐所有文件后才写入。

扩展继续使用 `AnnotationDescriptor`、`AnnotationBinder`、`AnnotationGenerator`：Descriptor 定义位置、参数和重复约束；Binder 消费完整扫描后的节点构建模型；Generator 只消费模型生成源码，不再扫描文件或写入磁盘。存在依赖的扩展必须按依赖顺序注册。扩展名称不可重复（忽略大小写），也不可包含路径分隔符。

## ORM CLI

`ormgen.ScanPackages` 先加载全部目标包并校验注解，再完整采集所有 XML 资源，最后绑定实体、DAO 和 XML 映射。同一包的相同 XML 路径只读取一次，绑定阶段消费扫描快照，不再访问 XML 文件。源码语法错误必须阻止生成；首次生成时尚不存在的生成类型引用不因此被误拒绝。单包 `ScanPackage` 也走相同阶段链。CLI 随后统一规划输出路径并调用 `genoutput.Prepare` 采集原包导出类型信息，最终调用计划的 `Render`；渲染阶段不再读取原始 Go 源码。

实现职责为：`internal/sourcegen` 负责源码发现、AST、注解和 XML；`internal/genmodel` 负责共享中间模型；`internal/genrender` 负责同包源码渲染；`ormgen/genoutput` 负责 `gen` 子包输出计划；`internal/ormcli` 负责参数、批量编排与文件输出。`ormgen` 保留公开入口兼容性。

实体和 DAO 可采用任意合法包名与布局，CLI 不依赖 `entity`、`dao` 的命名。产物始终位于被扫描包的 `gen` 子目录。使用 `--config` 时，扫描参数应写入配置文件，不能再同时传入 `--dir`、`--package`、`--output`、`--build-tag` 或 `--type-handler`。

## 验证边界

阶段顺序、重复或空阶段拒绝、错误链保留、后续包失败阻止前序包渲染，以及注解冲突均有回归测试。生成验证可以通过 `--check` 和编译生成包完成，不要求连接数据库。数据库会话、事务和 Goark 集成属于独立运行时验证范围。

SQL 注解拒绝未知属性、非法 `statementType` 和同时指定 `timeout`、`timeoutDuration`。Web 参数的方括号选择器与 `param` 属性不允许指向不同参数。未初始化的 Goark 生成计划返回错误，不触发 panic。
