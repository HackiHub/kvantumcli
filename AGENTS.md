# Repository instructions

## Branch creation and pushing

- Never configure a feature or fix branch to track `origin/main` or another
  differently named remote branch. Using `origin/main` as the starting commit
  is fine; using it as the feature branch's upstream is not.
- Create branches with tracking disabled, for example:
  `git switch --no-track -c <branch-name> origin/main`.
- After creating or switching branches, check `git branch -vv`. Before pushing,
  verify the current branch, upstream, and destination. If the upstream is wrong,
  remove it with `git branch --unset-upstream` before proceeding.
- When a push is authorized, use an explicit destination:
  `git push --set-upstream origin HEAD:refs/heads/<branch-name>`, where
  `<branch-name>` is the exact current local branch name. Its upstream must then
  be `origin/<branch-name>`.
- Never push feature work to `main`, or configure a push refspec targeting `main`.
  A direct push to `main` requires an explicit user request for that action.

## User skill maintenance

The distributable [`kvantumci` skill](skills/kvantumci/SKILL.md) is for users
running an installed CLI outside this repository. Keep it self-contained: do
not make it depend on this checkout, `bin/`, or local documentation paths.
When changing user-visible commands, flags, or output contracts, check whether
the skill and its installation instructions in `README.md` need updating.
