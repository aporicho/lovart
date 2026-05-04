# 文件架构设计哲学

Status: Architecture Principle

目录树表达模块层级，`mod.rs` 表达模块接口，文件名表达模块职责，依赖方向表达架构层级。

## 核心原则

1. 文件层级即软件模块层级。代码放在哪里，就代表它属于哪个模块。
2. 文件架构即软件架构。只看目录树，就应该能理解系统由哪些模块组成、模块之间如何分层。
3. 目录名代表子系统。稳定领域模块应以目录表达，例如 `tree/`、`canvas/`、`panel/`、`control/`、`renderer/`、`context/`。
4. 目录越深，职责越具体。例如 `panel/runtime/resize.rs` 表示 GUI 的 panel 子系统里 runtime 层的 resize 能力。
5. 文件名代表单一职责。文件名必须是明确概念，例如 `storage.rs`、`mutation.rs`、`resize.rs`、`layout_flush.rs`。
6. 一个文件就是一个模块。一个文件只承载一个稳定职责，不把多个领域逻辑塞进同一个文件。
7. 禁止隐藏模块。如果一个文件内部实际包含 header、body、ports、controls 等多个模块，这些模块应该出现在目录树里。
8. 禁止伪模块文件。文件名叫 `tree.rs`，但内部包含 panel、canvas、snapshot、dirty、runtime 等多个职责，就不合格。
9. `mod.rs` 只做模块声明和 API 导出。它是模块接口清单，不是实现文件。
10. API 和实现分离。调用方依赖父级 `mod.rs` 导出的标准 API，不随意穿透内部实现文件。
11. 依赖方向必须清楚。底层通用模块不能反向依赖业务模块，例如 `tree` 不应依赖 `canvas` 或 `panel`。
12. 业务逻辑必须回到业务目录。panel resize 属于 `panel/`，canvas node runtime 属于 `canvas/`，text box runtime 属于 `control/text_box/`。
13. 拆得细，但不碎片化。以职责边界拆，不以行数机械拆。强相关逻辑可以同文件，跨职责必须拆。
14. 测试也按模块组织。测试文件应对应模块，例如 `resize_tests.rs`、`storage_tests.rs`，避免生产模块被测试淹没。
15. 迁移必须完整清理旧入口。不保留 legacy wrapper、兼容 facade、重复入口、临时胶水。
16. 架构边界要能被脚本守住。检查脚本应防止旧模块回流、依赖反转、绕过标准 API、巨型文件继续膨胀。

## `mod.rs` 规则

`mod.rs` 允许：

- `//!` 简短模块文档。
- `mod xxx;` 或 `pub(crate) mod xxx;`。
- `pub use xxx::Yyy;` 或 `pub(crate) use xxx::Yyy;`。

`mod.rs` 禁止：

- 业务实现。
- 测试。
- 常量堆积。
- helper 函数。
- `struct`、`enum`、`trait` 的主体定义。

如果需要类型，放入 `types.rs`、`model.rs` 或 `state.rs`。如果需要常量，放入 `ids.rs`、`constants.rs` 或领域命名文件。如果需要 trait，放入 `traits.rs` 或明确的 API 文件。

## 命名规则

推荐使用稳定职责名：

- `storage.rs`
- `hierarchy.rs`
- `mutation.rs`
- `dirty.rs`
- `snapshot.rs`
- `resize.rs`
- `drag.rs`
- `layout_flush.rs`
- `paint_flush.rs`
- `preedit.rs`
- `intrinsic.rs`

避免使用模糊命名：

- `utils.rs`
- `helpers.rs`
- `misc.rs`
- `common.rs`
- `legacy.rs`
- `compat.rs`
- `glue.rs`

如果确实需要共享能力，文件名必须说明共享的领域概念，而不是说明它是“工具”。

## GUI 目录示例

```text
gui/src/
  context/
    mod.rs
    core.rs
    api/
      mod.rs
      scene.rs
      query.rs
      input.rs
      rendering.rs
      canvas.rs
      panel.rs
      overlay.rs
      resources.rs
      animation.rs
      controls.rs
    flush/
      mod.rs
      layout.rs
      paint.rs
    roots.rs

  tree/
    mod.rs
    core.rs
    storage.rs
    hierarchy.rs
    mutation.rs
    dirty.rs
    layout_boundary.rs
    paint_boundary.rs
    snapshot.rs
    runtime_slots.rs

  canvas/
    mod.rs
    runtime/
      mod.rs
      layout_store.rs
      interaction.rs
      resize.rs
    node_card/
      mod.rs
      template.rs
      header.rs
      body.rs
      ports.rs
      controls.rs
    scene/
      mod.rs
      diff.rs
      model.rs

  panel/
    mod.rs
    runtime/
      mod.rs
      store.rs
      drag.rs
      resize.rs
      layout_store.rs
    retained/
      mod.rs
      frame.rs
      content.rs
    interaction/
      mod.rs
      events.rs

  control/
    mod.rs
    text_box/
      mod.rs
      runtime.rs
      store.rs
      layout.rs
      preedit.rs
      intrinsic.rs
      diagnostics.rs
    param/
      mod.rs
      mapping.rs
      layout.rs
```

示例目录不是一次性目标，而是判断后续拆分是否符合架构哲学的参考形态。

## 迁移判断标准

一次模块迁移完成时，必须同时满足：

1. 新目录树能表达新的模块边界。
2. 父级 `mod.rs` 暴露标准 API。
3. 调用方通过标准 API 访问，不穿透内部路径。
4. 旧文件里的对应职责已经消失。
5. 旧路径、旧 wrapper、旧 facade 已删除。
6. 没有为兼容旧调用保留临时胶水。
7. 相关测试移动到对应模块或测试文件。
8. 架构检查脚本能防止旧结构回流。

## Review 标准

评审 GUI 架构改动时，优先检查这些问题：

1. 目录树是否能说明这次改动属于哪个模块。
2. 文件名是否说明了单一职责。
3. `mod.rs` 是否只做声明和导出。
4. 是否存在底层模块反向依赖业务模块。
5. 是否存在大文件继续承载多个隐藏模块。
6. 是否存在 legacy、compat、glue、helper 形式的临时层。
7. 是否有调用方绕过父级 API 直接依赖内部实现。
8. 是否有旧入口没有删除。

违反这些标准时，优先修正文件架构，再讨论局部实现细节。
