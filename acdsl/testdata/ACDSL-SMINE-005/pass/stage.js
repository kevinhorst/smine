const result = await agent('run the fixture', { label: 'fixture', model: 'opus', effort: 'low' })
const pinned = await agent('run the fixture', { label: 'fixture', model: DEFAULT_SUBAGENT_MODEL })
const asserted = await agent('run the fixture', { label: 'fixture', model: assertModel('laneModel', args.laneModel || DEFAULT_SUBAGENT_MODEL) })
return { result, pinned, asserted }
