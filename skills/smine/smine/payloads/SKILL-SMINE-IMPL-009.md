unattended nightly change: implement exactly this change; commit it as exactly ONE commit — `git add <paths>` then `git commit -q -m "<subject>" -m "<problem> <fix>" -m "Smine-Change: <id>
Smine-Sessions: <id>, <id>"` — no /package-commit, no other commit; stage only this change's files, never sessions/ or .smine-run/
