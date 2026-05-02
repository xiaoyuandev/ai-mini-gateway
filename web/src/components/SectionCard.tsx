import type { ReactNode } from "react"

interface SectionCardProps {
  id?: string
  eyebrow?: string
  title: string
  description?: string
  actions?: ReactNode
  children: ReactNode
  className?: string
}

export function SectionCard(props: SectionCardProps) {
  const cardClassName = props.className ? `panel ${props.className}` : "panel"

  return (
    <section id={props.id} className={cardClassName}>
      <div className="mb-5 flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          {props.eyebrow ? <p className="section-eyebrow">{props.eyebrow}</p> : null}
          <h2 className="section-title">{props.title}</h2>
          {props.description ? <p className="section-copy">{props.description}</p> : null}
        </div>
        {props.actions ? <div className="flex flex-wrap gap-3">{props.actions}</div> : null}
      </div>
      {props.children}
    </section>
  )
}
