export const meta = {
  name: 'config-ui-fidelity-gate',
  description: 'Pre-first-commit gate for config-server UI changes: render each touched template via a throwaway harness, screenshot it, diff new inputs against the existing style.css design system, assert a single Save button, and gate the commit on the result',
  whenToUse: 'Invoked by /fimplement (or /fdesign change) before the first commit of any change that touches config-server templates or internal/server/assets/style.css',
  phases: [
    { title: 'Render', detail: 'per template: stand up a throwaway render harness (never the live server) and screenshot the rendered template' },
    { title: 'Gate', detail: 'per template: diff new selectors vs style.css, count Save buttons and unstyled inputs, decide pass/violations' },
  ],
}

// args contract (built by the fronting skill from the touched files):
// { templates: string[], cssPath?: string, renderHint?: string }
// templates — repo-relative config-server template paths the diff touches.
// cssPath   — the design-system stylesheet to diff against (default internal/server/assets/style.css).
// renderHint — optional free-text hint on how the throwaway render harness is stood up.
if (!args || typeof args !== 'object' || Array.isArray(args)) {
  throw new Error('args must be an object: {templates, cssPath, renderHint}')
}
const templates = Array.isArray(args.templates) ? args.templates.filter(t => typeof t === 'string' && t) : []
if (templates.length === 0) {
  throw new Error('args.templates is required — the config-server template paths touched by the diff')
}
const cssPath = typeof args.cssPath === 'string' && args.cssPath ? args.cssPath : 'internal/server/assets/style.css'
const renderHint = typeof args.renderHint === 'string' ? args.renderHint : ''

const RENDER_SCHEMA = {
  type: 'object',
  required: ['template', 'harness_ok', 'image_path'],
  properties: {
    template: { type: 'string', description: 'the template path rendered' },
    harness_ok: { type: 'boolean', description: 'true if the throwaway render harness produced the page' },
    image_path: { type: 'string', description: 'repo-relative path of the screenshot written, empty if harness_ok is false' },
    notes: { type: 'string', description: 'how the harness was stood up, or why it failed' },
  },
}

const GATE_SCHEMA = {
  type: 'object',
  required: ['template', 'save_button_count', 'pass', 'violations'],
  properties: {
    template: { type: 'string' },
    new_selectors: { type: 'array', items: { type: 'string' }, description: 'selectors/inputs the template introduces that are absent from the design-system stylesheet' },
    unstyled_input_types: { type: 'array', items: { type: 'string' }, description: 'input types rendered with no matching style.css rule' },
    save_button_count: { type: 'integer', description: 'number of Save/submit buttons rendered on the page' },
    pass: { type: 'boolean' },
    violations: { type: 'array', items: { type: 'string' }, description: 'human-readable fidelity violations blocking the commit' },
  },
}

phase('Render')
const results = await pipeline(
  templates,
  template => agent(
    `Config-server UI fidelity gate, RENDER stage for template "${template}". Work from the repo root. ` +
    `The config-server CANNOT be launched from a worktree (routines.SyncAll registers worktree paths with launchd), so DO NOT start the live server. ` +
    `Instead drive a throwaway render harness that renders this single template with representative data` +
    (renderHint ? ` (${renderHint})` : '') +
    `, screenshot the rendered page to a repo-relative path under a scratch dir, and return harness_ok, the image_path, and notes. ` +
    `If you cannot stand up a harness, return harness_ok=false with the reason in notes and an empty image_path.`,
    { label: `render ← ${template}`, phase: 'Render', agentType: 'general-purpose', schema: RENDER_SCHEMA },
  ),
  (rendered, template) => {
    if (!rendered || !rendered.harness_ok || !rendered.image_path) {
      return {
        template,
        new_selectors: [],
        unstyled_input_types: [],
        save_button_count: 0,
        pass: false,
        violations: [`render harness failed for ${template} — cannot verify UI fidelity${rendered && rendered.notes ? `: ${rendered.notes}` : ''}`],
      }
    }
    phase('Gate')
    return agent(
      `Config-server UI fidelity gate, DIFF+GATE stage for template "${template}". Work from the repo root. ` +
      `The rendered screenshot is at "${rendered.image_path}". Read the template and the design-system stylesheet at "${cssPath}". ` +
      `Determine: (1) new_selectors — every selector/input class the template introduces that has no matching rule in ${cssPath}; ` +
      `(2) unstyled_input_types — every input type rendered with no matching style.css rule; ` +
      `(3) save_button_count — the number of Save/submit buttons on the page (more than one is a violation; the account rule is exactly one). ` +
      `Inspect the screenshot for mismatched design system, mis-sized or mis-placed widgets. ` +
      `Set pass=true only when save_button_count === 1, unstyled_input_types is empty, and no widget is visually off-spec; ` +
      `otherwise pass=false and list every problem in violations.`,
      { label: `gate ← ${template}`, phase: 'Gate', agentType: 'general-purpose', schema: GATE_SCHEMA },
    )
  },
)

const gated = results.map((r, i) => r || {
  template: templates[i],
  new_selectors: [],
  unstyled_input_types: [],
  save_button_count: 0,
  pass: false,
  violations: [`gate agent returned nothing for ${templates[i]}`],
})
const violations = gated.filter(r => !r.pass).flatMap(r => r.violations.map(v => `${r.template}: ${v}`))

return {
  gate: { pass: violations.length === 0, violations },
  results: gated,
}
