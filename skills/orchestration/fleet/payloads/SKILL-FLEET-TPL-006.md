After restack returned: HEAD is on the work branch <work> (git rev-parse --abbrev-ref HEAD), <branch> is untouched.
One command per call, in this order:
1. git switch <merge_branch>
2. git merge --ff-only <work>
3. git branch -d <work>
