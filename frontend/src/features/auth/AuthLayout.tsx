import type { ReactNode } from 'react'
import { Link } from 'react-router'

type Props = { title: string; children: ReactNode; footer: { text: string; to: string; link: string } }

/** The centred card both sign-in pages share. */
export function AuthLayout({ title, children, footer }: Props) {
  return (
    <div className="auth-page">
      <main className="auth-card">
        <span className="brand dark">dbdiagram</span>
        <h1>{title}</h1>
        {children}
        <p className="auth-foot">
          {footer.text} <Link to={footer.to}>{footer.link}</Link>
        </p>
      </main>
    </div>
  )
}
