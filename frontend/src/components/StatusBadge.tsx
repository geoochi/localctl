import type { Service } from '../types'

export function StatusBadge({ service }: { service: Service }) {
  if (service.state === 'running') {
    return <span className="badge running">● running{service.pid ? ` (${service.pid})` : ''}</span>
  }
  if (service.state === 'failed') {
    return <span className="badge failed">● exit {service.exit_code ?? '?'}</span>
  }
  if (service.state === 'exited') {
    return <span className="badge exited">○ exited</span>
  }
  return <span className="badge idle">○ not running</span>
}
