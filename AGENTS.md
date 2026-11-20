# tally

## 分支与合并

单人自用项目,不走 PR。除非用户明确要求新开分支,否则一切改动都直接在 `main` 上做、提交、推送。

用户要求开过功能分支时,完成后在本地直接合并到 `main`:

```bash
git checkout main && git merge <分支> && git push origin main
```

合并后删除该分支(本地 + 远端)。归档类提交(如 openspec archive)在 `main` 上做,别落在即将删除的分支上。
