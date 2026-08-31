export const meta = {
  name: 'foreign-toolchain-pretag',
  description: 'Pre-tag gate for release artifacts built by a toolchain the repo does not run natively (Inno Setup .iss, GOOS cross-builds): compile each locally — dockerized where needed — assert exit 0, and block the tag on any failure so a CI run on a pushed tag is never the first compile',
  whenToUse: 'Invoked by /package-commit (or any release-tag step) before a release tag whose build includes a foreign-toolchain artifact',
  phases: [
    { title: 'Build', detail: 'per artifact: run the local (dockerized where needed) compile and capture its exit code and log' },
    { title: 'Gate', detail: 'map every non-zero exit to a blocking error; pass only when every artifact compiled exit 0' },
  ],
}

// args contract (built by the fronting skill from the release artifacts):
// { artifacts: {toolchain, artifact, buildCmd?, dockerized?}[], tagRef?: string }
// artifacts — the foreign-toolchain artifacts to compile before the tag; one Build agent per entry.
//   toolchain   — the foreign toolchain (e.g. inno-setup, goos-cross-build).
//   artifact    — repo-relative source/output the toolchain builds.
//   buildCmd    — the local compile command (e.g. `make installer-check`, `GOOS=windows GOARCH=amd64 go build ./...`).
//   dockerized  — hint that the compile runs in a container because the toolchain is not installed natively.
// tagRef    — optional tag the gate protects; reporting only — this workflow NEVER tags.
if (!args || typeof args !== 'object' || Array.isArray(args)) {
  throw new Error('args must be an object: {artifacts, tagRef}')
}
const artifacts = Array.isArray(args.artifacts)
  ? args.artifacts.filter(a => a && typeof a === 'object' && !Array.isArray(a) && typeof a.artifact === 'string' && a.artifact)
  : []
if (artifacts.length === 0) {
  throw new Error('args.artifacts is required — the foreign-toolchain artifacts to compile before the tag')
}
const tagRef = typeof args.tagRef === 'string' ? args.tagRef : ''

const BUILD_SCHEMA = {
  type: 'object',
  required: ['toolchain', 'artifact', 'exit_code'],
  properties: {
    toolchain: { type: 'string', description: 'the foreign toolchain that built the artifact' },
    artifact: { type: 'string', description: 'the artifact compiled' },
    exit_code: { type: 'integer', description: 'the local compile exit code — 0 is a clean build' },
    log_ref: { type: 'string', description: 'repo-relative path of the captured build log, or the compiler error line' },
    notes: { type: 'string', description: 'how the compile was run (dockerized image, command), or why it could not run' },
  },
}

// ---- Stage Build: local compile per artifact (pipeline, no barrier) ----
phase('Build')
const results = await pipeline(
  artifacts,
  ({ toolchain, artifact, buildCmd, dockerized }) => agent(
    `Foreign-toolchain pre-tag gate, BUILD stage for artifact "${artifact}" built by "${toolchain || 'unknown toolchain'}". Work from the repo root. ` +
    `Compile it LOCALLY before any release tag — a CI run on a pushed tag must never be the first compile. ` +
    (buildCmd ? `Run: ${buildCmd}. ` : `Determine and run the local compile command for this toolchain. `) +
    (dockerized ? `The toolchain is not installed natively — run it dockerized (e.g. iscc via the amake/innosetup image). ` : '') +
    `Capture the exit code and write the build output to a repo-relative log under a scratch dir; return toolchain, artifact, exit_code, log_ref, and notes. ` +
    `If you cannot run the compile at all, return a non-zero exit_code with the reason in notes.`,
    { label: `build ← ${artifact}`, phase: 'Build', agentType: 'general-purpose', schema: BUILD_SCHEMA },
  ),
)

// ---- Stage Gate: pure-JS pass/block on the captured exit codes ----
phase('Gate')
const builds = results.map((r, i) => r || {
  toolchain: artifacts[i].toolchain || '',
  artifact: artifacts[i].artifact,
  exit_code: 1,
  log_ref: '',
  notes: `build agent returned nothing for ${artifacts[i].artifact}`,
})
const blocking_errors = builds
  .filter(b => b.exit_code !== 0)
  .map(b => `${b.artifact} (${b.toolchain || 'unknown toolchain'}): exit ${b.exit_code}${b.notes ? ` — ${b.notes}` : ''}`)

return {
  tagRef,
  gate: { pass: blocking_errors.length === 0, blocking_errors },
  builds,
}
