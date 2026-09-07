// 与后端 internal/server/handlers.go 及 internal/plistinfo 的 JSON 输出对齐。

export type ServiceState = 'running' | 'exited' | 'failed' | 'idle'

export interface Agent {
  path: string
  label: string
  file_name: string
  program?: string
  program_arguments?: string[]
  run_at_load?: boolean
  keep_alive_text?: string
  start_interval?: number
  start_calendar?: string[]
  watch_paths?: string[]
  queue_directories?: string[]
  std_out_path?: string
  std_err_path?: string
  working_dir?: string
  user_name?: string
  group_name?: string
  environment?: [string, string][]
  low_priority_io?: boolean
  process_type?: string
  run_description?: string
  parse_error?: string
}

export interface Service {
  label: string
  file_name?: string
  plist_path?: string
  program?: string
  state: ServiceState
  pid?: string
  exit_code?: string
  enabled: boolean
  loaded: boolean
  parse_error?: string
  runs?: number
  agent?: Agent
}

export interface CronEntry {
  index: number
  raw: string
  schedule: string
  command: string
  importable: boolean
  approximate?: boolean
  reason?: string
  label: string
  imported: boolean
}

export type PlistCreateType = 'runatload' | 'interval' | 'calendar'

export interface CreatePlistRequest {
  label: string
  command: string
  type: PlistCreateType
  interval_seconds?: number
  hour?: number
  minute?: number
  weekdays?: number[]
  keep_alive?: boolean
  working_dir?: string
  std_out_path?: string
  std_err_path?: string
}

export interface PlistSource {
  label: string
  path: string
  content: string
}
