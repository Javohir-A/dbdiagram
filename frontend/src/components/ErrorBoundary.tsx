import { Component, type ErrorInfo, type ReactNode } from 'react'
import { StatePanel } from './ui/StatePanel'
import { Button } from './ui/Button'

type Props = { children: ReactNode }
type State = { error: Error | null }

/** Last line of defence: a render crash shows a message instead of a white screen. */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('render error', error, info.componentStack)
  }

  render() {
    if (!this.state.error) return this.props.children
    return (
      <StatePanel kind="error" title="Something broke">
        <p>{this.state.error.message}</p>
        <Button onClick={() => this.setState({ error: null })}>Try again</Button>
      </StatePanel>
    )
  }
}
