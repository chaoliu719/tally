# tally

## 分支合并

单人自用项目,不走 PR。功能分支完成后在本地直接合并到 `main`:

```bash
git checkout main && git merge <分支> && git push origin main
```

合并后删除该分支(本地 + 远端)。归档类提交(如 openspec archive)先在 `main` 上做,别落在即将删除的分支上。
