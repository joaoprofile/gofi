export type Frame = string

declare module 'claude-code' {
  interface PluginState {
    'gofi-run': { frame: Shaped<Frame> }
  }
}
