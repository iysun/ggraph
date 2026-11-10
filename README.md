# ggraph

用 Go 写的交互式 git 提交图，TUI 版，类似 VS Code 的 Git Graph。

```
> *   abc1234 (HEAD -> main) Merge branch 'feature'
  |\
  | * def5678 (feature) add thing
  |/
  * 0123abc initial commit
```

## 特性

- 左侧车道图 + 提交列表，`jd/kc` 风格的光标移动（j/k、方向键、翻页）
- 分支 / 远端 / 标签着色，`HEAD ->` 高亮
- `enter` 看提交详情（`git show --stat`），详情里 `p` 切换成完整 patch
- `r` 重新加载
- `--dump` 非交互输出，能直接接管道/脚本
- 无外部依赖，只需要 PATH 里有 `git`

## 安装

```sh
go install github.com/iysun/ggraph@latest
```

或者本地构建：

```sh
git clone https://github.com/iysun/ggraph
cd ggraph
go build -o ggraph.exe .
```

## 用法

```sh
ggraph                  # 当前目录所在仓库，全部分支，最近 300 条
ggraph -n 1000          # 多看一些
ggraph --all=false      # 只看 HEAD 的历史
ggraph -C /path/to/repo
ggraph --dump -n 40     # 打印而不进入 TUI（--no-color 可关颜色）
```

按键：

| 键 | 作用 |
|---|---|
| `j` / `↓`、`k` / `↑` | 下一条 / 上一条提交 |
| `ctrl+d` / `pgdn`、`ctrl+u` / `pgup` | 翻页 |
| `g` / `G` | 第一条 / 最后一条 |
| `enter` | 提交详情（stat） |
| `p`（详情里） | 切换完整 patch |
| `r` | 重新加载 |
| `?` | 帮助 |
| `q` | 退出 |

在 Emacs 里可以直接 `C-: ggraph`（如果你配置了类似 `my/ghostel-run` 的启动器），它跑在真 TTY 里，颜色和按键都正常。

## 实现说明

- 提交数据来自 `git log --graph --date-order --decorate`，**车道图直接复用 git 自己的布局**，所以 merge/分叉的连线是准确的。
- Go 负责解析、着色、光标、详情面板和重绘（bubbletea + lipgloss）。
- 局限：没有自己实现车道布局算法；没有搜索/过滤；没有 checkout/cherry-pick 之类的写操作。

Roadmap（想做的话）：

- [ ] 自己算车道布局，去掉对 `git log --graph` 格式的依赖
- [ ] `/` 模糊搜索（提交信息 / 作者 / hash）
- [ ] 在图上直接做 checkout / cherry-pick / revert
- [ ] 图形可见的分支过滤（只看某些 ref）

## License

MIT
