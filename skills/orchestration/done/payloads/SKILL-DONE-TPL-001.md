After restack returned for <staging>: HEAD is on the work branch <work> (git rev-parse --abbrev-ref HEAD),
<staging> still points at its old tip. One command per call, in this order:
1. git branch <work>-orig <staging>                archive of the old tip; the name exists already: keep it, go on
2. git merge-base --is-ancestor <default> HEAD     exit 0 required, else Target unmovable
3. git branch -f <staging> HEAD
4. git switch <start>
5. $SKILL_DONE_AGENTWT_REMOVE remove --delete-branch <work>
