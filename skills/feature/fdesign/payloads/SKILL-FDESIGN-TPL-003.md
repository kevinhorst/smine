The master is on <branch>: every derived run starts from a branch that holds it.

Per split row, in Depends-on order, each as its own prompt in its own session:
  /fdesign derived <master plan file> <slug>

Rows without a Depends-on may run side by side. Unattended, designs only — one headless designer per row, no code:
  /fleet fleet: <FleetId> target: <branch> playbook: design master: <master plan file>
Attended designers plus one implementation worker:
  /fleet fleet: <FleetId> target: <branch> playbook: feature    one session
  /join fleet: <FleetId>                                        each row's session, before its /fdesign line
  /done                                                         at the code gate: approves and delivers the plan
A row's session starts from the fleet's merge branch tip once the rows it depends on are merged there.

After the last row is implemented: run the master's Verification.
