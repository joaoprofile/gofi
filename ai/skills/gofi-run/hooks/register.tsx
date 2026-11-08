import { atom, read, update } from 'claude-code'
import type { Register } from 'claude-code'

// The pane only runs `gofi watch --frame`, which reads the disk: no hook here
// calls the model or adds to the conversation, so it spends no token.
const PANE = 'gofi-watch'
const EVERY_MS = 2000
// With nothing changing for this many ticks, look only one tick in IDLE_STEP.
const IDLE_AFTER = 15
const IDLE_STEP = 5

const OUTDATED = 'O gofi instalado é anterior a este painel: atualize o binário do gofi.'

const frame = atom({ plugin: 'gofi-run', key: 'frame' } as const, '', {
  shape: 'frame-1',
})

type Engine = Parameters<typeof read>[0]
type Look = 'shown' | 'outdated' | 'absent'

let columns = 80
let rows = 30
let last = ''
let busy = false
let idle = 0
let tick = 0
let timer: { cancel: () => void } | undefined

async function look($: Engine): Promise<Look> {
  let text: string
  let result: Look = 'shown'
  try {
    const argv = ['gofi', 'watch', '--frame', '--width', String(columns), '--height', String(rows)]
    // The editor's environment may have no Brazilian locale: the pane speaks
    // the language of gofi's skills, whatever it says.
    const { exitCode, stdout, stderr } = await $.process.run(argv, {
      timeoutMs: 5000,
      env: { GOFI_LANG: 'pt' },
    })
    if (exitCode === 0) {
      text = stdout.trimEnd()
    } else if (stderr.includes('unknown flag')) {
      text = OUTDATED
      result = 'outdated'
    } else {
      return 'absent'
    }
  } catch {
    return 'absent'
  }
  if (text === last) {
    idle++
  } else {
    idle = 0
    last = text
    await update($, frame, () => text)
  }
  return result
}

async function poll($: Engine) {
  tick++
  if (busy || (idle >= IDLE_AFTER && tick % IDLE_STEP !== 0)) {
    return
  }
  busy = true
  try {
    await look($)
  } finally {
    busy = false
  }
}

// open shows the pane when this is a gofi project, and keeps it current.
async function open($: Engine): Promise<boolean> {
  if ((await look($)) === 'absent') {
    return false
  }
  timer ??= $.clock.every(EVERY_MS, () => poll($))
  void $.ui.open({ id: PANE, title: 'gofi' })
  return true
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    await $.command.register({
      name: 'gofi-painel',
      description: 'Abre o painel com o andamento do loop do gofi (não gasta tokens)',
    })
    await open($)

    return next(e)
  })

  on('command.run', { command: 'gofi-painel' }, async $ => ({
    text: (await open($)) ? 'Painel do gofi aberto.' : 'Aqui não há um projeto gofi.',
  }))

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const { Code } = $.ui.resolve(e)
    if (e.viewport) {
      columns = Math.max(e.viewport.columns - 4, 40)
      rows = Math.max(e.viewport.rows - 4, 12)
    }
    const text = await read($, frame)

    return <Code source={text || 'Lendo as execuções…'} />
  })
}
