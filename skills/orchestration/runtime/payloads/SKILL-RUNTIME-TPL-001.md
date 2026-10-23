| Finding kind | Default move | Move when |
| --- | --- | --- |
| check_failed | send (recovery) | always, unless the evidence shows the denied call was already corrected |
| check_failed | stop | the evidence shows the agent cannot recover (the producing call itself failed) |
| judgment | none | the evidence answers the question |
| judgment | send | the evidence contradicts the expectation the question names |
| judgment | stop | the evidence shows the run works on the wrong thing (wrong base, wrong range) |

- The eighth failure of the same check kills the run in heartbeat; the runtime agent never retries a move.
- A move allowed by heartbeat is not an obligation: skip a finding whose evidence contradicts its reason, and say why.
- A refused move (exit 1) is reported verbatim and never retried.
