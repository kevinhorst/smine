1. Register:  $SKILL_FLEET_HEARTBEAT_WORKER worker --branch claude/<name> --state running        prints the worker's token
2. Launch, Bash with run_in_background:
   $SKILL_FLEET_ROUTINE_WORKER worker -base <merge_branch> -name <name> -skill <skill> -args '<args>' <flags>
   Never a leading / in -skill or -args: the launcher writes the slash.
3. The launch's exit wakes the loop. Its last stdout line is the result:
   {"branch","commit","cost_usd","exit_status","name","result","session_id","status"}
   status done: the cell wrote its marker. failed: non-zero exit or an error envelope. stopped: the skill ended on one
   of its own stops; result holds its final message.
4. Delivered call, result done only:  $SKILL_FLEET_HEARTBEAT_WORKER worker --branch claude/<name> --state delivered
5. Remove call, a worker given up:    $SKILL_FLEET_HEARTBEAT_REMOVE remove --fleet <fleet> --member <letter>
6. Resume call, a stopped worker launched with -keep-stopped — same name, its session continues in its kept worktree:
   $SKILL_FLEET_ROUTINE_WORKER worker -resume <session_id> -name <name> -skill <skill> -args '<message>' <flags>
   The message is the whole prompt. Its exit and result line are those of step 3.
